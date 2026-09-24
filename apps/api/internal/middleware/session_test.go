package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/upstream"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
)

// The bodies are what infra's OAuthHandler.Token writes (nextmoe-infra
// apps/api/internal/platform/auth/handler/oauth_handler.go): protoErr answers
// 401 for invalid_client and 400 for every other error string, and
// protoServerError answers 500 server_error.
const (
	tokenOK                 = `{"access_token":"new-access","token_type":"Bearer","expires_in":900,"refresh_token":"new-refresh","scope":"openid profile"}`
	tokenInvalidGrant       = `{"error":"invalid_grant","error_description":"无效的令牌"}`
	tokenInvalidClient      = `{"error":"invalid_client","error_description":"无效的 client secret"}`
	tokenUnauthorizedClient = `{"error":"unauthorized_client","error_description":"客户端未被授权使用该授权类型"}`
	tokenServerError        = `{"error":"server_error","error_description":"操作失败"}`
)

type tokenFake struct {
	status int
	body   string
	hook   func()
	calls  int
}

func (f *tokenFake) serve(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls++
		if r.URL.Path != "/oauth/token" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if f.hook != nil {
			f.hook()
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(f.status)
		_, _ = w.Write([]byte(f.body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

type sessionRig struct {
	app *fiber.App
	rdb *redis.Client
	mr  *miniredis.Miniredis
	cfg config.OAuthConfig
}

func newSessionRig(t *testing.T, fake *tokenFake) *sessionRig {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	cfg := config.OAuthConfig{ClientID: "moyu", ClientSecret: "secret"}
	if fake != nil {
		cfg.ServerURL = fake.serve(t).URL
	} else {
		cfg.ServerURL = "http://127.0.0.1:1"
	}
	app := fiber.New()
	ok := func(c fiber.Ctx) error {
		return c.JSON(response.Response{Code: 0, Data: GetUserID(c)})
	}
	app.Get("/auth", Auth(rdb, cfg), ok)
	app.Get("/optional", OptionalAuth(rdb, cfg), ok)
	return &sessionRig{app: app, rdb: rdb, mr: mr, cfg: cfg}
}

func (r *sessionRig) putSession(t *testing.T, id string, expiresAt int64) {
	t.Helper()
	blob, _ := json.Marshal(SessionData{
		UserInfo:          UserInfo{ID: 42, Sub: "sub-42"},
		OAuthAccessToken:  "old-access",
		OAuthRefreshToken: "old-refresh",
		OAuthExpiresAt:    expiresAt,
	})
	if err := r.rdb.Set(context.Background(), SessionPrefix+id, blob, SessionTTL).Err(); err != nil {
		t.Fatal(err)
	}
}

func (r *sessionRig) get(t *testing.T, path, id string) (int, response.Response, *http.Response) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: id})
	resp, err := r.app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	var body response.Response
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body, resp
}

func (r *sessionRig) session(t *testing.T, id string) *SessionData {
	t.Helper()
	raw, err := r.rdb.Get(context.Background(), SessionPrefix+id).Bytes()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var s SessionData
	_ = json.Unmarshal(raw, &s)
	return &s
}

func clearsCookie(resp *http.Response) bool {
	for _, c := range resp.Cookies() {
		if c.Name == SessionCookieName && c.MaxAge < 0 {
			return true
		}
	}
	return false
}

func expired() int64 { return time.Now().Unix() - 10 }

func TestRefreshSuccessStoresTheRotatedTokens(t *testing.T) {
	rig := newSessionRig(t, &tokenFake{status: http.StatusOK, body: tokenOK})
	rig.putSession(t, "s1", expired())

	status, body, _ := rig.get(t, "/auth", "s1")
	if status != http.StatusOK || body.Code != 0 {
		t.Fatalf("got %d/%d, want 200/0", status, body.Code)
	}
	s := rig.session(t, "s1")
	if s == nil || s.OAuthAccessToken != "new-access" || s.OAuthRefreshToken != "new-refresh" {
		t.Fatalf("session after refresh = %+v", s)
	}
}

func TestOnlyInvalidGrantEndsTheSession(t *testing.T) {
	rig := newSessionRig(t, &tokenFake{status: http.StatusBadRequest, body: tokenInvalidGrant})
	rig.putSession(t, "s1", expired())

	status, body, resp := rig.get(t, "/auth", "s1")
	if status != http.StatusUnauthorized || body.Code != 40101 {
		t.Fatalf("got %d/%d, want 401/40101", status, body.Code)
	}
	if rig.session(t, "s1") != nil {
		t.Fatal("a refused refresh token must end the session")
	}
	if !clearsCookie(resp) {
		t.Fatal("the dead session's cookie must be cleared")
	}

	rig.putSession(t, "s2", expired())
	status, body, _ = rig.get(t, "/optional", "s2")
	if status != http.StatusOK || body.Data != float64(0) {
		t.Fatalf("optional auth over a dead session = %d/%v, want anonymous 200", status, body.Data)
	}
}

func TestMoyuCredentialFailuresKeepTheSession(t *testing.T) {
	for name, fake := range map[string]*tokenFake{
		"invalid_client":      {status: http.StatusUnauthorized, body: tokenInvalidClient},
		"unauthorized_client": {status: http.StatusBadRequest, body: tokenUnauthorizedClient},
	} {
		t.Run(name, func(t *testing.T) {
			rig := newSessionRig(t, fake)
			rig.putSession(t, "s1", expired())

			for _, path := range []string{"/auth", "/optional"} {
				status, body, resp := rig.get(t, path, "s1")
				if status != http.StatusInternalServerError || body.Code != 50000 {
					t.Fatalf("%s: got %d/%d, want 500/50000", path, status, body.Code)
				}
				if clearsCookie(resp) {
					t.Fatalf("%s: moyu's own credential must not clear the reader's cookie", path)
				}
			}
			if s := rig.session(t, "s1"); s == nil || s.OAuthRefreshToken != "old-refresh" {
				t.Fatalf("session = %+v, want it kept as it was", s)
			}
		})
	}
}

func TestTransientRefreshFailuresAnswer503(t *testing.T) {
	t.Run("server_error", func(t *testing.T) {
		rig := newSessionRig(t, &tokenFake{status: http.StatusInternalServerError, body: tokenServerError})
		rig.putSession(t, "s1", expired())
		for _, path := range []string{"/auth", "/optional"} {
			status, body, _ := rig.get(t, path, "s1")
			if status != http.StatusServiceUnavailable || body.Code == 40101 {
				t.Fatalf("%s: got %d/%d, want 503", path, status, body.Code)
			}
		}
		if rig.session(t, "s1") == nil {
			t.Fatal("a transient refresh failure must keep the session")
		}
	})
	t.Run("unreachable", func(t *testing.T) {
		rig := newSessionRig(t, nil)
		rig.putSession(t, "s1", expired())
		status, body, _ := rig.get(t, "/auth", "s1")
		if status != http.StatusServiceUnavailable || body.Code == 40101 {
			t.Fatalf("got %d/%d, want 503", status, body.Code)
		}
		if rig.session(t, "s1") == nil {
			t.Fatal("an unreachable OAuth must keep the session")
		}
	})
}

func TestSessionStoreFailureIsNotAnonymous(t *testing.T) {
	rig := newSessionRig(t, nil)
	rig.putSession(t, "s1", time.Now().Unix()+3600)
	rig.mr.Close()

	for _, path := range []string{"/auth", "/optional"} {
		status, body, resp := rig.get(t, path, "s1")
		if status != http.StatusServiceUnavailable || body.Code != 50300 {
			t.Fatalf("%s: got %d/%d, want 503/50300", path, status, body.Code)
		}
		if clearsCookie(resp) {
			t.Fatalf("%s: a Redis outage must not clear the cookie", path)
		}
	}
}

// A deploy that changes a SessionData field's type leaves blobs like this in
// Redis. Each one must end its own session, not 500 every request its reader
// makes for as long as the TTL keeps sliding.
func TestUndecodableSessionIsEnded(t *testing.T) {
	const blob = `{"id":42,"sub":"sub-42","oauth_access_token":"a","oauth_refresh_token":"r","oauth_expires_at":"soon"}`
	for _, tc := range []struct {
		path       string
		wantStatus int
		wantCode   int
	}{
		{"/auth", http.StatusUnauthorized, 40101},
		{"/optional", http.StatusOK, 0},
	} {
		t.Run(tc.path, func(t *testing.T) {
			rig := newSessionRig(t, nil)
			rig.rdb.Set(context.Background(), SessionPrefix+"s1", blob, SessionTTL)

			status, body, resp := rig.get(t, tc.path, "s1")
			if status != tc.wantStatus || body.Code != tc.wantCode {
				t.Fatalf("got %d/%d, want %d/%d", status, body.Code, tc.wantStatus, tc.wantCode)
			}
			if tc.path == "/optional" && body.Data != float64(0) {
				t.Fatalf("optional auth over an undecodable session = user %v, want anonymous", body.Data)
			}
			if !clearsCookie(resp) {
				t.Fatal("the undecodable session's cookie must be cleared")
			}
			if n, _ := rig.rdb.Exists(context.Background(), SessionPrefix+"s1").Result(); n != 0 {
				t.Fatal("the undecodable session must be deleted")
			}
		})
	}
}

func TestRefreshDoesNotResurrectALoggedOutSession(t *testing.T) {
	var rig *sessionRig
	fake := &tokenFake{status: http.StatusOK, body: tokenOK, hook: func() {
		rig.rdb.Del(context.Background(), SessionPrefix+"s1")
	}}
	rig = newSessionRig(t, fake)
	rig.putSession(t, "s1", expired())

	s := rig.session(t, "s1")
	err := refreshOAuthToken(context.Background(), rig.rdb, rig.cfg, "s1", s)
	if err != errNoSession {
		t.Fatalf("refresh over a logout = %v, want errNoSession", err)
	}
	if rig.session(t, "s1") != nil {
		t.Fatal("the refresh wrote the logged-out session back")
	}
}

func TestContendedRefreshWaitsForTheWinner(t *testing.T) {
	fake := &tokenFake{status: http.StatusOK, body: tokenOK}
	rig := newSessionRig(t, fake)
	rig.putSession(t, "s1", expired())
	rig.rdb.Set(context.Background(), "lock:refresh:s1", 1, time.Minute)

	go func() {
		time.Sleep(300 * time.Millisecond)
		blob, _ := json.Marshal(SessionData{
			UserInfo: UserInfo{ID: 42}, OAuthAccessToken: "winner", OAuthExpiresAt: time.Now().Unix() + 900,
		})
		rig.rdb.Set(context.Background(), SessionPrefix+"s1", blob, SessionTTL)
	}()

	status, body, _ := rig.get(t, "/auth", "s1")
	if status != http.StatusOK || body.Data != float64(42) {
		t.Fatalf("got %d/%v, want 200 as user 42", status, body.Data)
	}
	if fake.calls != 0 {
		t.Fatalf("the loser called /oauth/token %d times", fake.calls)
	}
}

func TestTokenEndpointErrorKinds(t *testing.T) {
	cases := []struct {
		status int
		code   string
		want   upstream.Kind
	}{
		{400, "invalid_grant", upstream.Rejected},
		{401, "invalid_client", upstream.Internal},
		{400, "unauthorized_client", upstream.Internal},
		{400, "invalid_request", upstream.Internal},
		{429, "", upstream.RateLimited},
		{502, "", upstream.Unavailable},
	}
	for _, tc := range cases {
		resp := &http.Response{StatusCode: tc.status, Header: http.Header{}}
		if got := tokenEndpointError("refresh", resp, tc.code, "").Kind; got != tc.want {
			t.Errorf("%d %s = kind %d, want %d", tc.status, tc.code, got, tc.want)
		}
	}
}
