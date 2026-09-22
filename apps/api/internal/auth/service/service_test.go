package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"kun-galgame-patch-api/pkg/config"
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

	info, err := svc.GetUserInfo("token")
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

	info, err := svc.GetUserInfo("token")
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

	info, err := svc.GetUserInfo("token")
	if err != nil {
		t.Fatal(err)
	}
	if info.AdultConfirmed || info.NsfwDisplay != "" {
		t.Fatalf("absent claims = %v/%q, want false/\"\"", info.AdultConfirmed, info.NsfwDisplay)
	}
}

func TestPreferencesNamespaceIsTheOAuthClientID(t *testing.T) {
	svc := New(nil, nil, config.OAuthConfig{ClientID: "moyu-client-id"})
	if got := svc.PreferencesNamespace(); got != "moyu-client-id" {
		t.Fatalf("namespace = %q", got)
	}
}
