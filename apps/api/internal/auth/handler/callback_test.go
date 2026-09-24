package handler

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"kun-galgame-patch-api/internal/auth/service"
	"kun-galgame-patch-api/internal/testutil"
	"kun-galgame-patch-api/pkg/config"
)

type oauthCallbackFake struct {
	tokenStatus    int
	tokenReply     string
	userinfoStatus int
	userinfoReply  string
	revoked        atomic.Int32
}

// Replies are infra's: OAuthHandler.Token for /oauth/token and BearerAuth /
// OAuthHandler.UserInfo for /oauth/userinfo.
func (f *oauthCallbackFake) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/token":
			w.WriteHeader(f.tokenStatus)
			_, _ = w.Write([]byte(f.tokenReply))
		case "/oauth/userinfo":
			w.WriteHeader(f.userinfoStatus)
			_, _ = w.Write([]byte(f.userinfoReply))
		case "/oauth/revoke":
			f.revoked.Add(1)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func postCallback(t *testing.T, f *oauthCallbackFake) (int, int) {
	t.Helper()
	svc := service.New(nil, nil, config.OAuthConfig{ServerURL: f.serve(t), ClientID: "moyu", ClientSecret: "s"})
	h := New(svc, nil, nil, nil, nil)
	ta := testutil.NewTestApp(t)
	ta.App.Post("/auth/oauth/callback", h.OAuthCallback)

	resp := ta.Request(t, http.MethodPost, "/auth/oauth/callback", `{"code":"c","code_verifier":"v"}`, "")
	return resp.StatusCode, testutil.ParseResponse(t, resp).Code
}

const tokenMinted = `{"access_token":"a","token_type":"Bearer","expires_in":900,"refresh_token":"r","scope":"openid profile"}`

func TestCallbackExchangeFailures(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		reply      string
		wantStatus int
	}{
		{"the reader's spent code", 400, `{"error":"invalid_grant","error_description":"无效的授权码"}`, 400},
		{"moyu's secret", 401, `{"error":"invalid_client","error_description":"客户端密钥无效"}`, 500},
		{"oauth down", 500, `{"error":"server_error","error_description":"操作失败"}`, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := postCallback(t, &oauthCallbackFake{tokenStatus: tc.status, tokenReply: tc.reply})
			if status != tc.wantStatus {
				t.Fatalf("status = %d, want %d", status, tc.wantStatus)
			}
		})
	}
}

func TestCallbackAfterTheExchange(t *testing.T) {
	t.Run("banned", func(t *testing.T) {
		f := &oauthCallbackFake{tokenStatus: 200, tokenReply: tokenMinted,
			userinfoStatus: 403, userinfoReply: `{"error":"invalid_token","error_description":"The account is banned"}`}
		status, code := postCallback(t, f)
		if status != http.StatusForbidden || code != 10014 {
			t.Fatalf("got %d/%d, want 403/10014", status, code)
		}
	})
	t.Run("userinfo down revokes the minted token", func(t *testing.T) {
		f := &oauthCallbackFake{tokenStatus: 200, tokenReply: tokenMinted,
			userinfoStatus: 502, userinfoReply: `<html>bad gateway</html>`}
		status, _ := postCallback(t, f)
		if status != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", status)
		}
		deadline := time.Now().Add(2 * time.Second)
		for f.revoked.Load() == 0 && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		if f.revoked.Load() == 0 {
			t.Fatal("the token bought with a spent code was left live upstream")
		}
	})
}
