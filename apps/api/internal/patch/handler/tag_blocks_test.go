package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/internal/testutil"
	"kun-galgame-patch-api/pkg/catalogv2/catalogv2test"
	"kun-galgame-patch-api/pkg/config"
)

type tagBlockUpstream struct {
	mu     sync.Mutex
	calls  []string
	bodies []string
	auth   string
	refuse string
}

func (f *tagBlockUpstream) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.calls = append(f.calls, r.Method+" "+r.URL.Path)
		f.bodies = append(f.bodies, string(body))
		f.auth = r.Header.Get("Authorization")
		refuse := f.refuse
		f.mu.Unlock()
		if refuse != "" {
			catalogv2test.Problem(w, r, refuse, "refused", nil)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodPut:
			_, _ = w.Write([]byte(`{"object":"tag_block","tag_id":"42","mode":"exact","created_at":"2026-10-11T01:00:00Z","updated_at":"2026-10-11T01:00:00Z"}`))
		default:
			_, _ = w.Write([]byte(`{"object":"list","items":[{"object":"tag_block","tag_id":"6","mode":"tree","created_at":"2026-10-11T01:00:00Z","updated_at":"2026-10-11T01:00:00Z"}],"next_cursor":null}`))
		}
	}
}

func tagBlockApp(t *testing.T, fake *tagBlockUpstream) (*testutil.TestApp, string) {
	t.Helper()
	srv := httptest.NewServer(fake.handler())
	t.Cleanup(srv.Close)
	h := New(nil, galgameClient.NewWithKey(srv.URL, "nm_test_key"), nil, nil, nil)
	ta := testutil.NewTestApp(t)
	auth := middleware.Auth(ta.RDB, config.OAuthConfig{})
	ta.App.Get("/tag-block", auth, h.MyTagBlocks)
	ta.App.Put("/tag-block/:tagId", auth, h.PutTagBlock)
	ta.App.Delete("/tag-block/:tagId", auth, h.DeleteTagBlock)
	return ta, ta.CreateTestSession(t, 42)
}

func TestTagBlockProxyCarriesTheReadersOwnToken(t *testing.T) {
	fake := &tagBlockUpstream{}
	ta, session := tagBlockApp(t, fake)

	resp := ta.Request(t, http.MethodGet, "/tag-block", "", session)
	body := testutil.ReadBody(t, resp)
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, `"blocks":[{"tag_id":6,"mode":"tree"`) {
		t.Fatalf("status = %d body = %s", resp.StatusCode, body)
	}
	if !strings.HasPrefix(fake.auth, "Bearer ey") {
		t.Errorf("Authorization = %q, want the session's access token, not the app key", fake.auth)
	}

	resp = ta.Request(t, http.MethodPut, "/tag-block/42", "", session)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT without a body: status = %d body = %s", resp.StatusCode, testutil.ReadBody(t, resp))
	}
	ta.Request(t, http.MethodPut, "/tag-block/42", `{"mode":"exact"}`, session)
	ta.Request(t, http.MethodDelete, "/tag-block/42", "", session)

	fake.mu.Lock()
	defer fake.mu.Unlock()
	want := []string{"GET /v2/me/blocked-tags", "PUT /v2/me/blocked-tags/42", "PUT /v2/me/blocked-tags/42", "DELETE /v2/me/blocked-tags/42"}
	if strings.Join(fake.calls, "|") != strings.Join(want, "|") {
		t.Fatalf("calls = %v, want %v", fake.calls, want)
	}
	if fake.bodies[1] != `{"mode":"tree"}` || fake.bodies[2] != `{"mode":"exact"}` {
		t.Errorf("PUT bodies = %q, %q; a bare PUT blocks the whole tree", fake.bodies[1], fake.bodies[2])
	}
}

func TestTagBlockProxyRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, refuse, path, body string
		status                   int
		message                  string
	}{
		{"an unknown mode never leaves", "", "/tag-block/42", `{"mode":"subtree"}`, http.StatusBadRequest, "未知的屏蔽方式"},
		{"a bad tag id never leaves", "", "/tag-block/abc", "", http.StatusBadRequest, "无效的标签 ID"},
		{"catalog has no such tag", "NOT_FOUND", "/tag-block/42", "", http.StatusNotFound, "标签不存在"},
		{"the account is at the cap", "VALIDATION_FAILED", "/tag-block/42", "", http.StatusUnprocessableEntity, "最多屏蔽 100 个标签"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &tagBlockUpstream{refuse: tc.refuse}
			ta, session := tagBlockApp(t, fake)
			resp := ta.Request(t, http.MethodPut, tc.path, tc.body, session)
			body := testutil.ReadBody(t, resp)
			if resp.StatusCode != tc.status || !strings.Contains(body, tc.message) {
				t.Fatalf("status = %d body = %s, want %d %s", resp.StatusCode, body, tc.status, tc.message)
			}
			fake.mu.Lock()
			defer fake.mu.Unlock()
			if tc.refuse == "" && len(fake.calls) != 0 {
				t.Errorf("a request refused locally still reached catalog: %v", fake.calls)
			}
		})
	}

	ta, _ := tagBlockApp(t, &tagBlockUpstream{})
	if resp := ta.Request(t, http.MethodGet, "/tag-block", "", ""); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("signed out: status = %d, want 401", resp.StatusCode)
	}
}
