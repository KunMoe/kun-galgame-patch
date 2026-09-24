package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"log/slog"
	"net/http"
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
	refreshWait       = 3 * time.Second
	refreshAheadOfExp = 300
)

var (
	errNoSession            = stderrors.New("session is gone")
	errSessionStore         = stderrors.New("session store")
	errRefreshLockContended = stderrors.New("refresh lock contended")
)

var oauthRefreshHTTP = &http.Client{Timeout: 10 * time.Second}

// errNoSession is the only answer that logs the reader out, so it is kept for a
// session that is over: absent, logged out, or refused as invalid_grant. A
// transient refresh failure used to answer it too, which the frontend turned
// into a logout.
func liveSession(c fiber.Ctx, rdb *redis.Client, oauthCfg config.OAuthConfig, sessionID string) (*SessionData, error) {
	ctx := c.Context()
	session, err := readSession(ctx, rdb, sessionID)
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
		go refreshInBackground(rdb, oauthCfg, sessionID, *session)
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
		return nil, fmt.Errorf("decode session %s: %w", sessionID[:min(8, len(sessionID))], err)
	}
	return &session, nil
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
	if err == nil || stderrors.Is(err, errRefreshLockContended) || stderrors.Is(err, errNoSession) {
		return
	}
	level := slog.LevelError
	if k := upstream.KindOf(err); k == upstream.Unavailable || k == upstream.RateLimited {
		level = slog.LevelWarn
	}
	slog.Log(context.Background(), level, "OAuth background refresh failed; the session keeps its token", "error", err)
}

func waitForRefreshedSession(ctx context.Context, rdb *redis.Client, sessionID string, session *SessionData) error {
	prevExpiresAt := session.OAuthExpiresAt
	deadline := time.Now().Add(refreshWait)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		fresh, err := readSession(ctx, rdb, sessionID)
		if err != nil {
			return err
		}
		if fresh.OAuthExpiresAt > prevExpiresAt {
			*session = *fresh
			return nil
		}
	}
	return &upstream.Error{Service: "oauth", Op: "refresh", Kind: upstream.Unavailable,
		Detail: "a concurrent refresh of this session did not finish in time"}
}

func refreshOAuthToken(ctx context.Context, rdb *redis.Client, oauthCfg config.OAuthConfig, sessionID string, session *SessionData) error {
	lockKey := "lock:refresh:" + sessionID
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

	// XX: a logout that landed while the refresh was in flight must stay a logout.
	blob, _ := json.Marshal(session)
	err = rdb.SetArgs(ctx, SessionPrefix+sessionID, blob, redis.SetArgs{Mode: "XX", TTL: SessionTTL}).Err()
	if err == redis.Nil {
		return errNoSession
	}
	if err != nil {
		slog.Warn("refreshed OAuth token not saved; this request uses it and a later one refreshes again",
			"userID", session.ID, "error", err)
	}
	return nil
}

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
