// Package middleware: session-cookie auth backed by Redis, plus role-gated
// helpers that read OAuth roles from the access_token JWT.
//
// The session is intentionally minimal -- only userID, sub and the OAuth tokens.
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
	stderrors "errors"
	"fmt"
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
//
// `id` matches the DB-truth chain (Prisma user.id → Go MeResponse.id →
// /user/:id → KunUser). The JWT/URL label `userID` was a transport-layer
// alias for the same integer; it lives at the OAuth layer only and does
// not propagate into local types.
type UserInfo struct {
	ID  int    `json:"id"`
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
	// SessionCookieName / SessionPrefix MUST be distinct from kungal's
	// (kun-galgame-nuxt4) values. In local dev both sites run on
	// 127.0.0.1 — cookies are domain-scoped, NOT port-scoped — and share
	// one Redis. A shared cookie name + key prefix made kungal and moyu
	// read/refresh/delete each other's sessions, producing cross-site
	// logout (client_id_mismatch on the OAuth server). Keep site-unique.
	SessionCookieName     = "moyu_session"
	SessionTTL            = 7 * 24 * time.Hour
	SessionPrefix         = "moyu:session:"
	userContextKey        = "user"
	rolesContextKey       = "oauth_roles"
	accessTokenContextKey = "oauth_access_token"
)

// RevokeUserSessions best-effort deletes every Redis session belonging to a
// user, matched by the id embedded in SessionData. Used by the admin user
// purge: the request path reads identity from the session blob (not the DB
// row), so without this a purged spammer's active scripted session would keep
// authenticating for the rest of its 7-day TTL. Returns the count deleted.
//
// SCANs moyu:session:* with a cursor (non-blocking). A GET/parse miss on one
// key is skipped, not fatal — this is cleanup, not an auth gate. NOTE: this
// does not revoke the upstream OAuth grant; a truly persistent spammer must
// also be banned on the OAuth console (out of moyu's scope).
func RevokeUserSessions(ctx context.Context, rdb *redis.Client, userID int) (int, error) {
	var (
		cursor  uint64
		deleted int
	)
	for {
		keys, next, err := rdb.Scan(ctx, cursor, SessionPrefix+"*", 200).Result()
		if err != nil {
			return deleted, err
		}
		for _, key := range keys {
			val, gerr := rdb.Get(ctx, key).Result()
			if gerr != nil {
				continue
			}
			var s SessionData
			if json.Unmarshal([]byte(val), &s) != nil {
				continue
			}
			if s.ID == userID {
				if rdb.Del(ctx, key).Err() == nil {
					deleted++
				}
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return deleted, nil
}

func Auth(rdb *redis.Client, oauthCfg config.OAuthConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionID := c.Cookies(SessionCookieName)
		if sessionID == "" {
			return response.Error(c, errors.ErrUnauthorized())
		}

		ctx := c.Context()
		data, err := rdb.Get(ctx, SessionPrefix+sessionID).Result()
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

		// Two-tier refresh:
		//   - HARD expired (now >= ExpiresAt): synchronous refresh. If OAuth
		//     permanently rejects (invalid_grant / revoked / 401), the
		//     refresher itself deletes the Redis session; we then clear the
		//     cookie and reject the request so the user re-logs in.
		//   - SOFT window (T-5min .. T): the token is still valid, refresh in
		//     the background and let the request through.
		now := time.Now().Unix()
		if session.OAuthExpiresAt > 0 && now >= session.OAuthExpiresAt {
			if err := refreshOAuthToken(ctx, rdb, oauthCfg, sessionID, &session); err != nil {
				// Lock contention: another concurrent request is refreshing
				// this very session right now (the SSR fan-out norm). Wait
				// for it to publish the fresh tokens instead of clearing the
				// cookie and kicking the user.
				if stderrors.Is(err, errRefreshLockContended) &&
					waitForRefreshedSession(ctx, rdb, sessionID, &session) {
					// fresh session loaded into `session`; fall through.
				} else {
					slog.Warn("OAuth access token expired and refresh failed; rejecting request",
						"sessionPrefix", sessionID[:min(8, len(sessionID))], "error", err)
					// Only destroy the cookie on a definitively-dead session.
					// refreshOAuthToken already DELETEs the Redis session on a
					// permanent OAuth reject; contention/transient leaves it,
					// so a missing Redis key is our "permanent" signal. On a
					// transient/contention-timeout we keep the cookie so the
					// next request retries (matches kungal's behavior).
					if exists, _ := rdb.Exists(ctx, SessionPrefix+sessionID).Result(); exists == 0 {
						clearSessionCookie(c)
					}
					return response.Error(c, errors.ErrAuthExpired())
				}
			}
		} else if session.OAuthExpiresAt > 0 && now >= session.OAuthExpiresAt-300 {
			go func(s SessionData) {
				if err := refreshOAuthToken(context.Background(), rdb, oauthCfg, sessionID, &s); err != nil {
					slog.Warn("OAuth background refresh failed", "error", err)
				}
			}(session)
		}

		c.Locals(userContextKey, &session.UserInfo)
		c.Locals(rolesContextKey, decodeJWTRoles(session.OAuthAccessToken))
		c.Locals(accessTokenContextKey, session.OAuthAccessToken)
		return c.Next()
	}
}

func OptionalAuth(rdb *redis.Client, oauthCfg config.OAuthConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionID := c.Cookies(SessionCookieName)
		if sessionID == "" {
			return c.Next()
		}

		ctx := c.Context()
		data, err := rdb.Get(ctx, SessionPrefix+sessionID).Result()
		if err != nil {
			return c.Next()
		}

		var session SessionData
		if err := json.Unmarshal([]byte(data), &session); err != nil {
			return c.Next()
		}

		// Same two-tier policy as Auth, but on hard-expired-and-cannot-refresh
		// we degrade to anonymous (continue without user context) instead of
		// returning an error -- OptionalAuth's contract is "best effort".
		now := time.Now().Unix()
		if session.OAuthExpiresAt > 0 && now >= session.OAuthExpiresAt {
			if err := refreshOAuthToken(ctx, rdb, oauthCfg, sessionID, &session); err != nil {
				if stderrors.Is(err, errRefreshLockContended) &&
					waitForRefreshedSession(ctx, rdb, sessionID, &session) {
					// fresh session loaded; fall through with user context.
				} else {
					// OptionalAuth contract is best-effort: only drop the
					// cookie when the session is definitively dead (Redis
					// key gone = permanent reject); otherwise just degrade
					// to anonymous and keep the cookie for a later retry.
					if exists, _ := rdb.Exists(ctx, SessionPrefix+sessionID).Result(); exists == 0 {
						clearSessionCookie(c)
					}
					return c.Next()
				}
			}
		} else if session.OAuthExpiresAt > 0 && now >= session.OAuthExpiresAt-300 {
			go func(s SessionData) {
				if err := refreshOAuthToken(context.Background(), rdb, oauthCfg, sessionID, &s); err != nil {
					slog.Warn("OAuth background refresh failed", "error", err)
				}
			}(session)
		}

		c.Locals(userContextKey, &session.UserInfo)
		c.Locals(rolesContextKey, decodeJWTRoles(session.OAuthAccessToken))
		c.Locals(accessTokenContextKey, session.OAuthAccessToken)
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

func GetUserID(c *fiber.Ctx) int {
	user := GetUser(c)
	if user == nil {
		return 0
	}
	return user.ID
}

// GetAccessToken returns the OAuth access_token bound to the current session
// (empty string if no session). Handlers that proxy write operations to
// upstream services (e.g. the Galgame Wiki Service) forward this token as
// `Authorization: Bearer ...` so the upstream can validate user identity and
// apply its own creator/admin authorization.
func GetAccessToken(c *fiber.Ctx) string {
	v, ok := c.Locals(accessTokenContextKey).(string)
	if !ok {
		return ""
	}
	return v
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

// oauthRefreshHTTP is the timeout-bound HTTP client used for OAuth
// /oauth/token refresh calls in the auth middleware. Shared (and reused)
// across middleware invocations because the middleware itself is constructed
// per-route and would otherwise allocate a fresh client every call.
// 10s is consistent with the wiki/userclient transports.
var oauthRefreshHTTP = &http.Client{Timeout: 10 * time.Second}

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

// refreshOAuthToken performs one OAuth refresh round-trip and persists the
// new tokens back to Redis on success. Semantics:
//
//   - Returns nil on success. The passed-in *session pointer is mutated to
//     the new tokens, and the Redis session blob is rewritten.
//   - Returns a non-nil error in all other cases. Two distinct buckets:
//   - PERMANENT: OAuth says the refresh_token is invalid / revoked / expired
//     (4xx). The Redis session is DELETED as a side effect — any future
//     request bearing this cookie should fail closed.
//   - TRANSIENT: network errors, 5xx, JSON decode errors. The Redis session
//     is left intact so a subsequent retry can succeed.
//
// A per-session Redis lock (`lock:refresh:<sid>`, 30s TTL) prevents
// concurrent refreshes; a lock miss returns a transient error so callers can
// re-read the (possibly already-refreshed) session.
// errRefreshLockContended means another in-flight request already holds the
// per-session refresh lock and is doing the OAuth round-trip right now. This
// is the NORMAL case under SSR concurrency (one page load fans out into many
// parallel API calls that all hit hard-expiry together). It is NOT a refresh
// failure — the caller must wait for the winner to publish the new session
// and then proceed, NOT clear the cookie / kick the user. Treating contention
// as a hard failure was the moyu-specific logout bug (kungal's middleware
// already waits for the winner; moyu didn't).
var errRefreshLockContended = stderrors.New("refresh lock contended")

// waitForRefreshedSession is the lock-loser path. Another request holds the
// per-session refresh lock and is doing the OAuth round-trip; we poll Redis
// for it to publish the new session blob, then load it into *session and
// return true so the caller can proceed with the fresh tokens.
//
// Returns false if: the session disappears (winner hit a permanent OAuth
// reject and deleted it), or the deadline passes (winner still in flight /
// died). The 3s deadline sits below the 30s lock TTL and the 10s OAuth HTTP
// timeout's realistic p99 (refresh is normally <500ms), with a 100ms poll
// for sub-second hand-off. Mirrors kungal's waitForRefresh.
func waitForRefreshedSession(ctx context.Context, rdb *redis.Client, sessionID string, session *SessionData) bool {
	prevExpiresAt := session.OAuthExpiresAt
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)

		data, err := rdb.Get(ctx, SessionPrefix+sessionID).Result()
		if err != nil {
			// redis.Nil → winner deleted it (permanent reject). Any other
			// error → can't tell; bail and let the caller fail closed.
			return false
		}
		var fresh SessionData
		if json.Unmarshal([]byte(data), &fresh) != nil {
			continue
		}
		// Winner published when OAuthExpiresAt advanced past what we read.
		if fresh.OAuthExpiresAt > prevExpiresAt {
			*session = fresh
			return true
		}
	}
	return false
}

func refreshOAuthToken(ctx context.Context, rdb *redis.Client, oauthCfg config.OAuthConfig, sessionID string, session *SessionData) error {
	lockKey := "lock:refresh:" + sessionID
	ok, err := rdb.SetArgs(ctx, lockKey, 1, redis.SetArgs{
		TTL:  30 * time.Second,
		Mode: "NX",
	}).Result()
	if err != nil || ok != "OK" {
		return errRefreshLockContended
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		oauthCfg.ServerURL+"/oauth/token", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := oauthRefreshHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("oauth refresh transport: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var env struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int64  `json:"expires_in"`
		} `json:"data"`
	}
	_ = json.Unmarshal(respBody, &env)

	// Permanent reject: 401/403 (RFC 6749 invalid_grant style) or the
	// OAuth-side business codes that mean "this session can never refresh":
	//   10002 invalid token / 10003 token expired / 15003 invalid auth code /
	//   10014 account banned / 15005 grant-type not enabled / 15008 invalid
	//   client secret.
	// Per docs/oauth/README.md, OAuth returns some of these as HTTP 200 with a
	// non-zero `code` (business 401 → HTTP 200), so we MUST match on the code
	// and not rely on the HTTP status — otherwise these dead sessions fall
	// through to the transient branch below and retry every request forever
	// instead of being cleared (audit F077).
	// In every case we destroy the local session — there is no recovery.
	if resp.StatusCode == http.StatusUnauthorized ||
		resp.StatusCode == http.StatusForbidden ||
		env.Code == 10002 || env.Code == 10003 || env.Code == 15003 ||
		env.Code == 10014 || env.Code == 15005 || env.Code == 15008 {
		slog.Warn("OAuth refresh permanently rejected; destroying session",
			"status", resp.StatusCode, "code", env.Code, "msg", env.Message)
		rdb.Del(ctx, SessionPrefix+sessionID)
		return fmt.Errorf("refresh permanently rejected (status=%d code=%d)", resp.StatusCode, env.Code)
	}
	// Transient failure: leave the session for a future retry.
	if resp.StatusCode != 200 {
		return fmt.Errorf("oauth refresh status=%d body=%s", resp.StatusCode, truncate(string(respBody), 200))
	}
	if env.Code != 0 {
		return fmt.Errorf("oauth refresh code=%d msg=%s", env.Code, env.Message)
	}

	session.OAuthAccessToken = env.Data.AccessToken
	session.OAuthRefreshToken = env.Data.RefreshToken
	session.OAuthExpiresAt = time.Now().Unix() + env.Data.ExpiresIn

	blob, _ := json.Marshal(session)
	return rdb.Set(ctx, SessionPrefix+sessionID, blob, SessionTTL).Err()
}

// clearSessionCookie wipes the kun_session cookie on the response. Used
// when we reject a request because the upstream OAuth refresh permanently
// failed -- the cookie no longer points to a valid Redis session so it
// shouldn't keep being presented.
func clearSessionCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   SecureCookies,
		SameSite: "Lax",
		Path:     "/",
	})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
