// Package handler implements the auth HTTP handlers.
//
// After the OAuth migration the surface is small: callback, logout, me.
// Password / forgot / email-verify routes have been removed -- the OAuth
// server owns all of that.
package handler

import (
	"encoding/json"
	stderrors "errors"
	"log/slog"
	"time"

	authModel "kun-galgame-patch-api/internal/auth/model"
	"kun-galgame-patch-api/internal/auth/dto"
	"kun-galgame-patch-api/internal/auth/service"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/userclient"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type AuthHandler struct {
	service *service.AuthService
	rdb     *redis.Client
	db      *gorm.DB
	users   *userclient.Client
}

func New(svc *service.AuthService, rdb *redis.Client, db *gorm.DB, users *userclient.Client) *AuthHandler {
	return &AuthHandler{service: svc, rdb: rdb, db: db, users: users}
}

// OAuthCallback POST /api/v1/auth/oauth/callback
//
// Exchanges the authorization code for tokens, fetches user identity from
// /oauth/userinfo, ensures a local site row exists with id == userinfo.id,
// stamps last_login_time + ip, and creates the Redis session.
func (h *AuthHandler) OAuthCallback(c *fiber.Ctx) error {
	var req dto.OAuthCallbackRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	tokenResp, err := h.service.ExchangeCode(req.Code, req.CodeVerifier)
	if err != nil {
		slog.Error("OAuth code exchange failed", "error", err)
		return response.Error(c, errors.ErrBadRequest("OAuth authentication failed"))
	}

	userInfo, err := h.service.GetUserInfo(tokenResp.AccessToken)
	if err != nil {
		if stderrors.Is(err, service.ErrUserBanned) {
			slog.Warn("OAuth login blocked: account banned (10014)")
			return response.Error(c, errors.ErrAccountBanned(""))
		}
		slog.Error("OAuth get userinfo failed", "error", err)
		return response.Error(c, errors.ErrBadRequest("failed to get user info"))
	}
	if userInfo.ID == 0 {
		slog.Error("OAuth userinfo missing id field", "sub", userInfo.Sub)
		return response.Error(c, errors.ErrBadRequest("invalid user info"))
	}

	localUser, err := h.service.FindOrCreateUserByID(userInfo.ID)
	if err != nil {
		slog.Error("Failed to provision local user row", "uid", userInfo.ID, "error", err)
		return response.Error(c, errors.ErrInternal(""))
	}

	// Stamp last login (best effort).
	go func(uid int, ip string) {
		h.db.Table("user").Where("id = ?", uid).Updates(map[string]any{
			"last_login_time": time.Now().Format(time.RFC3339),
			"ip":              ip,
		})
	}(userInfo.ID, c.IP())

	session := &middleware.SessionData{
		UserInfo: middleware.UserInfo{
			UID: userInfo.ID,
			Sub: userInfo.Sub,
		},
		OAuthAccessToken:  tokenResp.AccessToken,
		OAuthRefreshToken: tokenResp.RefreshToken,
		OAuthExpiresAt:    time.Now().Unix() + tokenResp.ExpiresIn,
	}

	if err := middleware.CreateSession(c, h.rdb, session); err != nil {
		slog.Error("Create session failed", "error", err)
		return response.Error(c, errors.ErrInternal(""))
	}

	return response.OK(c, h.composeMe(c, localUser, userInfo.Sub, userInfo.Roles))
}

// Logout POST /api/v1/auth/logout
//
// Best-effort: revoke the OAuth refresh_token on the upstream OAuth server
// (per its implementation, /oauth/revoke looks up the session by refresh
// token — sending the access_token is a no-op), then destroy the local
// Redis session and clear the cookie. Revoke runs in a goroutine because
// network failure to OAuth must not block logout from this site.
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	sessionID := c.Cookies(middleware.SessionCookieName)
	if sessionID != "" {
		if data, err := h.rdb.Get(c.Context(), middleware.SessionPrefix+sessionID).Result(); err == nil {
			var session middleware.SessionData
			if err := json.Unmarshal([]byte(data), &session); err == nil && session.OAuthRefreshToken != "" {
				go h.service.RevokeOAuthToken(session.OAuthRefreshToken)
			}
		}
	}

	middleware.DestroySession(c, h.rdb)
	return response.OKMessage(c, "Logged out")
}

// Me GET /api/v1/auth/me
//
// Composes identity (uid/sub/roles from session+JWT), display fields
// (name/avatar/bio from OAuth /users/batch), and site-local state
// (moemoepoint, daily counters, follow counts) into a single response so
// the frontend can render the whole profile without extra round-trips.
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	roles := middleware.GetRoles(c)

	var local authModel.User
	if err := h.db.First(&local, user.UID).Error; err != nil {
		return response.Error(c, errors.ErrNotFound("user not found"))
	}

	return response.OK(c, h.composeMe(c, &local, user.Sub, roles))
}

// composeMe merges the local user row with the OAuth brief (name/avatar/bio)
// into a MeResponse. If the OAuth /users/batch call fails we still return
// the local fields -- name/avatar will simply be empty rather than crashing
// the page.
func (h *AuthHandler) composeMe(c *fiber.Ctx, local *authModel.User, sub string, roles []string) dto.MeResponse {
	resp := dto.MeResponse{
		ID:              local.ID,
		Sub:             sub,
		Roles:           roles,
		Moemoepoint:     local.Moemoepoint,
		DailyCheckIn:    local.DailyCheckIn,
		DailyImageCount: local.DailyImageCount,
		DailyUploadSize: local.DailyUploadSize,
		FollowerCount:   local.FollowerCount,
		FollowingCount:  local.FollowingCount,
	}

	brief, err := h.users.User(c.Context(), uint(local.ID))
	if err != nil {
		slog.Warn("OAuth /users/batch lookup failed in composeMe; returning empty display fields",
			"uid", local.ID, "error", err)
		return resp
	}
	if brief != nil {
		resp.Name = brief.Name
		resp.Avatar = brief.Avatar
		resp.AvatarImageHash = brief.AvatarImageHash
		resp.Bio = brief.Bio
	}
	return resp
}
