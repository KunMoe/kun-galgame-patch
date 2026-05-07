package service

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"kun-galgame-patch-api/internal/auth/model"
	"kun-galgame-patch-api/internal/auth/repository"
	"kun-galgame-patch-api/internal/infrastructure/mail"
	"kun-galgame-patch-api/pkg/config"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/argon2"
	"gorm.io/gorm"
)

type AuthService struct {
	repo     *repository.AuthRepository
	rdb      *redis.Client
	mailer   *mail.Mailer
	oauthCfg config.OAuthConfig
}

func New(repo *repository.AuthRepository, rdb *redis.Client, mailer *mail.Mailer, oauthCfg config.OAuthConfig) *AuthService {
	return &AuthService{repo: repo, rdb: rdb, mailer: mailer, oauthCfg: oauthCfg}
}

// OAuthTokenResponse is the OAuth token response
type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// OAuthUserInfo is the OAuth user info
type OAuthUserInfo struct {
	Sub     string `json:"sub"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Picture string `json:"picture"`
}

// ExchangeCode exchanges an authorization code for a token.
//
// The KUN OAuth Server takes a JSON body (not the RFC-6749 form-urlencoded
// shape) and wraps the response in `{code, message, data}` — see
// docs/oauth/oauth-integration-guide.md.
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

// GetUserInfo retrieves OAuth user info using an access token. The response is
// wrapped in `{code, message, data: {sub, name, ...}}`.
func (s *AuthService) GetUserInfo(accessToken string) (*OAuthUserInfo, error) {
	req, err := http.NewRequest("GET", s.oauthCfg.ServerURL+"/oauth/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OAuth userinfo request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OAuth userinfo request failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var env struct {
		Code    int           `json:"code"`
		Message string        `json:"message"`
		Data    OAuthUserInfo `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("failed to decode userinfo: %w", err)
	}
	if env.Code != 0 {
		return nil, fmt.Errorf("OAuth userinfo error code=%d: %s", env.Code, env.Message)
	}
	out := env.Data
	return &out, nil
}

// oauthPostJSON POSTs a JSON body to the OAuth Server and decodes the
// `{code, message, data}` envelope into `out`. A non-zero envelope code is
// treated as an error so callers don't have to.
func (s *AuthService) oauthPostJSON(path string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode oauth request: %w", err)
	}
	resp, err := http.Post(
		s.oauthCfg.ServerURL+path,
		"application/json",
		bytes.NewReader(payload),
	)
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

// FindOrCreateUser finds or creates a local user (core OAuth login logic)
func (s *AuthService) FindOrCreateUser(oauthUser *OAuthUserInfo) (*model.User, error) {
	// 1. Look up oauth_account by sub
	account, err := s.repo.FindOAuthAccountBySub(oauthUser.Sub)
	if err == nil {
		user, err := s.repo.FindUserByID(account.UserID)
		if err != nil {
			return nil, err
		}
		if user.Status == 2 {
			return nil, fmt.Errorf("this account has been banned")
		}
		return user, nil
	}

	// 2. Look up user by email (legacy user migration)
	user, err := s.repo.FindUserByEmail(oauthUser.Email)
	if err == nil {
		// Legacy user, create oauth_account association
		if user.Status == 2 {
			return nil, fmt.Errorf("this account has been banned")
		}
		oauthAccount := &model.OAuthAccount{
			UserID:   user.ID,
			Provider: "kun-oauth",
			Sub:      oauthUser.Sub,
		}
		if err := s.repo.CreateOAuthAccount(oauthAccount); err != nil {
			slog.Error("failed to create OAuth account for existing user", "error", err, "userId", user.ID)
			return nil, fmt.Errorf("failed to link OAuth account")
		}
		slog.Info("Linked OAuth account to existing user", "userId", user.ID, "sub", oauthUser.Sub)
		return user, nil
	}

	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	// 3. Brand new user
	newUser := &model.User{
		Name:   oauthUser.Name,
		Email:  oauthUser.Email,
		Avatar: oauthUser.Picture,
		Role:   1,
		Status: 0,
	}
	if err := s.repo.CreateUser(newUser); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	oauthAccount := &model.OAuthAccount{
		UserID:   newUser.ID,
		Provider: "kun-oauth",
		Sub:      oauthUser.Sub,
	}
	if err := s.repo.CreateOAuthAccount(oauthAccount); err != nil {
		return nil, fmt.Errorf("failed to create OAuth association: %w", err)
	}

	slog.Info("Created new user via OAuth", "userId", newUser.ID, "sub", oauthUser.Sub)
	return newUser, nil
}

// RevokeOAuthToken revokes an OAuth token. Fire-and-forget — RFC 7009 says
// the endpoint always returns 200 regardless of whether the token was valid,
// so we only log transport-level failures.
func (s *AuthService) RevokeOAuthToken(token string) {
	if err := s.oauthPostJSON("/oauth/revoke", map[string]string{"token": token}, nil); err != nil {
		slog.Error("OAuth revoke failed", "error", err)
	}
}

// SendVerificationCode sends an email verification code
func (s *AuthService) SendVerificationCode(email string) error {
	ctx := context.Background()

	// Check send rate limit
	rateLimitKey := "sendCode:email:" + email
	if exists, _ := s.rdb.Exists(ctx, rateLimitKey).Result(); exists > 0 {
		return fmt.Errorf("verification code sent too frequently, please try again after 60 seconds")
	}

	code := fmt.Sprintf("%06d", rand.Intn(1000000))
	codeKey := "verificationCode:" + email

	s.rdb.Set(ctx, codeKey, code, 10*time.Minute)
	s.rdb.Set(ctx, rateLimitKey, 1, 60*time.Second)

	subject := "KUN Visual Novel Patch - Verification Code"
	body := fmt.Sprintf(`<p>Your verification code is: <strong>%s</strong></p><p>Valid for 10 minutes. Please do not share it with others.</p>`, code)

	return s.mailer.Send(email, subject, body)
}

// VerifyCode verifies an email verification code
func (s *AuthService) VerifyCode(email, code string) error {
	ctx := context.Background()
	codeKey := "verificationCode:" + email

	storedCode, err := s.rdb.Get(ctx, codeKey).Result()
	if err == redis.Nil {
		return fmt.Errorf("verification code does not exist or has expired")
	}
	if err != nil {
		return fmt.Errorf("verification code check failed")
	}

	if storedCode != code {
		return fmt.Errorf("incorrect verification code")
	}

	s.rdb.Del(ctx, codeKey)
	return nil
}

// HashPassword hashes a password using Argon2id
func (s *AuthService) HashPassword(password string) string {
	salt := make([]byte, 16)
	crand.Read(salt)
	hash := argon2.IDKey([]byte(password), salt, 2, 8192, 3, 32)
	return fmt.Sprintf("%x:%x", salt, hash)
}

// VerifyPassword verifies a password against its hash
func (s *AuthService) VerifyPassword(hashedPassword, password string) bool {
	parts := strings.SplitN(hashedPassword, ":", 2)
	if len(parts) != 2 {
		return false
	}

	salt, _ := hexDecode(parts[0])
	expectedHash, _ := hexDecode(parts[1])
	if salt == nil || expectedHash == nil {
		return false
	}

	hash := argon2.IDKey([]byte(password), salt, 2, 8192, 3, 32)

	if len(hash) != len(expectedHash) {
		return false
	}
	for i := range hash {
		if hash[i] != expectedHash[i] {
			return false
		}
	}
	return true
}

func hexDecode(s string) ([]byte, error) {
	return hex.DecodeString(s)
}
