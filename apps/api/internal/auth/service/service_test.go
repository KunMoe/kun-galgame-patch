package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/upstream"
)

func newUserInfoService(t *testing.T, body string) *AuthService {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/userinfo" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return New(nil, nil, config.OAuthConfig{ServerURL: srv.URL})
}

func TestUserInfoCarriesTheContentStanceClaims(t *testing.T) {
	svc := newUserInfoService(t,
		`{"id":42,"sub":"s","name":"kun","adult_confirmed":true,"nsfw_display":"show"}`)

	info, err := svc.GetUserInfo(context.Background(), "token")
	if err != nil {
		t.Fatal(err)
	}
	if !info.AdultConfirmed || info.NsfwDisplay != "show" {
		t.Fatalf("stance claims = %v/%q, want true/show", info.AdultConfirmed, info.NsfwDisplay)
	}
}

// The migration backfilled nsfw_display='blur' onto accounts that never
// attested, so this is what the overwhelming majority of rows look like. Read
// alone the column says "blur"; the effective stance is hide, and the claim the
// fold needs is the one that is easiest to drop on the way to the client.
func TestUserInfoKeepsTheUnattestedAccountsBlurAndItsFalseFlagApart(t *testing.T) {
	svc := newUserInfoService(t,
		`{"id":42,"sub":"s","name":"kun","adult_confirmed":false,"nsfw_display":"blur"}`)

	info, err := svc.GetUserInfo(context.Background(), "token")
	if err != nil {
		t.Fatal(err)
	}
	if info.AdultConfirmed {
		t.Fatal("adult_confirmed must stay false")
	}
	if info.NsfwDisplay != "blur" {
		t.Fatalf("nsfw_display = %q; the stored value is reported verbatim, the fold happens on the client", info.NsfwDisplay)
	}
}

// Without the `profile` scope both keys are absent (same rule as name/picture),
// which must read as "unknown" rather than as a stance.
func TestUserInfoWithoutTheProfileScopeReportsNoStance(t *testing.T) {
	svc := newUserInfoService(t, `{"id":42,"sub":"s"}`)

	info, err := svc.GetUserInfo(context.Background(), "token")
	if err != nil {
		t.Fatal(err)
	}
	if info.AdultConfirmed || info.NsfwDisplay != "" {
		t.Fatalf("absent claims = %v/%q, want false/\"\"", info.AdultConfirmed, info.NsfwDisplay)
	}
}

func newFakeOAuth(t *testing.T, status int, body string) *AuthService {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return New(nil, nil, config.OAuthConfig{ServerURL: srv.URL, ClientID: "moyu", ClientSecret: "s"})
}

// The bodies are what infra's OAuthHandler.Token answers (protoErr: 401 for
// invalid_client, 400 for the rest; protoServerError: 500).
func TestExchangeCodeKnowsWhoseFaultItIs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   upstream.Kind
	}{
		{"spent code", 400, `{"error":"invalid_grant","error_description":"无效的授权码"}`, upstream.Rejected},
		{"rotated secret", 401, `{"error":"invalid_client","error_description":"客户端密钥无效"}`, upstream.Internal},
		{"grant not registered", 400, `{"error":"unauthorized_client","error_description":"客户端未被授权使用该授权类型"}`, upstream.Internal},
		{"oauth down", 500, `{"error":"server_error","error_description":"操作失败"}`, upstream.Unavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newFakeOAuth(t, tc.status, tc.body).ExchangeCode(context.Background(), "code", "verifier")
			if got := upstream.KindOf(err); got != tc.want {
				t.Fatalf("kind = %d, want %d (err %v)", got, tc.want, err)
			}
		})
	}
}

// Userinfo sits behind infra's middleware.BearerAuth: 403 is the ban, 401 a
// token it would not take.
func TestUserInfoFailures(t *testing.T) {
	_, err := newFakeOAuth(t, 403, `{"error":"invalid_token","error_description":"The account is banned"}`).
		GetUserInfo(context.Background(), "token")
	if err != ErrUserBanned {
		t.Fatalf("403 = %v, want ErrUserBanned", err)
	}
	_, err = newFakeOAuth(t, 401, `{"error":"invalid_token","error_description":"The access token is expired, revoked or malformed"}`).
		GetUserInfo(context.Background(), "token")
	if upstream.KindOf(err) != upstream.Internal {
		t.Fatalf("401 = %v, want Internal", err)
	}
	_, err = newFakeOAuth(t, 502, `<html>bad gateway</html>`).GetUserInfo(context.Background(), "token")
	if upstream.KindOf(err) != upstream.Unavailable {
		t.Fatalf("502 = %v, want Unavailable", err)
	}
}

func TestPreferencesNamespaceIsTheOAuthClientID(t *testing.T) {
	svc := New(nil, nil, config.OAuthConfig{ClientID: "moyu-client-id"})
	if got := svc.PreferencesNamespace(); got != "moyu-client-id" {
		t.Fatalf("namespace = %q", got)
	}
}
