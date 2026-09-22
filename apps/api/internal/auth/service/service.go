package service

import (
	"bytes"
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

func (s *AuthService) ListEcosystem() []EcosystemApp {
	s.ecoMu.RLock()
	apps, fetched := s.ecoApps, s.ecoFetched
	s.ecoMu.RUnlock()
	if !fetched.IsZero() && time.Since(fetched) < ecosystemTTL {
		return apps
	}

	fresh, err := s.fetchEcosystem()
	if err != nil {
		slog.Warn("OAuth ecosystem refetch failed; serving stale cache", "error", err, "stale_count", len(apps))
		return apps
	}

	s.ecoMu.Lock()
	s.ecoApps, s.ecoFetched = fresh, time.Now()
	s.ecoMu.Unlock()
	return fresh
}

func (s *AuthService) fetchEcosystem() ([]EcosystemApp, error) {
	req, err := http.NewRequest(http.MethodGet, s.oauthCfg.ServerURL+"/oauth/ecosystem", nil)
	if err != nil {
		return nil, fmt.Errorf("build oauth ecosystem request: %w", err)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth ecosystem request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
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

var ErrUserBanned = errors.New("oauth user banned")

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

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrUserBanned
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("OAuth userinfo request failed (%d): %s", resp.StatusCode, string(respBody))
	}
	var info OAuthUserInfo
	if err := json.Unmarshal(respBody, &info); err != nil {
		return nil, fmt.Errorf("decode userinfo: %w", err)
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
	if err := s.oauthPostJSON("/oauth/revoke", map[string]string{"token": token}, nil); err != nil {
		slog.Error("OAuth revoke failed", "error", err)
	}
}

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

	var oauthErr struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(respBody, &oauthErr)

	if resp.StatusCode != 200 {
		msg := oauthErr.ErrorDescription
		if msg == "" {
			msg = oauthErr.Error
		}
		if msg == "" {
			msg = truncate(string(respBody), 500)
		}
		return fmt.Errorf("OAuth %s failed (%d): %s", path, resp.StatusCode, msg)
	}

	respPayload := json.RawMessage(respBody)
	if out == nil || len(respPayload) == 0 {
		return nil
	}
	if err := json.Unmarshal(respPayload, out); err != nil {
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

// PreferencesNamespace is moyu's slot in the account-wide preferences KV. It is
// this app's OAuth client_id and nothing else: an OAuth token may only touch the
// namespace named by its own client_id (or the literal `global`), so a
// hand-written name is a 403/18003 from upstream.
func (s *AuthService) PreferencesNamespace() string {
	return s.oauthCfg.ClientID
}

func (s *AuthService) ProxyUserToOAuth(
	method, path, accessToken string,
	body []byte,
	contentType string,
) (status int, raw []byte, err error) {
	return s.ProxyUserToOAuthWithHeaders(method, path, accessToken, body, contentType, nil)
}

func (s *AuthService) ProxyUserToOAuthWithHeaders(
	method, path, accessToken string,
	body []byte,
	contentType string,
	headers map[string]string,
) (status int, raw []byte, err error) {
	var rdr io.Reader
	if len(body) > 0 {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, s.oauthCfg.ServerURL+path, rdr)
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
		return 0, nil, fmt.Errorf("oauth %s %s transport: %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ = io.ReadAll(resp.Body)
	return resp.StatusCode, raw, nil
}
