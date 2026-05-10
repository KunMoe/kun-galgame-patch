// Package handler implements the auth HTTP handlers.
//
// After the OAuth migration the surface is small: callback, logout, me.
// Password / forgot / email-verify routes have been removed -- the OAuth
// server owns all of that.
package handler

import (
	"encoding/json"
	"log/slog"
	"time"

	"kun-galgame-patch-api/internal/auth/dto"
	"kun-galgame-patch-api/internal/auth/service"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type AuthHandler struct {
	service *service.AuthService
	rdb     *redis.Client
	db      *gorm.DB
}

func New(svc *service.AuthService, rdb *redis.Client, db *gorm.DB) *AuthHandler {
	return &AuthHandler{service: svc, rdb: rdb, db: db}
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
		slog.Error("OAuth get userinfo failed", "error", err)
		return response.Error(c, errors.ErrBadRequest("failed to get user info"))
	}
	if userInfo.ID == 0 {
		slog.Error("OAuth userinfo missing id field", "sub", userInfo.Sub)
		return response.Error(c, errors.ErrBadRequest("invalid user info"))
	}

	if _, err := h.service.FindOrCreateUserByID(userInfo.ID); err != nil {
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

	return response.OK(c, dto.MeResponse{
		UID:   userInfo.ID,
		Sub:   userInfo.Sub,
		Roles: userInfo.Roles,
	})
}

// Logout POST /api/v1/auth/logout
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	sessionID := c.Cookies(middleware.SessionCookieName)
	if sessionID != "" {
		if data, err := h.rdb.Get(c.Context(), middleware.SessionPrefix+sessionID).Result(); err == nil {
			var session middleware.SessionData
			if err := json.Unmarshal([]byte(data), &session); err == nil && session.OAuthAccessToken != "" {
				go h.service.RevokeOAuthToken(session.OAuthAccessToken)
			}
		}
	}

	middleware.DestroySession(c, h.rdb)
	return response.OKMessage(c, "Logged out")
}

// Me GET /api/v1/auth/me
//
// Returns the bare-minimum identity for the current session: uid + sub +
// roles (decoded from the OAuth access_token JWT, no signature verify needed
// since the token was placed in the session by us). Front-end should call
// /oauth/userinfo or /users/batch?ids=<uid> for display fields (name, avatar,
// bio); Phase 5-6 will fold local-only fields (moemoepoint, daily counters,
// follow counts) into a single composed payload returned by this endpoint.
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	roles := middleware.GetRoles(c)
	return response.OK(c, dto.MeResponse{
		UID:   user.UID,
		Sub:   user.Sub,
		Roles: roles,
	})
}
