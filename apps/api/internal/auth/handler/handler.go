package handler

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"log/slog"
	"time"

	"kun-galgame-patch-api/internal/auth/dto"
	authModel "kun-galgame-patch-api/internal/auth/model"
	"kun-galgame-patch-api/internal/auth/service"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/moemoepoint"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/upstream"
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
	points  *moemoepoint.Awarder
}

func New(svc *service.AuthService, rdb *redis.Client, db *gorm.DB, users *userclient.Client, points *moemoepoint.Awarder) *AuthHandler {
	return &AuthHandler{service: svc, rdb: rdb, db: db, users: users, points: points}
}

func (h *AuthHandler) OAuthCallback(c fiber.Ctx) error {
	var req dto.OAuthCallbackRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	tokenResp, err := h.service.ExchangeCode(c.Context(), req.Code, req.CodeVerifier)
	if err != nil {
		if upstream.KindOf(err) == upstream.Rejected {
			slog.Warn("OAuth refused the authorization code", "error", err)
		}
		return response.Upstream(c, err, "登录凭据已失效，请重新登录")
	}

	// The code is spent from here on, so nothing below can be retried with it:
	// the reader has to start the sign-in again, and the token it bought is
	// revoked rather than left live upstream.
	abandon := func(step string, err error) {
		slog.Error("sign-in abandoned after the code exchange; its token is revoked",
			"step", step, "error", err)
		go h.service.RevokeOAuthToken(tokenResp.RefreshToken)
	}

	userInfo, err := h.service.GetUserInfo(c.Context(), tokenResp.AccessToken)
	if stderrors.Is(err, service.ErrUserBanned) {
		slog.Warn("OAuth login blocked: account banned (10014)")
		go h.service.RevokeOAuthToken(tokenResp.RefreshToken)
		return response.Error(c, errors.ErrAccountBanned(""))
	}
	if err != nil {
		abandon("userinfo", err)
		return response.Upstream(c, err, "")
	}
	if userInfo.ID == 0 {
		abandon("userinfo", stderrors.New("userinfo carried no id"))
		return response.Error(c, errors.ErrInternal("登录失败，请重新登录"))
	}

	localUser, err := h.service.FindOrCreateUserByID(userInfo.ID)
	if err != nil {
		abandon("local user row", err)
		return response.Error(c, errors.ErrInternal("登录失败，请重新登录"))
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
		abandon("session", err)
		return response.Error(c, errors.ErrInternal("登录失败，请重新登录"))
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
	return response.OK(c, fiber.Map{"apps": h.service.ListEcosystem(c.Context())})
}

func (h *AuthHandler) Me(c fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	roles := middleware.GetRoles(c)

	var local authModel.User
	if err := h.db.First(&local, user.ID).Error; err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return response.Error(c, errors.ErrNotFound("user not found"))
		}
		slog.Error("read local user row", "userID", user.ID, "error", err)
		return response.Error(c, errors.ErrInternal(""))
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
	info, err := h.service.GetUserInfo(c.Context(), token)
	if err != nil {
		logDegraded(c.Context(), "OAuth userinfo lookup failed; leaving the content stance unreported", err,
			"userID", middleware.GetUserID(c))
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
	status, raw, err := h.service.ProxyUserToOAuth(c.Context(), method, path, accessToken, body, ct)
	if err != nil {
		return response.Upstream(c, err, "")
	}
	return relayOAuth(c, method+" "+path, status, raw)
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

	// The local column only moves on moyu's own awards, so the forum's and
	// the shop's never reached it.
	if balance, err := h.points.Balance(c.Context(), local.ID); err == nil {
		resp.Moemoepoint = balance
	} else {
		logDegraded(c.Context(), "moemoepoint balance unavailable; /auth/me shows the cached one", err,
			"userID", local.ID)
	}

	h.users.Invalidate(uint(local.ID))

	brief, err := h.users.User(c.Context(), uint(local.ID))
	if err != nil {
		logDegraded(c.Context(), "OAuth /users/batch lookup failed in composeMe; returning empty display fields",
			err, "userID", local.ID)
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

func logDegraded(ctx context.Context, msg string, err error, attrs ...any) {
	level := slog.LevelError
	if k := upstream.KindOf(err); k == upstream.Unavailable || k == upstream.RateLimited {
		level = slog.LevelWarn
	}
	slog.Log(ctx, level, msg, append(attrs, "error", err)...)
}
