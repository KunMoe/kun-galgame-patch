package middleware

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

type UserInfo struct {
	UID   int    `json:"uid"`
	Sub   string `json:"sub"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  int    `json:"role"`
}

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

		// Refresh OAuth token if expiring soon (< 5 minutes)
		if session.OAuthExpiresAt > 0 && time.Now().Unix() > session.OAuthExpiresAt-300 {
			go refreshOAuthToken(rdb, oauthCfg, sessionID, &session)
		}

		c.Locals(userContextKey, &session.UserInfo)
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

func GetRole(c *fiber.Ctx) int {
	user := GetUser(c)
	if user == nil {
		return 0
	}
	return user.Role
}

// SecureCookies controls whether HTTPS-only cookies are enabled. Set by the app at startup based on environment.
// In dev over HTTP this must be off, otherwise the browser refuses to store the cookie.
var SecureCookies = true

// CreateSession creates a new session in Redis and sets the cookie.
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

// DestroySession removes the session from Redis and clears the cookie.
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

func refreshOAuthToken(rdb *redis.Client, oauthCfg config.OAuthConfig, sessionID string, session *SessionData) {
	ctx := context.Background()
	lockKey := "lock:refresh:" + sessionID

	ok, err := rdb.SetArgs(ctx, lockKey, 1, redis.SetArgs{
		TTL: 30 * time.Second,
		Mode: "NX",
	}).Result()
	if err != nil || ok != "OK" {
		return
	}
	defer rdb.Del(ctx, lockKey)

	// KUN OAuth Server takes JSON, not form-urlencoded — see
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
