package handler

import (
	"encoding/json"
	stderrors "errors"
	"log/slog"
	"time"

	"kun-galgame-patch-api/internal/auth/dto"
	authModel "kun-galgame-patch-api/internal/auth/model"
	"kun-galgame-patch-api/internal/auth/service"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/userclient"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
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

func (h *AuthHandler) OAuthCallback(c fiber.Ctx) error {
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
		slog.Error("Failed to provision local user row", "userID", userInfo.ID, "error", err)
		return response.Error(c, errors.ErrInternal(""))
	}

	go func(userID int, ip string) {
		h.db.Table("user").Where("id = ?", userID).Updates(map[string]any{
			"last_login_time": time.Now().Format(time.RFC3339),
			"ip":              ip,
		})
	}(userInfo.ID, c.IP())

	session := &middleware.SessionData{
		UserInfo: middleware.UserInfo{
			ID:  userInfo.ID,
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

	return response.OK(c, h.composeMe(c, localUser, userInfo.Sub, userInfo.Roles, userInfo.SiteRoles,
		contentStance{adultConfirmed: userInfo.AdultConfirmed, nsfwDisplay: userInfo.NsfwDisplay}))
}

func (h *AuthHandler) Logout(c fiber.Ctx) error {
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

func (h *AuthHandler) Ecosystem(c fiber.Ctx) error {
	return response.OK(c, fiber.Map{"apps": h.service.ListEcosystem()})
}

func (h *AuthHandler) Me(c fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	roles := middleware.GetRoles(c)

	var local authModel.User
	if err := h.db.First(&local, user.ID).Error; err != nil {
		return response.Error(c, errors.ErrNotFound("user not found"))
	}

	return response.OK(c, h.composeMe(c, &local, user.Sub, roles, middleware.GetSiteRoles(c),
		h.readContentStance(c)))
}

type contentStance struct {
	adultConfirmed bool
	nsfwDisplay    string
}

// The stance is re-read from OAuth on every /auth/me rather than cached in the
// session, because the only place a reader can attest their age is the account
// centre — a session-cached stance would make them log out and back in to see
// the setting they just changed there.
func (h *AuthHandler) readContentStance(c fiber.Ctx) contentStance {
	token := middleware.GetAccessToken(c)
	if token == "" {
		return contentStance{}
	}
	info, err := h.service.GetUserInfo(token)
	if err != nil {
		slog.Warn("OAuth userinfo lookup failed; leaving the content stance unreported",
			"userID", middleware.GetUserID(c), "error", err)
		return contentStance{}
	}
	return contentStance{adultConfirmed: info.AdultConfirmed, nsfwDisplay: info.NsfwDisplay}
}

func (h *AuthHandler) UpdateMe(c fiber.Ctx) error {
	err := h.proxyUserOAuth(c, fiber.MethodPatch, "/auth/me")
	if uid := middleware.GetUserID(c); uid > 0 {
		h.users.Invalidate(uint(uid))
	}
	return err
}

func (h *AuthHandler) UploadAvatar(c fiber.Ctx) error {
	err := h.proxyUserOAuth(c, fiber.MethodPost, "/auth/me/avatar")
	if uid := middleware.GetUserID(c); uid > 0 {
		h.users.Invalidate(uint(uid))
	}
	return err
}

func (h *AuthHandler) proxyUserOAuth(c fiber.Ctx, method, path string) error {
	accessToken := middleware.GetAccessToken(c)
	if accessToken == "" {
		return response.Error(c, errors.ErrUnauthorized())
	}
	body := c.Body()
	ct := string(c.Request().Header.ContentType())
	status, raw, err := h.service.ProxyUserToOAuth(method, path, accessToken, body, ct)
	if err != nil {
		slog.Error("OAuth profile proxy failed", "method", method, "path", path, "error", err)
		return response.Error(c, errors.ErrInternal("OAuth 服务不可达"))
	}
	c.Set("Content-Type", "application/json")
	return c.Status(status).Send(raw)
}

func (h *AuthHandler) composeMe(c fiber.Ctx, local *authModel.User, sub string, roles, siteRoles []string, stance contentStance) dto.MeResponse {
	if roles == nil {
		roles = []string{}
	}
	if siteRoles == nil {
		siteRoles = []string{}
	}
	resp := dto.MeResponse{
		ID:              local.ID,
		Sub:             sub,
		Roles:           roles,
		SiteRoles:       siteRoles,
		Moemoepoint:     local.Moemoepoint,
		DailyCheckIn:    local.DailyCheckIn,
		DailyImageCount: local.DailyImageCount,
		DailyUploadSize: local.DailyUploadSize,
		FollowerCount:   local.FollowerCount,
		FollowingCount:  local.FollowingCount,
		AdultConfirmed:  stance.adultConfirmed,
		NsfwDisplay:     stance.nsfwDisplay,
	}

	h.users.Invalidate(uint(local.ID))

	brief, err := h.users.User(c.Context(), uint(local.ID))
	if err != nil {
		slog.Warn("OAuth /users/batch lookup failed in composeMe; returning empty display fields",
			"userID", local.ID, "error", err)
		return resp
	}
	if brief != nil {
		resp.Name = brief.Name
		resp.Avatar = brief.Avatar
		resp.AvatarImageHash = brief.AvatarImageHash
		resp.Cosmetics = brief.Cosmetics
		resp.Bio = brief.Bio
		if len(brief.Roles) > 0 {
			resp.Roles = brief.Roles
		}
		if len(brief.SiteRoles) > 0 {
			resp.SiteRoles = brief.SiteRoles
		}
	}
	return resp
}
