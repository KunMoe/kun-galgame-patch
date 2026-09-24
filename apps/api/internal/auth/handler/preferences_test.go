package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kun-galgame-patch-api/internal/auth/service"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/internal/testutil"
	"kun-galgame-patch-api/pkg/config"
)

const moyuClientID = "moyu-client-id"

type oauthPrefFake struct {
	paths   []string
	auth    []string
	ifMatch []string
	bodies  []string

	status int
	reply  string
}

func (f *oauthPrefFake) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		f.paths = append(f.paths, r.URL.Path)
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		f.ifMatch = append(f.ifMatch, r.Header.Get("If-Match"))
		f.bodies = append(f.bodies, string(raw))

		w.Header().Set("Content-Type", "application/json")
		status := f.status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(f.reply))
	}
}

func newPrefApp(t *testing.T, fake *oauthPrefFake, clientID string) (*testutil.TestApp, string) {
	t.Helper()
	srv := httptest.NewServer(fake.handler())
	t.Cleanup(srv.Close)

	svc := service.New(nil, nil, config.OAuthConfig{ServerURL: srv.URL, ClientID: clientID})
	h := New(svc, nil, nil, nil, nil)

	ta := testutil.NewTestApp(t)
	auth := middleware.Auth(ta.RDB, config.OAuthConfig{})
	ta.App.Put("/auth/me/nsfw", auth, h.UpdateNsfwDisplay)
	ta.App.Get("/auth/me/preferences", auth, h.GetPreferences)
	ta.App.Put("/auth/me/preferences", auth, h.UpdatePreferences)
	return ta, ta.CreateTestSession(t, 42)
}

func TestNsfwDisplayForwardsTheStanceAsTheUser(t *testing.T) {
	fake := &oauthPrefFake{reply: `{"code":0,"message":"成功","data":{"nsfw_display":"blur","adult_confirmed_at":"2026-09-22T08:30:00Z"}}`}
	ta, session := newPrefApp(t, fake, moyuClientID)

	resp := ta.Request(t, http.MethodPut, "/auth/me/nsfw", `{"nsfw_display":"blur"}`, session)
	r := testutil.ParseResponse(t, resp)
	if resp.StatusCode != http.StatusOK || r.Code != 0 {
		t.Fatalf("got %d/%d, want 200/0", resp.StatusCode, r.Code)
	}
	if len(fake.paths) != 1 || fake.paths[0] != "/auth/me/nsfw" {
		t.Fatalf("upstream path = %v", fake.paths)
	}
	if !strings.HasPrefix(fake.auth[0], "Bearer ") || len(fake.auth[0]) <= len("Bearer ") {
		t.Fatalf("the write must speak as the session user, got %q", fake.auth[0])
	}
	if fake.bodies[0] != `{"nsfw_display":"blur"}` {
		t.Fatalf("upstream body = %q", fake.bodies[0])
	}
	data, _ := r.Data.(map[string]any)
	if data["adult_confirmed_at"] == nil {
		t.Fatalf("the attestation timestamp must survive the passthrough: %v", r.Data)
	}
}

// 18001 is "this token predates the preferences scope", which no refresh can
// fix — 40399 is the one code the client already turns into "log out and back
// in once".
func TestNsfwDisplayScopeDenialLandsOnTheRelogInCode(t *testing.T) {
	fake := &oauthPrefFake{
		status: http.StatusForbidden,
		reply:  `{"code":18001,"message":"缺少 preferences 权限"}`,
	}
	ta, session := newPrefApp(t, fake, moyuClientID)

	resp := ta.Request(t, http.MethodPut, "/auth/me/nsfw", `{"nsfw_display":"show"}`, session)
	r := testutil.ParseResponse(t, resp)
	if resp.StatusCode != http.StatusForbidden || r.Code != 40399 {
		t.Fatalf("got %d/%d, want 403/40399", resp.StatusCode, r.Code)
	}
}

// 18001 is the only upstream code this lane rewrites. Every other refusal keeps
// its own code and message, or the page reports a generic failure for something
// the reader could have fixed.
func TestNsfwDisplayOtherRefusalsPassThrough(t *testing.T) {
	fake := &oauthPrefFake{
		status: http.StatusBadRequest,
		reply:  `{"code":18007,"message":"成人内容显示方式必须是 hide / blur / show"}`,
	}
	ta, session := newPrefApp(t, fake, moyuClientID)

	resp := ta.Request(t, http.MethodPut, "/auth/me/nsfw", `{"nsfw_display":"maybe"}`, session)
	r := testutil.ParseResponse(t, resp)
	if resp.StatusCode != http.StatusBadRequest || r.Code != 18007 {
		t.Fatalf("got %d/%d, want 400/18007", resp.StatusCode, r.Code)
	}
}

func TestPreferencesReadNamesMoyusOwnClientID(t *testing.T) {
	fake := &oauthPrefFake{reply: `{"code":0,"message":"成功","data":{"namespace":"` + moyuClientID + `","doc":{},"version":0,"updated_at":null}}`}
	ta, session := newPrefApp(t, fake, moyuClientID)

	resp := ta.Request(t, http.MethodGet, "/auth/me/preferences", "", session)
	r := testutil.ParseResponse(t, resp)
	if resp.StatusCode != http.StatusOK || r.Code != 0 {
		t.Fatalf("got %d/%d", resp.StatusCode, r.Code)
	}
	if fake.paths[0] != "/auth/me/preferences/"+moyuClientID {
		t.Fatalf("upstream path = %q, want the namespace to be this client's id", fake.paths[0])
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q; a cached preference answers another device's stale state", got)
	}
}

// A settings page writes on every keystroke's debounce. Turning a missing scope
// into 40399 there would fire the "log out and back in" toast over and over,
// so the KV lane degrades quietly to the cookie instead.
func TestPreferencesScopeDenialDegradesInsteadOfNagging(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		t.Run(method, func(t *testing.T) {
			fake := &oauthPrefFake{
				status: http.StatusForbidden,
				reply:  `{"code":18001,"message":"缺少 preferences 权限"}`,
			}
			ta, session := newPrefApp(t, fake, moyuClientID)

			resp := ta.Request(t, method, "/auth/me/preferences", `{"doc":{}}`, session)
			r := testutil.ParseResponse(t, resp)
			if r.Code != 40398 {
				t.Fatalf("%s got code %d, want 40398", method, r.Code)
			}
		})
	}
}

func TestPreferencesWriteSendsTheVersionAsIfMatch(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"claims the first write", `{"doc":{"title_language":"zh-cn"},"version":0}`, `"0"`},
		{"holds a read version", `{"doc":{"title_language":"zh-cn"},"version":7}`, `"7"`},
		{"last write wins", `{"doc":{"title_language":"zh-cn"}}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &oauthPrefFake{reply: `{"code":0,"message":"成功","data":{"doc":{},"version":1}}`}
			ta, session := newPrefApp(t, fake, moyuClientID)

			resp := ta.Request(t, http.MethodPut, "/auth/me/preferences", tc.body, session)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d", resp.StatusCode)
			}
			if fake.ifMatch[0] != tc.want {
				t.Fatalf("If-Match = %q, want %q", fake.ifMatch[0], tc.want)
			}

			var sent map[string]any
			if err := json.Unmarshal([]byte(fake.bodies[0]), &sent); err != nil {
				t.Fatalf("upstream body is not JSON: %v (%s)", err, fake.bodies[0])
			}
			if _, ok := sent["version"]; ok {
				t.Fatalf("version rides If-Match, never the document: %s", fake.bodies[0])
			}
			doc, ok := sent["doc"].(map[string]any)
			if !ok || doc["title_language"] != "zh-cn" {
				t.Fatalf("the document did not survive the rewrite: %s", fake.bodies[0])
			}
		})
	}
}

func TestPreferencesVersionClashPassesThrough(t *testing.T) {
	fake := &oauthPrefFake{
		status: http.StatusPreconditionFailed,
		reply:  `{"code":18006,"message":"版本不匹配"}`,
	}
	ta, session := newPrefApp(t, fake, moyuClientID)

	resp := ta.Request(t, http.MethodPut, "/auth/me/preferences", `{"doc":{},"version":3}`, session)
	r := testutil.ParseResponse(t, resp)
	if resp.StatusCode != http.StatusPreconditionFailed || r.Code != 18006 {
		t.Fatalf("got %d/%d, want 412/18006", resp.StatusCode, r.Code)
	}
}

// Everything here is moyu's to fix, so none of it may reach the page as
// OAuth's own code: 18002/18003 refuse the namespace moyu names (infra
// preferenceGate), and a 401 refuses a token moyu has just judged live.
func TestPreferencesMoyusOwnFaultsAreA500(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		reply  string
	}{
		{"namespace denied", http.StatusForbidden, `{"code":18003,"message":"无权访问该命名空间"}`},
		{"namespace invalid", http.StatusBadRequest, `{"code":18002,"message":"命名空间格式不合法"}`},
		{"token refused", http.StatusUnauthorized, `{"code":10003,"message":"令牌已过期，请重新登录"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &oauthPrefFake{status: tc.status, reply: tc.reply}
			ta, session := newPrefApp(t, fake, moyuClientID)

			resp := ta.Request(t, http.MethodGet, "/auth/me/preferences", "", session)
			r := testutil.ParseResponse(t, resp)
			if resp.StatusCode != http.StatusInternalServerError || r.Code != 50000 {
				t.Fatalf("got %d/%d, want 500/50000", resp.StatusCode, r.Code)
			}
		})
	}
}

func TestNsfwDisplayOutageIsA503(t *testing.T) {
	fake := &oauthPrefFake{status: http.StatusInternalServerError, reply: `{"code":10,"message":"操作失败"}`}
	ta, session := newPrefApp(t, fake, moyuClientID)

	resp := ta.Request(t, http.MethodPut, "/auth/me/nsfw", `{"nsfw_display":"show"}`, session)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}

func TestPreferencesWithoutAConfiguredClientNeverCallsUpstream(t *testing.T) {
	fake := &oauthPrefFake{reply: `{"code":0}`}
	ta, session := newPrefApp(t, fake, "")

	resp := ta.Request(t, http.MethodGet, "/auth/me/preferences", "", session)
	if r := testutil.ParseResponse(t, resp); r.Code != 40398 {
		t.Fatalf("code = %d, want 40398", r.Code)
	}
	if len(fake.paths) != 0 {
		t.Fatalf("an unnamed namespace must not be sent upstream: %v", fake.paths)
	}
}

func TestPreferencesNeedASession(t *testing.T) {
	fake := &oauthPrefFake{reply: `{"code":0}`}
	ta, _ := newPrefApp(t, fake, moyuClientID)

	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPut, "/auth/me/nsfw", `{"nsfw_display":"show"}`},
		{http.MethodGet, "/auth/me/preferences", ""},
		{http.MethodPut, "/auth/me/preferences", `{"doc":{}}`},
	} {
		resp := ta.Request(t, tc.method, tc.path, tc.body, "")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s: status %d, want 401", tc.method, tc.path, resp.StatusCode)
		}
	}
	if len(fake.paths) != 0 {
		t.Fatalf("an anonymous request must never reach OAuth: %v", fake.paths)
	}
}
