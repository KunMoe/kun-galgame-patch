// Package middleware: session-cookie auth backed by Redis, plus role-gated
// helpers that read OAuth roles from the access_token JWT.
//
// The session is intentionally minimal -- only uid, sub and the OAuth tokens.
// Display fields (name, avatar, bio) are fetched from OAuth on demand by
// downstream handlers via pkg/userclient. Roles are read from the JWT roles
// claim in the OAuth access_token (no signature verify needed: the token was
// stored in this Redis session by us, so it's not user-controlled at request
// time).
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"

	"bytes"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

// UserInfo is the slim per-request identity stamped onto fiber locals by
// the Auth / OptionalAuth middleware.
type UserInfo struct {
	UID int    `json:"uid"`
	Sub string `json:"sub"`
}

// SessionData is the JSON value stored in Redis under "session:<id>".
// OAuth tokens live here so middleware can refresh them in the background
// and HasRole can read the roles claim.
type SessionData struct {
	UserInfo
	OAuthAccessToken  string `json:"oauth_access_token"`
	OAuthRefreshToken string `json:"oauth_refresh_token"`
	OAuthExpiresAt    int64  `json:"oauth_expires_at"`
}

const (
	SessionCookieName = "kun_session"
	SessionTTL        = 7 * 24 * time.Hour
	SessionPrefix     = "session:"
	userContextKey    = "user"
	rolesContextKey   = "oauth_roles"
)

func Auth(rdb *redis.Client, oauthCfg config.OAuthConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionID := c.Cookies(SessionCookieName)
		if sessionID == "" {
			return response.Error(c, errors.ErrUnauthorized())
		}

		data, err := rdb.Get(context.Background(), SessionPrefix+sessionID).Result()
		if err == redis.Nil {
			return response.Error(c, errors.ErrAuthExpired())
		}
		if err != nil {
			slog.Error("Redis get session failed", "error", err)
			return response.Error(c, errors.ErrInternal(""))
		}

		var session SessionData
		if err := json.Unmarshal([]byte(data), &session); err != nil {
			return response.Error(c, errors.ErrInternal(""))
		}

		if session.OAuthExpiresAt > 0 && time.Now().Unix() > session.OAuthExpiresAt-300 {
			go refreshOAuthToken(rdb, oauthCfg, sessionID, &session)
		}

		c.Locals(userContextKey, &session.UserInfo)
		c.Locals(rolesContextKey, decodeJWTRoles(session.OAuthAccessToken))
		return c.Next()
	}
}

func OptionalAuth(rdb *redis.Client, oauthCfg config.OAuthConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionID := c.Cookies(SessionCookieName)
		if sessionID == "" {
			return c.Next()
		}

		data, err := rdb.Get(context.Background(), SessionPrefix+sessionID).Result()
		if err != nil {
			return c.Next()
		}

		var session SessionData
		if err := json.Unmarshal([]byte(data), &session); err != nil {
			return c.Next()
		}

		c.Locals(userContextKey, &session.UserInfo)
		c.Locals(rolesContextKey, decodeJWTRoles(session.OAuthAccessToken))
		return c.Next()
	}
}

func GetUser(c *fiber.Ctx) *UserInfo {
	user, ok := c.Locals(userContextKey).(*UserInfo)
	if !ok {
		return nil
	}
	return user
}

func MustGetUser(c *fiber.Ctx) *UserInfo {
	return c.Locals(userContextKey).(*UserInfo)
}

func GetUID(c *fiber.Ctx) int {
	user := GetUser(c)
	if user == nil {
		return 0
	}
	return user.UID
}

// GetRoles returns the OAuth roles for the current request, or an empty slice
// if no session is attached. Roles come from the access_token JWT roles claim.
func GetRoles(c *fiber.Ctx) []string {
	v, ok := c.Locals(rolesContextKey).([]string)
	if !ok {
		return nil
	}
	return v
}

// HasRole reports whether the current request's roles set contains role.
func HasRole(c *fiber.Ctx, role string) bool {
	return slices.Contains(GetRoles(c), role)
}

// HasAnyRole reports whether the current request's roles set contains any of
// the listed roles. Pass nothing to require "at least logged in".
func HasAnyRole(c *fiber.Ctx, roles ...string) bool {
	if len(roles) == 0 {
		return GetUser(c) != nil
	}
	have := GetRoles(c)
	for _, want := range roles {
		if slices.Contains(have, want) {
			return true
		}
	}
	return false
}

// SecureCookies controls whether the session cookie is HTTPS-only. Set by
// the app at startup based on KUN_SERVER_MODE; in dev over HTTP this must
// be false or the browser refuses to store the cookie.
var SecureCookies = true

func CreateSession(c *fiber.Ctx, rdb *redis.Client, session *SessionData) error {
	sessionID, err := generateSessionID()
	if err != nil {
		return err
	}

	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	if err := rdb.Set(context.Background(), SessionPrefix+sessionID, data, SessionTTL).Err(); err != nil {
		return err
	}

	c.Cookie(&fiber.Cookie{
		Name:     SessionCookieName,
		Value:    sessionID,
		MaxAge:   int(SessionTTL.Seconds()),
		HTTPOnly: true,
		Secure:   SecureCookies,
		SameSite: "Lax",
		Path:     "/",
	})

	return nil
}

func DestroySession(c *fiber.Ctx, rdb *redis.Client) error {
	sessionID := c.Cookies(SessionCookieName)
	if sessionID != "" {
		rdb.Del(context.Background(), SessionPrefix+sessionID)
	}

	c.Cookie(&fiber.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   SecureCookies,
		SameSite: "Lax",
		Path:     "/",
	})

	return nil
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// decodeJWTRoles extracts the `roles` claim from a JWT without verifying the
// signature. Safe here because the token came out of our Redis session
// (placed by OAuthCallback after a verified /oauth/token exchange) -- it is
// never user-controlled at request time. Returns nil on any decode error.
func decodeJWTRoles(token string) []string {
	if token == "" {
		return nil
	}
	parts := strings.SplitN(token, ".", 3)
	if len(parts) < 2 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Some encoders pad; try the std variant.
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil
		}
	}
	var claims struct {
		Roles []string `json:"roles"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil
	}
	return claims.Roles
}

func refreshOAuthToken(rdb *redis.Client, oauthCfg config.OAuthConfig, sessionID string, session *SessionData) {
	ctx := context.Background()
	lockKey := "lock:refresh:" + sessionID

	ok, err := rdb.SetArgs(ctx, lockKey, 1, redis.SetArgs{
		TTL:  30 * time.Second,
		Mode: "NX",
	}).Result()
	if err != nil || ok != "OK" {
		return
	}
	defer rdb.Del(ctx, lockKey)

	// KUN OAuth Server takes JSON, not form-urlencoded -- see
	// docs/oauth/oauth-integration-guide.md.
	payload, _ := json.Marshal(map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": session.OAuthRefreshToken,
		"client_id":     oauthCfg.ClientID,
		"client_secret": oauthCfg.ClientSecret,
	})
	resp, err := http.Post(
		oauthCfg.ServerURL+"/oauth/token",
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		slog.Error("OAuth token refresh failed", "error", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		slog.Error("OAuth token refresh failed", "status", resp.StatusCode, "body", string(respBody))
		return
	}

	var env struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int64  `json:"expires_in"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &env); err != nil {
		slog.Error("OAuth token refresh decode failed", "error", err, "body", string(respBody))
		return
	}
	if env.Code != 0 {
		slog.Error("OAuth token refresh business error", "code", env.Code, "message", env.Message)
		return
	}

	session.OAuthAccessToken = env.Data.AccessToken
	session.OAuthRefreshToken = env.Data.RefreshToken
	session.OAuthExpiresAt = time.Now().Unix() + env.Data.ExpiresIn

	data, _ := json.Marshal(session)
	rdb.Set(ctx, SessionPrefix+sessionID, data, SessionTTL)
}
