package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	authModel "kun-galgame-patch-api/internal/auth/model"
	"kun-galgame-patch-api/internal/auth/repository"
	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/upstream"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const ecosystemTTL = 10 * time.Minute

type AuthService struct {
	repo     *repository.AuthRepository
	rdb      *redis.Client
	oauthCfg config.OAuthConfig
	http     *http.Client

	ecoMu      sync.RWMutex
	ecoApps    []EcosystemApp
	ecoFetched time.Time
}

func New(repo *repository.AuthRepository, rdb *redis.Client, oauthCfg config.OAuthConfig) *AuthService {
	return &AuthService{
		repo:     repo,
		rdb:      rdb,
		oauthCfg: oauthCfg,
		http:     &http.Client{Timeout: 10 * time.Second},
	}
}

type EcosystemApp struct {
	Name        string `json:"name"`
	SiteDomain  string `json:"site_domain"`
	LogoURL     string `json:"logo_url,omitempty"`
	Tagline     string `json:"tagline,omitempty"`
	AutoConsent bool   `json:"auto_consent"`
}

func (s *AuthService) ListEcosystem(ctx context.Context) []EcosystemApp {
	s.ecoMu.RLock()
	apps, fetched := s.ecoApps, s.ecoFetched
	s.ecoMu.RUnlock()
	if !fetched.IsZero() && time.Since(fetched) < ecosystemTTL {
		return apps
	}

	fresh, err := s.fetchEcosystem(ctx)
	if err != nil {
		slog.Warn("OAuth ecosystem refetch failed; serving stale cache", "error", err, "stale_count", len(apps))
		return apps
	}

	s.ecoMu.Lock()
	s.ecoApps, s.ecoFetched = fresh, time.Now()
	s.ecoMu.Unlock()
	return fresh
}

func (s *AuthService) fetchEcosystem(ctx context.Context) ([]EcosystemApp, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.oauthCfg.ServerURL+"/oauth/ecosystem", nil)
	if err != nil {
		return nil, fmt.Errorf("build oauth ecosystem request: %w", err)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth ecosystem request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := upstream.ReadBody(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read oauth ecosystem: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth ecosystem failed (%d): %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	var env struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Apps []EcosystemApp `json:"apps"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &env); err != nil {
		return nil, fmt.Errorf("decode oauth ecosystem envelope: %w (body=%s)", err, truncate(string(respBody), 200))
	}
	if env.Code != 0 {
		return nil, fmt.Errorf("oauth ecosystem error code=%d: %s", env.Code, env.Message)
	}
	return env.Data.Apps, nil
}

type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type OAuthUserInfo struct {
	ID        int      `json:"id"`
	Sub       string   `json:"sub"`
	Name      string   `json:"name"`
	Email     string   `json:"email"`
	Picture   string   `json:"picture"`
	Roles     []string `json:"roles"`
	SiteRoles []string `json:"site_roles"`
	// Both claims ride the `profile` scope moyu already holds, so reading the
	// account's content stance needs no new grant. AdultConfirmed is the half a
	// reader forgets: the migration backfilled nsfw_display='blur' on accounts
	// that never attested, so NsfwDisplay alone says "blur" for almost everyone
	// while the effective stance is "hide".
	AdultConfirmed bool   `json:"adult_confirmed"`
	NsfwDisplay    string `json:"nsfw_display"`
}

func (s *AuthService) ExchangeCode(ctx context.Context, code, codeVerifier string) (*OAuthTokenResponse, error) {
	payload, _ := json.Marshal(map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"code_verifier": codeVerifier,
		"client_id":     s.oauthCfg.ClientID,
		"client_secret": s.oauthCfg.ClientSecret,
		"redirect_uri":  s.oauthCfg.RedirectURI,
	})
	resp, body, err := s.oauthPost(ctx, "exchange", "/oauth/token", payload)
	if err != nil {
		return nil, err
	}
	var tok struct {
		OAuthTokenResponse
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	decodeErr := json.Unmarshal(body, &tok)
	if resp.StatusCode == http.StatusOK && decodeErr == nil && tok.AccessToken != "" {
		return &tok.OAuthTokenResponse, nil
	}
	return nil, tokenEndpointError("exchange", resp, tok.Error, tok.ErrorDescription)
}

// tokenEndpointError reads /oauth/token's RFC 6749 answer. invalid_grant is the
// reader's code (spent, expired, or minted for another verifier); every other
// 4xx is about moyu's own client registration or secret.
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

var ErrUserBanned = errors.New("oauth user banned")

// GetUserInfo answers ErrUserBanned for the 403 infra's BearerAuth gives a
// banned account. A 401 is not the reader's: moyu holds this token and has just
// judged it unexpired.
func (s *AuthService) GetUserInfo(ctx context.Context, accessToken string) (*OAuthUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.oauthCfg.ServerURL+"/oauth/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, upstream.Transport("oauth", "userinfo", err)
	}
	defer resp.Body.Close()
	body, err := upstream.ReadBody(resp.Body)
	if err != nil {
		return nil, upstream.Transport("oauth", "userinfo", err)
	}

	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrUserBanned
	}
	if resp.StatusCode != http.StatusOK {
		var bearer struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		_ = json.Unmarshal(body, &bearer)
		kind := upstream.Internal
		switch {
		case resp.StatusCode >= 500:
			kind = upstream.Unavailable
		case resp.StatusCode == http.StatusTooManyRequests:
			kind = upstream.RateLimited
		}
		return nil, &upstream.Error{
			Service: "oauth", Op: "userinfo", Kind: kind, Status: resp.StatusCode,
			Code: bearer.Error, Detail: bearer.ErrorDescription,
			RequestID: resp.Header.Get("X-Request-ID"), RetryAfter: upstream.RetryAfter(resp.Header),
		}
	}
	var info OAuthUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, &upstream.Error{Service: "oauth", Op: "userinfo", Kind: upstream.Internal,
			Status: resp.StatusCode, Cause: fmt.Errorf("decode userinfo: %w", err)}
	}
	return &info, nil
}

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
	if persisted, ferr := s.repo.FindUserByID(id); ferr == nil {
		return persisted, nil
	}
	slog.Info("Provisioned local user row", "userID", id)
	return newUser, nil
}

func (s *AuthService) RevokeOAuthToken(token string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	payload, _ := json.Marshal(map[string]string{"token": token})
	resp, _, err := s.oauthPost(ctx, "revoke", "/oauth/revoke", payload)
	if err != nil {
		slog.Error("OAuth revoke failed", "error", err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		slog.Error("OAuth revoke refused", "status", resp.StatusCode)
	}
}

func (s *AuthService) oauthPost(ctx context.Context, op, path string, payload []byte) (*http.Response, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.oauthCfg.ServerURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, nil, fmt.Errorf("build oauth %s request: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, nil, upstream.Transport("oauth", op, err)
	}
	defer resp.Body.Close()
	body, err := upstream.ReadBody(resp.Body)
	if err != nil {
		return nil, nil, upstream.Transport("oauth", op, err)
	}
	return resp, body, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// PreferencesNamespace is moyu's slot in the account-wide preferences KV. It is
// this app's OAuth client_id and nothing else: an OAuth token may only touch the
// namespace named by its own client_id (or the literal `global`), so a
// hand-written name is a 403/18003 from upstream.
func (s *AuthService) PreferencesNamespace() string {
	return s.oauthCfg.ClientID
}

func (s *AuthService) ProxyUserToOAuth(
	ctx context.Context,
	method, path, accessToken string,
	body []byte,
	contentType string,
) (status int, raw []byte, err error) {
	return s.ProxyUserToOAuthWithHeaders(ctx, method, path, accessToken, body, contentType, nil)
}

func (s *AuthService) ProxyUserToOAuthWithHeaders(
	ctx context.Context,
	method, path, accessToken string,
	body []byte,
	contentType string,
	headers map[string]string,
) (status int, raw []byte, err error) {
	var rdr io.Reader
	if len(body) > 0 {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.oauthCfg.ServerURL+path, rdr)
	if err != nil {
		return 0, nil, fmt.Errorf("build oauth %s %s: %w", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	if len(body) > 0 {
		if contentType == "" {
			contentType = "application/json"
		}
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return 0, nil, upstream.Transport("oauth", method+" "+path, err)
	}
	defer resp.Body.Close()
	raw, err = upstream.ReadBody(resp.Body)
	if err != nil {
		return 0, nil, upstream.Transport("oauth", method+" "+path, err)
	}
	return resp.StatusCode, raw, nil
}
