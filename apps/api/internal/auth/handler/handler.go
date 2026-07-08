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

// OAuthCallback POST /api/v1/auth/oauth/callback
//
// Exchanges the authorization code for tokens, fetches user identity from
// /oauth/userinfo, ensures a local site row exists with id == userinfo.id,
// stamps last_login_time + ip, and creates the Redis session.
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

	// Stamp last login (best effort).
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

	return response.OK(c, h.composeMe(c, localUser, userInfo.Sub, userInfo.Roles, userInfo.SiteRoles))
}

// Logout POST /api/v1/auth/logout
//
// Best-effort: revoke the OAuth refresh_token on the upstream OAuth server
// (per its implementation, /oauth/revoke looks up the session by refresh
// token — sending the access_token is a no-op), then destroy the local
// Redis session and clear the cookie. Revoke runs in a goroutine because
// network failure to OAuth must not block logout from this site.
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

// Ecosystem GET /api/v1/auth/oauth/ecosystem
//
// Public, unauthenticated. Serves the OAuth app-directory strip ("可以用这个账号
// 登录以下网站") on the login modal. The list is fetched server-to-server from the
// OAuth provider's public GET /oauth/ecosystem and cached in-memory with a TTL
// (see service.ListEcosystem) so the browser reads it same-origin — moyu is on a
// different TLD and is not in the provider's CORS allow-list, so a direct browser
// fetch would be blocked. Always 200 with {apps:[...]} (empty on a cold-start
// upstream failure) — a marketing strip must never break the login page.
func (h *AuthHandler) Ecosystem(c fiber.Ctx) error {
	return response.OK(c, fiber.Map{"apps": h.service.ListEcosystem()})
}

// Me GET /api/v1/auth/me
//
// Composes identity (userID/sub/roles from session+JWT), display fields
// (name/avatar/bio from OAuth /users/batch), and site-local state
// (moemoepoint, daily counters, follow counts) into a single response so
// the frontend can render the whole profile without extra round-trips.
func (h *AuthHandler) Me(c fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	roles := middleware.GetRoles(c)

	var local authModel.User
	if err := h.db.First(&local, user.ID).Error; err != nil {
		return response.Error(c, errors.ErrNotFound("user not found"))
	}

	return response.OK(c, h.composeMe(c, &local, user.Sub, roles, middleware.GetSiteRoles(c)))
}

// ─── OAuth display-layer proxy ────────────────────────────────────────
// Only name / bio / avatar are proxied — these are 展示层 per the
// 2026-05-23 OAuth policy (docs/oauth/02-user-profile.md §身份操作 vs
// 展示操作). Identity-layer ops (改密码 / 改邮箱 / 注销账号 / 2FA) are
// intentionally NOT exposed here; the moyu frontend sends users to OAuth's
// own /profile via an external link instead. Reason per upstream policy:
// security audit at a single point, future 2FA / anomaly-notify rollouts
// only need to be implemented once, and email-hijack attack surface stays
// out of every downstream UI.

// UpdateMe PATCH /api/v1/auth/me
// Proxies → OAuth PATCH /auth/me. Accepts {name, avatar, avatar_image_hash, bio}.
func (h *AuthHandler) UpdateMe(c fiber.Ctx) error {
	err := h.proxyUserOAuth(c, fiber.MethodPatch, "/auth/me")
	// The userclient caches OAuth briefs ~10min. After a self profile edit, evict
	// this user's entry so the next /auth/me (the frontend's refreshMe) reflects
	// the new name/bio immediately instead of the stale cached copy — otherwise
	// the change "doesn't stick" until the cache expires ("个人签名无法更改").
	if uid := middleware.GetUserID(c); uid > 0 {
		h.users.Invalidate(uint(uid))
	}
	return err
}

// UploadAvatar POST /api/v1/auth/me/avatar
// Proxies multipart → OAuth POST /auth/me/avatar (OAuth handles
// image_service upload internally and writes avatar_image_hash). Body is
// forwarded as-is so the multipart boundary survives.
func (h *AuthHandler) UploadAvatar(c fiber.Ctx) error {
	err := h.proxyUserOAuth(c, fiber.MethodPost, "/auth/me/avatar")
	// Same cache-staleness fix as UpdateMe: evict so the new avatar_image_hash
	// shows on the next /auth/me instead of the ~10min-cached old avatar.
	if uid := middleware.GetUserID(c); uid > 0 {
		h.users.Invalidate(uint(uid))
	}
	return err
}

// proxyUserOAuth is the shared helper for the display-layer proxies.
// Pulls access_token from session, forwards body + content-type to the
// OAuth endpoint, writes OAuth's response back to the client unchanged.
func (h *AuthHandler) proxyUserOAuth(c fiber.Ctx, method, path string) error {
	accessToken := middleware.GetAccessToken(c)
	if accessToken == "" {
		return response.Error(c, errors.ErrUnauthorized())
	}
	body := c.Body() // multipart parts ride along untouched
	ct := string(c.Request().Header.ContentType())
	status, raw, err := h.service.ProxyUserToOAuth(method, path, accessToken, body, ct)
	if err != nil {
		slog.Error("OAuth profile proxy failed", "method", method, "path", path, "error", err)
		return response.Error(c, errors.ErrInternal("OAuth 服务不可达"))
	}
	// Re-emit OAuth's exact response — preserve status code (4xx errors from
	// OAuth like "name already taken" need to round-trip with their
	// original code so the FE shows the right message).
	c.Set("Content-Type", "application/json")
	return c.Status(status).Send(raw)
}

// composeMe merges the local user row with the OAuth brief (name/avatar/bio)
// into a MeResponse. If the OAuth /users/batch call fails we still return
// the local fields -- name/avatar will simply be empty rather than crashing
// the page.
func (h *AuthHandler) composeMe(c fiber.Ctx, local *authModel.User, sub string, roles, siteRoles []string) dto.MeResponse {
	// Never marshal roles as JSON null. A nil []string serializes to `null`,
	// which the frontend persists into its cookie-backed user store; then
	// isAdmin/isModerator run roles.includes(...) during SSR and throw
	// "Cannot read properties of null (reading 'includes')" → 500s on the
	// /patch/* pages. An empty slice serializes to [] and is safe.
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
	}

	// composeMe is ONLY ever called for the current user (Me + OAuthCallback).
	// The /users/batch brief is cached ~10min (userclient). For OTHER users that
	// staleness is fine (contract C6), but this is the user's OWN /auth/me: if they
	// changed their avatar / 用户名 / 签名 on another ecosystem site (the OAuth
	// profile page, the forum), the cached brief would keep showing the old value
	// for up to 10min — even though the frontend correctly re-pulls /auth/me on tab
	// focus (plugins/revalidate-me.client.ts). moyu only auto-evicts on a SELF edit
	// here (UpdateMe / UploadAvatar Invalidate); a cross-app change leaves no signal.
	// Evict first so the current user's own profile is always live. Costs one
	// single-id batch fetch per /auth/me (frontend-deduped to ~1/min/active user);
	// the refetch repopulates the cache for any other context that needs this id.
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
		resp.Bio = brief.Bio
		// Prefer the live OAuth brief's roles over the JWT-derived `roles`. A role
		// granted in OAuth (e.g. `creator`) only enters the access-token JWT after
		// a token refresh (docs/oauth/08-creator-applications.md §下游耦合点), so
		// decodeJWTRoles keeps showing the stale set and the user still renders as
		// "用户". The /users/batch brief reflects the grant within the userclient
		// TTL (~10min) — this mirrors kungal's /auth/me (RoleFromOAuthRoles(u.Roles)).
		// Real authorization is still enforced server-side from the JWT
		// (middleware.HasRole), so this only refreshes the display badge / FE gate.
		if len(brief.Roles) > 0 {
			resp.Roles = brief.Roles
		}
		// Same freshness tradeoff as roles above: prefer the live brief's
		// site_roles, but only when it carried some so an empty brief slice
		// doesn't clobber the JWT/userinfo-derived set.
		if len(brief.SiteRoles) > 0 {
			resp.SiteRoles = brief.SiteRoles
		}
	}
	return resp
}
