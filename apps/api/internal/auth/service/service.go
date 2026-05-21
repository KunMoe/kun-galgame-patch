// Package service holds AuthService: OAuth code/token exchange,
// /oauth/userinfo retrieval, local user provisioning at first login,
// and OAuth token revocation.
//
// After the OAuth migration:
//   - The OAuth server is the single source of truth for identity.
//   - Local user.id == OAuth.users.id (aligned by migrate-users).
//   - There is no oauth_account indirection table; we look up the local user
//     directly by the integer id returned in /oauth/userinfo.
//   - Password / email / verification-code logic lives entirely on the OAuth
//     server. Site-side has no password handling.
package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	authModel "kun-galgame-patch-api/internal/auth/model"
	"kun-galgame-patch-api/internal/auth/repository"
	"kun-galgame-patch-api/pkg/config"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type AuthService struct {
	repo     *repository.AuthRepository
	rdb      *redis.Client
	oauthCfg config.OAuthConfig
	http     *http.Client
}

func New(repo *repository.AuthRepository, rdb *redis.Client, oauthCfg config.OAuthConfig) *AuthService {
	return &AuthService{
		repo:     repo,
		rdb:      rdb,
		oauthCfg: oauthCfg,
		// All OAuth-server round-trips share one timeout-bound client so a
		// slow/dead OAuth server cannot tie up login / userinfo / revoke
		// indefinitely (audit finding: http.Post / http.DefaultClient were
		// previously used here without a timeout).
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

// OAuthTokenResponse is the OAuth /oauth/token response payload.
type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// OAuthUserInfo is the /oauth/userinfo payload. ID is the integer primary key
// in OAuth.users (and, after migrate-users, the same integer used as the
// local user.id). Sub is the OIDC UUID. Roles is the OAuth-side role set
// (e.g. ["user", "admin"]).
type OAuthUserInfo struct {
	ID      int      `json:"id"`
	Sub     string   `json:"sub"`
	Name    string   `json:"name"`
	Email   string   `json:"email"`
	Picture string   `json:"picture"`
	Roles   []string `json:"roles"`
}

// ExchangeCode trades an authorization code (+ PKCE verifier) for tokens.
//
// KUN OAuth takes a JSON body, not RFC-6749 form-encoded, and wraps the
// response in {code, message, data}. See docs/oauth/oauth-integration-guide.md.
func (s *AuthService) ExchangeCode(code, codeVerifier string) (*OAuthTokenResponse, error) {
	var tokenResp OAuthTokenResponse
	err := s.oauthPostJSON("/oauth/token", map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"code_verifier": codeVerifier,
		"client_id":     s.oauthCfg.ClientID,
		"client_secret": s.oauthCfg.ClientSecret,
		"redirect_uri":  s.oauthCfg.RedirectURI,
	}, &tokenResp)
	if err != nil {
		return nil, err
	}
	return &tokenResp, nil
}

// ErrUserBanned is returned by GetUserInfo when the OAuth server signals
// the account is banned (HTTP 403 + envelope code 10014). The caller MUST
// translate this into errors.ErrAccountBanned so the frontend can redirect
// to a banned-account page rather than the login page.
var ErrUserBanned = errors.New("oauth user banned")

// GetUserInfo fetches the current user's identity from /oauth/userinfo.
func (s *AuthService) GetUserInfo(accessToken string) (*OAuthUserInfo, error) {
	req, err := http.NewRequest(http.MethodGet, s.oauthCfg.ServerURL+"/oauth/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OAuth userinfo request failed: %w", err)
	}
	defer resp.Body.Close()

	// Parse the envelope eagerly so we can detect business codes (e.g. 10014
	// banned) that ship under non-2xx responses.
	respBody, _ := io.ReadAll(resp.Body)
	var env struct {
		Code    int           `json:"code"`
		Message string        `json:"message"`
		Data    OAuthUserInfo `json:"data"`
	}
	_ = json.Unmarshal(respBody, &env)

	if env.Code == 10014 || resp.StatusCode == http.StatusForbidden {
		return nil, ErrUserBanned
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("OAuth userinfo request failed (%d): %s", resp.StatusCode, string(respBody))
	}
	if env.Code != 0 {
		return nil, fmt.Errorf("OAuth userinfo error code=%d: %s", env.Code, env.Message)
	}
	out := env.Data
	return &out, nil
}

// FindOrCreateUserByID looks up the local user row by id, inserting an empty
// site-local row if it does not exist. The id MUST be the integer returned
// by /oauth/userinfo -- that's the contract that ties this site's user.id to
// OAuth.users.id (aligned by migrate-users).
//
// We do NOT copy name/email/avatar/etc. into the local row -- those fields
// live on OAuth and the local row only carries site-local state (moemoepoint,
// daily counters, follow counts, ...). Phase 5-6 will drop the OAuth-managed
// columns from this struct via migration 005.
func (s *AuthService) FindOrCreateUserByID(id int) (*authModel.User, error) {
	user, err := s.repo.FindUserByID(id)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	newUser := &authModel.User{ID: id}
	if err := s.repo.CreateUser(newUser); err != nil {
		return nil, fmt.Errorf("failed to create local user row: %w", err)
	}
	slog.Info("Provisioned local user row", "userID", id)
	return newUser, nil
}

// RevokeOAuthToken is fire-and-forget per RFC 7009: the endpoint always returns
// 200 regardless of whether the token was valid, so we only log transport-level
// failures.
func (s *AuthService) RevokeOAuthToken(token string) {
	if err := s.oauthPostJSON("/oauth/revoke", map[string]string{"token": token}, nil); err != nil {
		slog.Error("OAuth revoke failed", "error", err)
	}
}

// oauthPostJSON POSTs JSON to the OAuth server and decodes the wrapped
// {code, message, data} envelope into out (when not nil). A non-zero envelope
// code is surfaced as an error.
func (s *AuthService) oauthPostJSON(path string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode oauth request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, s.oauthCfg.ServerURL+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build oauth %s request: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("OAuth %s request failed: %w", path, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("OAuth %s failed (%d): %s", path, resp.StatusCode, truncate(string(respBody), 500))
	}

	var env struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(respBody, &env); err != nil {
		return fmt.Errorf("decode oauth envelope: %w (body=%s)", err, truncate(string(respBody), 200))
	}
	if env.Code != 0 {
		return fmt.Errorf("OAuth %s error code=%d: %s", path, env.Code, env.Message)
	}
	if out == nil || len(env.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("decode oauth data: %w", err)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
