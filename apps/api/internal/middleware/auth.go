package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"slices"
	"strings"
	"time"

	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
)

// ID is the same integer in OAuth and this database; never translate or renumber it.
type UserInfo struct {
	ID  int    `json:"id"`
	Sub string `json:"sub"`
}

type SessionData struct {
	UserInfo
	OAuthAccessToken  string `json:"oauth_access_token"`
	OAuthRefreshToken string `json:"oauth_refresh_token"`
	OAuthExpiresAt    int64  `json:"oauth_expires_at"`
}

const (
	// Cookie names and Redis prefixes must stay site-specific. Localhost cookies
	// ignore ports; sharing them caused client_id_mismatch logouts.
	SessionCookieName     = "moyu_session"
	SessionTTL            = 90 * 24 * time.Hour
	SessionPrefix         = "moyu:session:"
	sessionRenewPrefix    = "moyu:session-renew:"
	userContextKey        = "user"
	rolesContextKey       = "oauth_roles"
	siteRolesContextKey   = "oauth_site_roles"
	accessTokenContextKey = "oauth_access_token"
)

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
	return func(c fiber.Ctx) error {
		sessionID := c.Cookies(SessionCookieName)
		if sessionID == "" {
			return response.Error(c, errors.ErrUnauthorized())
		}
		session, err := liveSession(c, rdb, oauthCfg, sessionID)
		if stderrors.Is(err, errNoSession) {
			return response.Error(c, errors.ErrAuthExpired())
		}
		if err != nil {
			return sessionFailure(c, err)
		}
		attachSession(c, rdb, sessionID, session)
		return c.Next()
	}
}

// OptionalAuth reads a dead session as anonymous, and nothing else: a reader
// whose session could not be read is not a stranger. Read as one, an owner gets
// a stranger's view of their private folders, which is how 3953 readers saw an
// empty 收藏 tab when the route was registered without this middleware.
func OptionalAuth(rdb *redis.Client, oauthCfg config.OAuthConfig) fiber.Handler {
	return func(c fiber.Ctx) error {
		sessionID := c.Cookies(SessionCookieName)
		if sessionID == "" {
			return c.Next()
		}
		session, err := liveSession(c, rdb, oauthCfg, sessionID)
		if stderrors.Is(err, errNoSession) {
			return c.Next()
		}
		if err != nil {
			return sessionFailure(c, err)
		}
		attachSession(c, rdb, sessionID, session)
		return c.Next()
	}
}

func attachSession(c fiber.Ctx, rdb *redis.Client, sessionID string, session *SessionData) {
	renewSlidingSession(c, rdb, sessionID)

	roles, siteRoles := decodeJWTClaims(session.OAuthAccessToken)
	c.Locals(userContextKey, &session.UserInfo)
	c.Locals(rolesContextKey, roles)
	c.Locals(siteRolesContextKey, siteRoles)
	c.Locals(accessTokenContextKey, session.OAuthAccessToken)
}

func GetUser(c fiber.Ctx) *UserInfo {
	user, ok := c.Locals(userContextKey).(*UserInfo)
	if !ok {
		return nil
	}
	return user
}

func MustGetUser(c fiber.Ctx) *UserInfo {
	return c.Locals(userContextKey).(*UserInfo)
}

func GetUserID(c fiber.Ctx) int {
	user := GetUser(c)
	if user == nil {
		return 0
	}
	return user.ID
}

func GetAccessToken(c fiber.Ctx) string {
	v, ok := c.Locals(accessTokenContextKey).(string)
	if !ok {
		return ""
	}
	return v
}

func GetRoles(c fiber.Ctx) []string {
	v, ok := c.Locals(rolesContextKey).([]string)
	if !ok {
		return nil
	}
	return v
}

func GetSiteRoles(c fiber.Ctx) []string {
	v, ok := c.Locals(siteRolesContextKey).([]string)
	if !ok {
		return nil
	}
	return v
}

func mergeRoles(a, b []string) []string {
	if len(b) == 0 {
		return a
	}
	out := append([]string(nil), a...)
	for _, r := range b {
		if !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	return out
}

func effectiveRoles(c fiber.Ctx) []string {
	return mergeRoles(GetRoles(c), GetSiteRoles(c))
}

func HasRole(c fiber.Ctx, role string) bool {
	return slices.Contains(effectiveRoles(c), role)
}

func HasAnyRole(c fiber.Ctx, roles ...string) bool {
	if len(roles) == 0 {
		return GetUser(c) != nil
	}
	have := effectiveRoles(c)
	for _, want := range roles {
		if slices.Contains(have, want) {
			return true
		}
	}
	return false
}

var (
	SuperAdminRoles = []string{"admin", "ren"}
	ModeratorRoles  = []string{"admin", "ren", "moderator"}
)

func IsAdmin(c fiber.Ctx) bool { return HasAnyRole(c, SuperAdminRoles...) }

func IsModerator(c fiber.Ctx) bool { return HasAnyRole(c, ModeratorRoles...) }

var SecureCookies = true

func CreateSession(c fiber.Ctx, rdb *redis.Client, session *SessionData) error {
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

func renewSlidingSession(c fiber.Ctx, rdb *redis.Client, sessionID string) {
	ctx := c.Context()
	if ok, _ := rdb.SetNX(ctx, sessionRenewPrefix+sessionID, "1", SessionTTL/2).Result(); !ok {
		return
	}
	rdb.Expire(ctx, SessionPrefix+sessionID, SessionTTL)
	c.Cookie(&fiber.Cookie{
		Name:     SessionCookieName,
		Value:    sessionID,
		MaxAge:   int(SessionTTL.Seconds()),
		HTTPOnly: true,
		Secure:   SecureCookies,
		SameSite: "Lax",
		Path:     "/",
	})
}

func DestroySession(c fiber.Ctx, rdb *redis.Client) error {
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

func decodeJWTClaims(token string) (roles, siteRoles []string) {
	if token == "" {
		return nil, nil
	}
	parts := strings.SplitN(token, ".", 3)
	if len(parts) < 2 {
		return nil, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, nil
		}
	}
	var claims struct {
		Roles     []string `json:"roles"`
		SiteRoles []string `json:"site_roles"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, nil
	}
	return claims.Roles, claims.SiteRoles
}

func clearSessionCookie(c fiber.Ctx) {
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
