package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/upstream"

	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
)

const (
	refreshLockTTL    = 30 * time.Second
	refreshPoll       = 100 * time.Millisecond
	refreshAheadOfExp = 300
)

var (
	errNoSession            = stderrors.New("session is gone")
	errSessionUndecodable   = fmt.Errorf("undecodable: %w", errNoSession)
	errSessionStore         = stderrors.New("session store")
	errRefreshLockContended = stderrors.New("refresh lock contended")
	errSessionOwnerMismatch = stderrors.New("session key belongs to another user")
)

var oauthRefreshHTTP = &http.Client{Timeout: 10 * time.Second}

// errNoSession is the only answer that logs the reader out, so it is kept for a
// session that is over: absent, logged out, or refused as invalid_grant. A
// transient refresh failure used to answer it too, which the frontend turned
// into a logout.
func liveSession(c fiber.Ctx, rdb *redis.Client, oauthCfg config.OAuthConfig, sessionID string) (*SessionData, error) {
	ctx := c.Context()
	session, err := readSession(ctx, rdb, sessionID)
	if stderrors.Is(err, errSessionUndecodable) {
		clearSessionCookie(c)
	}
	if err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	switch {
	case session.OAuthExpiresAt == 0:
	case now >= session.OAuthExpiresAt:
		err := refreshOAuthToken(ctx, rdb, oauthCfg, sessionID, session)
		if stderrors.Is(err, errRefreshLockContended) {
			err = waitForRefreshedSession(ctx, rdb, sessionID, session)
		}
		if stderrors.Is(err, errNoSession) {
			clearSessionCookie(c)
		}
		if err != nil {
			return nil, err
		}
	case now >= session.OAuthExpiresAt-refreshAheadOfExp:
		// fiber hands out cookie values that alias the request buffer. The refresh
		// returns after the handler, when that buffer holds the next request on the
		// connection, so the uncloned id wrote this reader into another reader's
		// session: the 2026-10-22 account swaps.
		// ast-grep-ignore: no-go-in-fiber-handler -- the id is cloned at the call
		go refreshInBackground(rdb, oauthCfg, strings.Clone(sessionID), *session)
	}
	return session, nil
}

func readSession(ctx context.Context, rdb *redis.Client, sessionID string) (*SessionData, error) {
	data, err := rdb.Get(ctx, SessionPrefix+sessionID).Bytes()
	if err == redis.Nil {
		return nil, errNoSession
	}
	if err != nil {
		return nil, fmt.Errorf("%w: read session: %w", errSessionStore, err)
	}
	var session SessionData
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, dropUndecodableSession(ctx, rdb, sessionID, data, err)
	}
	return &session, nil
}

// An undecodable session is over, not broken: answered as a 500 it failed every
// request the reader made for the 90 days its TTL keeps sliding, and a deploy
// that changed a SessionData field's type would have done that to every
// signed-in reader at once.
func dropUndecodableSession(ctx context.Context, rdb *redis.Client, sessionID string, data []byte, decodeErr error) error {
	var owner struct {
		ID int `json:"id"`
	}
	_ = json.Unmarshal(data, &owner)
	slog.Error("session blob does not decode; ending the session",
		"sessionPrefix", sessionID[:min(8, len(sessionID))], "userID", owner.ID, "error", decodeErr)
	if err := rdb.Del(ctx, SessionPrefix+sessionID).Err(); err != nil {
		slog.Error("undecodable session not deleted", "error", err)
	}
	return errSessionUndecodable
}

func sessionFailure(c fiber.Ctx, err error) error {
	if _, ok := upstream.As(err); ok {
		return response.Upstream(c, err, "")
	}
	slog.Error("session unusable", "method", c.Method(), "path", c.Path(), "error", err)
	if stderrors.Is(err, errSessionStore) {
		return response.Error(c, errors.New(50300, "会话服务暂不可用，请稍后再试", fiber.StatusServiceUnavailable))
	}
	return response.Error(c, errors.ErrInternal(""))
}

func refreshInBackground(rdb *redis.Client, oauthCfg config.OAuthConfig, sessionID string, session SessionData) {
	err := refreshOAuthToken(context.Background(), rdb, oauthCfg, sessionID, &session)
	if err == nil || stderrors.Is(err, errRefreshLockContended) || stderrors.Is(err, errNoSession) || stderrors.Is(err, errSessionOwnerMismatch) {
		return
	}
	level := slog.LevelError
	if k := upstream.KindOf(err); k == upstream.Unavailable || k == upstream.RateLimited {
		level = slog.LevelWarn
	}
	slog.Log(context.Background(), level, "OAuth background refresh failed; the session keeps its token", "error", err)
}

// The winner holds the lock until its refresh returns, which can take the
// whole oauthRefreshHTTP timeout. A fixed 3s wait answered 79 readers 503 on
// 2026-10-22 while OAuth was slow but still answering.
func waitForRefreshedSession(ctx context.Context, rdb *redis.Client, sessionID string, session *SessionData) error {
	prevExpiresAt := session.OAuthExpiresAt
	deadline := time.Now().Add(oauthRefreshHTTP.Timeout + time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(refreshPoll):
		}
		// The lock is read before the session: the winner saves the session
		// before it releases the lock, so a released lock with no new token
		// means the winner failed.
		held, err := rdb.Exists(ctx, refreshLockKey(sessionID)).Result()
		if err != nil {
			return fmt.Errorf("%w: read refresh lock: %w", errSessionStore, err)
		}
		fresh, err := readSession(ctx, rdb, sessionID)
		if err != nil {
			return err
		}
		if fresh.OAuthExpiresAt > prevExpiresAt {
			*session = *fresh
			return nil
		}
		if held == 0 {
			return &upstream.Error{Service: "oauth", Op: "refresh", Kind: upstream.Unavailable,
				Detail: "a concurrent refresh of this session failed"}
		}
	}
	return &upstream.Error{Service: "oauth", Op: "refresh", Kind: upstream.Unavailable,
		Detail: "a concurrent refresh of this session did not finish in time"}
}

func refreshLockKey(sessionID string) string { return "lock:refresh:" + sessionID }

func refreshOAuthToken(ctx context.Context, rdb *redis.Client, oauthCfg config.OAuthConfig, sessionID string, session *SessionData) error {
	lockKey := refreshLockKey(sessionID)
	won, err := rdb.SetNX(ctx, lockKey, 1, refreshLockTTL).Result()
	if err != nil {
		return fmt.Errorf("%w: take refresh lock: %w", errSessionStore, err)
	}
	if !won {
		return errRefreshLockContended
	}
	defer rdb.Del(context.WithoutCancel(ctx), lockKey)

	tok, err := requestRefresh(ctx, oauthCfg, session.OAuthRefreshToken)
	if e, ok := upstream.As(err); ok && e.Kind == upstream.Rejected {
		slog.Warn("OAuth refused the refresh token; ending the session",
			"sessionPrefix", sessionID[:min(8, len(sessionID))], "userID", session.ID, "error", err)
		if derr := rdb.Del(ctx, SessionPrefix+sessionID).Err(); derr != nil {
			slog.Error("dead session not deleted", "error", derr)
		}
		return errNoSession
	}
	if err != nil {
		return err
	}

	session.OAuthAccessToken = tok.AccessToken
	session.OAuthRefreshToken = tok.RefreshToken
	session.OAuthExpiresAt = time.Now().Unix() + tok.ExpiresIn

	blob, _ := json.Marshal(session)
	res, err := writeBackSession.Run(ctx, rdb, []string{SessionPrefix + sessionID},
		blob, session.ID, SessionTTL.Milliseconds()).Int64Slice()
	if err != nil {
		slog.Warn("refreshed OAuth token not saved; this request uses it and a later one refreshes again",
			"userID", session.ID, "error", err)
		return nil
	}
	switch res[0] {
	case writeBackGone:
		return errNoSession
	case writeBackOtherOwner:
		slog.Error("refresh write-back refused: the session key belongs to another user",
			"sessionPrefix", sessionID[:min(8, len(sessionID))], "userID", session.ID, "ownerID", res[1])
		return errSessionOwnerMismatch
	}
	return nil
}

const (
	writeBackSaved = iota
	writeBackGone
	writeBackOtherOwner
)

// A refresh may only rewrite a session its own user still owns. A blind SET XX
// let a wrong key (the 2026-10-22 buffer aliasing) swap one reader into
// another's account; an absent key is a logout that landed mid-refresh.
var writeBackSession = redis.NewScript(`
local cur = redis.call('GET', KEYS[1])
if not cur then return {1, 0} end
local ok, s = pcall(cjson.decode, cur)
local owner = ok and type(s) == 'table' and tonumber(s.id) or 0
if owner ~= tonumber(ARGV[2]) then return {2, owner} end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[3])
return {0, owner}
`)

type oauthToken struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int64  `json:"expires_in"`
}

func requestRefresh(ctx context.Context, oauthCfg config.OAuthConfig, refreshToken string) (*oauthToken, error) {
	payload, _ := json.Marshal(map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     oauthCfg.ClientID,
		"client_secret": oauthCfg.ClientSecret,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		oauthCfg.ServerURL+"/oauth/token", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := oauthRefreshHTTP.Do(req)
	if err != nil {
		return nil, upstream.Transport("oauth", "refresh", err)
	}
	defer resp.Body.Close()
	body, err := upstream.ReadBody(resp.Body)
	if err != nil {
		return nil, upstream.Transport("oauth", "refresh", err)
	}

	var tok oauthToken
	decodeErr := json.Unmarshal(body, &tok)
	if resp.StatusCode == http.StatusOK && decodeErr == nil && tok.AccessToken != "" {
		return &tok, nil
	}
	return nil, tokenEndpointError("refresh", resp, tok.Error, tok.ErrorDescription)
}

// Only invalid_grant is about the reader's grant. docs/oauth/04 tells an RP to
// end the session on invalid_client and unauthorized_client as well, and this
// middleware did, but those are moyu's own secret and registered grants: a
// rotated KUN_OAUTH_CLIENT_SECRET would have logged out every reader as each
// token reached refresh.
func tokenEndpointError(op string, resp *http.Response, code, desc string) *upstream.Error {
	kind := upstream.Internal
	switch {
	case resp.StatusCode >= 500:
		kind = upstream.Unavailable
	case resp.StatusCode == http.StatusTooManyRequests:
		kind = upstream.RateLimited
	case code == "invalid_grant":
		kind = upstream.Rejected
	}
	return &upstream.Error{
		Service:    "oauth",
		Op:         op,
		Kind:       kind,
		Status:     resp.StatusCode,
		Code:       code,
		Detail:     desc,
		RequestID:  resp.Header.Get("X-Request-ID"),
		RetryAfter: upstream.RetryAfter(resp.Header),
	}
}
