package client

// Face-selection tests: prove the galgame client routes each call to the right
// face by ROUTE membership, not HTTP method. Since open-API phase 2 wave 07
// (route-B endgame) the A-bucket READ set — search / batch / detail / calendar /
// vndb lookup + the taxonomy reads (tag/official/engine/series list/search/
// detail) + the galgame links/aliases edit-prefill reads — hits the {base}/v1
// public face + X-API-Key. The B-bucket platform-workflow reads (/galgame/mine,
// /galgame/messages/mine, taxonomy /:id/revisions), the S2S message feed, and
// the user write set (submit / draft update+delete / claim / image upload /
// links+aliases relation edits, wave 06a) stay on {base}/internal + key; only
// the staff taxonomy CRUD/revert + /admin/* stay on legacy {base}/api. The
// internal + v1 faces hard-depend on the internal-tier key — the empty-key
// rollback to legacy was retired in wave 05. Deterministic — a fake service
// records the last request.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// faceRecorder captures the last request the fake service received.
type faceRecorder struct {
	mu     sync.Mutex
	path   string
	apiKey string
	auth   string
}

func (r *faceRecorder) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		r.path = req.URL.Path
		r.apiKey = req.Header.Get("X-API-Key")
		r.auth = req.Header.Get("Authorization")
		r.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		// A valid empty envelope: unmarshals into every response type used below
		// (detail struct, paginated {items,total}, search-pending) as zero values.
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{}}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestFaceSelection_WithKey proves that, with an internal-tier key configured,
// reads (and the S2S message feed) hit {base}/internal + X-API-Key (personalized
// reads additionally carry the user JWT on Authorization — dual credential),
// the user write set (submit / update / draft patch+delete / claim / image
// upload / links+aliases relation edits) also hits {base}/internal with dual
// credentials (X-API-Key + Bearer), and only the staff taxonomy CRUD/revert +
// /admin/* proxies stay on {base}/api with no key.
func TestFaceSelection_WithKey(t *testing.T) {
	rec := &faceRecorder{}
	srv := rec.server(t)
	c := NewWithKey(srv.URL, "nm_test_key")
	ctx := context.Background()

	t.Run("anonymous detail read → v1 + key", func(t *testing.T) {
		if _, err := c.GetGalgame(ctx, 123, ""); err != nil {
			t.Fatalf("GetGalgame: %v", err)
		}
		if rec.path != "/v1/galgame/123" {
			t.Errorf("path = %q, want /v1/galgame/123", rec.path)
		}
		if rec.apiKey != "nm_test_key" {
			t.Errorf("X-API-Key = %q, want nm_test_key", rec.apiKey)
		}
		if rec.auth != "" {
			t.Errorf("Authorization = %q, want empty on anonymous read", rec.auth)
		}
	})

	t.Run("token read /galgame/mine → internal + key + user JWT (dual credential)", func(t *testing.T) {
		if _, err := c.ListMyGalgames(ctx, "user-jwt", "", 0, 0); err != nil {
			t.Fatalf("ListMyGalgames: %v", err)
		}
		if rec.path != "/internal/galgame/mine" {
			t.Errorf("path = %q, want /internal/galgame/mine", rec.path)
		}
		if rec.apiKey != "nm_test_key" {
			t.Errorf("X-API-Key = %q, want nm_test_key", rec.apiKey)
		}
		if rec.auth != "Bearer user-jwt" {
			t.Errorf("Authorization = %q, want Bearer user-jwt", rec.auth)
		}
	})

	t.Run("token read /galgame/messages/mine → internal + key + user JWT", func(t *testing.T) {
		if _, err := c.GetMyGalgameMessages(ctx, "user-jwt", 0, 0); err != nil {
			t.Fatalf("GetMyGalgameMessages: %v", err)
		}
		if rec.path != "/internal/galgame/messages/mine" {
			t.Errorf("path = %q, want /internal/galgame/messages/mine", rec.path)
		}
		if rec.apiKey != "nm_test_key" {
			t.Errorf("X-API-Key = %q, want nm_test_key", rec.apiKey)
		}
		if rec.auth != "Bearer user-jwt" {
			t.Errorf("Authorization = %q, want Bearer user-jwt", rec.auth)
		}
	})

	t.Run("publish search (include_pending) → v1 + key + user JWT", func(t *testing.T) {
		if _, err := c.SearchGalgameForPublish(ctx, "user-jwt", "q", 0); err != nil {
			t.Fatalf("SearchGalgameForPublish: %v", err)
		}
		if rec.path != "/v1/galgame/search" {
			t.Errorf("path = %q, want /v1/galgame/search", rec.path)
		}
		if rec.apiKey != "nm_test_key" {
			t.Errorf("X-API-Key = %q, want nm_test_key", rec.apiKey)
		}
		if rec.auth != "Bearer user-jwt" {
			t.Errorf("Authorization = %q, want Bearer user-jwt", rec.auth)
		}
	})

	t.Run("proxy GET (taxonomy A-bucket read) → v1 + key", func(t *testing.T) {
		if _, err := c.Proxy(ctx, http.MethodGet, "/tag/search?q=x", "", nil, ""); err != nil {
			t.Fatalf("Proxy GET: %v", err)
		}
		if rec.path != "/v1/galgame/tags/search" {
			t.Errorf("path = %q, want /v1/galgame/tags/search", rec.path)
		}
		if rec.apiKey != "nm_test_key" {
			t.Errorf("X-API-Key = %q, want nm_test_key", rec.apiKey)
		}
	})

	t.Run("proxy GET (taxonomy B-bucket revisions) → internal + key", func(t *testing.T) {
		if _, err := c.Proxy(ctx, http.MethodGet, "/tag/5/revisions?page=1", "", nil, ""); err != nil {
			t.Fatalf("Proxy GET revisions: %v", err)
		}
		if rec.path != "/internal/tag/5/revisions" {
			t.Errorf("path = %q, want /internal/tag/5/revisions (B-bucket stays internal)", rec.path)
		}
		if rec.apiKey != "nm_test_key" {
			t.Errorf("X-API-Key = %q, want nm_test_key", rec.apiKey)
		}
	})

	t.Run("proxy POST (taxonomy tag write) → legacy, no key", func(t *testing.T) {
		if _, err := c.Proxy(ctx, http.MethodPost, "/tag", "user-jwt", []byte(`{}`), "application/json"); err != nil {
			t.Fatalf("Proxy POST: %v", err)
		}
		if rec.path != "/api/tag" {
			t.Errorf("path = %q, want /api/tag", rec.path)
		}
		if rec.apiKey != "" {
			t.Errorf("X-API-Key = %q, want empty on legacy write face", rec.apiKey)
		}
	})

	t.Run("proxy POST (series write, staff) → legacy, no key", func(t *testing.T) {
		if _, err := c.Proxy(ctx, http.MethodPost, "/series", "user-jwt", []byte(`{}`), "application/json"); err != nil {
			t.Fatalf("Proxy POST /series: %v", err)
		}
		if rec.path != "/api/series" {
			t.Errorf("path = %q, want /api/series (staff taxonomy stays legacy)", rec.path)
		}
		if rec.apiKey != "" {
			t.Errorf("X-API-Key = %q, want empty on legacy write face", rec.apiKey)
		}
	})

	t.Run("proxy PUT (series update, staff) → legacy, no key", func(t *testing.T) {
		if _, err := c.Proxy(ctx, http.MethodPut, "/series/9", "user-jwt", []byte(`{}`), "application/json"); err != nil {
			t.Fatalf("Proxy PUT /series/9: %v", err)
		}
		if rec.path != "/api/series/9" {
			t.Errorf("path = %q, want /api/series/9 (staff taxonomy stays legacy)", rec.path)
		}
		if rec.apiKey != "" {
			t.Errorf("X-API-Key = %q, want empty on legacy write face", rec.apiKey)
		}
	})

	t.Run("proxy POST /galgame/:gid/links (relation write) → internal + key + JWT", func(t *testing.T) {
		if _, err := c.Proxy(ctx, http.MethodPost, "/galgame/42/links", "user-jwt", []byte(`{}`), "application/json"); err != nil {
			t.Fatalf("Proxy POST links: %v", err)
		}
		if rec.path != "/internal/galgame/42/links" {
			t.Errorf("path = %q, want /internal/galgame/42/links", rec.path)
		}
		if rec.apiKey != "nm_test_key" {
			t.Errorf("X-API-Key = %q, want nm_test_key", rec.apiKey)
		}
		if rec.auth != "Bearer user-jwt" {
			t.Errorf("Authorization = %q, want Bearer user-jwt", rec.auth)
		}
	})

	t.Run("proxy DELETE /galgame/:gid/aliases (relation write) → internal + key + JWT", func(t *testing.T) {
		if _, err := c.Proxy(ctx, http.MethodDelete, "/galgame/42/aliases", "user-jwt", []byte(`{}`), "application/json"); err != nil {
			t.Fatalf("Proxy DELETE aliases: %v", err)
		}
		if rec.path != "/internal/galgame/42/aliases" {
			t.Errorf("path = %q, want /internal/galgame/42/aliases", rec.path)
		}
		if rec.apiKey != "nm_test_key" {
			t.Errorf("X-API-Key = %q, want nm_test_key", rec.apiKey)
		}
		if rec.auth != "Bearer user-jwt" {
			t.Errorf("Authorization = %q, want Bearer user-jwt", rec.auth)
		}
	})

	t.Run("submit write → internal + key + JWT", func(t *testing.T) {
		if _, err := c.SubmitGalgame(ctx, "user-jwt", map[string]any{"x": 1}); err != nil {
			t.Fatalf("SubmitGalgame: %v", err)
		}
		if rec.path != "/internal/galgame/submit" {
			t.Errorf("path = %q, want /internal/galgame/submit", rec.path)
		}
		if rec.apiKey != "nm_test_key" {
			t.Errorf("X-API-Key = %q, want nm_test_key", rec.apiKey)
		}
		if rec.auth != "Bearer user-jwt" {
			t.Errorf("Authorization = %q, want Bearer user-jwt", rec.auth)
		}
	})

	t.Run("claim write → internal + key + JWT", func(t *testing.T) {
		if _, err := c.ClaimGalgame(ctx, "user-jwt", 7); err != nil {
			t.Fatalf("ClaimGalgame: %v", err)
		}
		if rec.path != "/internal/galgame/7/claim" {
			t.Errorf("path = %q, want /internal/galgame/7/claim", rec.path)
		}
		if rec.apiKey != "nm_test_key" {
			t.Errorf("X-API-Key = %q, want nm_test_key", rec.apiKey)
		}
		if rec.auth != "Bearer user-jwt" {
			t.Errorf("Authorization = %q, want Bearer user-jwt", rec.auth)
		}
	})

	t.Run("patch draft write → internal + key + JWT", func(t *testing.T) {
		if _, err := c.PatchGalgameDraft(ctx, "user-jwt", 8, map[string]any{"x": 1}); err != nil {
			t.Fatalf("PatchGalgameDraft: %v", err)
		}
		if rec.path != "/internal/galgame/8" {
			t.Errorf("path = %q, want /internal/galgame/8", rec.path)
		}
		if rec.apiKey != "nm_test_key" {
			t.Errorf("X-API-Key = %q, want nm_test_key", rec.apiKey)
		}
		if rec.auth != "Bearer user-jwt" {
			t.Errorf("Authorization = %q, want Bearer user-jwt", rec.auth)
		}
	})

	t.Run("delete draft write → internal + key + JWT", func(t *testing.T) {
		if err := c.DeleteGalgameDraft(ctx, "user-jwt", 9); err != nil {
			t.Fatalf("DeleteGalgameDraft: %v", err)
		}
		if rec.path != "/internal/galgame/9" {
			t.Errorf("path = %q, want /internal/galgame/9", rec.path)
		}
		if rec.apiKey != "nm_test_key" {
			t.Errorf("X-API-Key = %q, want nm_test_key", rec.apiKey)
		}
		if rec.auth != "Bearer user-jwt" {
			t.Errorf("Authorization = %q, want Bearer user-jwt", rec.auth)
		}
	})

	t.Run("update write → internal + key + JWT", func(t *testing.T) {
		if _, err := c.UpdateGalgame(ctx, "user-jwt", 5, map[string]any{"x": 1}); err != nil {
			t.Fatalf("UpdateGalgame: %v", err)
		}
		if rec.path != "/internal/galgame/5" {
			t.Errorf("path = %q, want /internal/galgame/5", rec.path)
		}
		if rec.apiKey != "nm_test_key" {
			t.Errorf("X-API-Key = %q, want nm_test_key", rec.apiKey)
		}
		if rec.auth != "Bearer user-jwt" {
			t.Errorf("Authorization = %q, want Bearer user-jwt", rec.auth)
		}
	})

	t.Run("image upload proxy → internal + key + JWT", func(t *testing.T) {
		if _, err := c.UploadGalgameImage(ctx, "user-jwt", "galgame_cover", "f.png", []byte("x"), "image/png"); err != nil {
			t.Fatalf("UploadGalgameImage: %v", err)
		}
		if rec.path != "/internal/galgame/image" {
			t.Errorf("path = %q, want /internal/galgame/image", rec.path)
		}
		if rec.apiKey != "nm_test_key" {
			t.Errorf("X-API-Key = %q, want nm_test_key", rec.apiKey)
		}
		if rec.auth != "Bearer user-jwt" {
			t.Errorf("Authorization = %q, want Bearer user-jwt", rec.auth)
		}
	})

	t.Run("messages feed → internal + key (S2S cron)", func(t *testing.T) {
		if _, err := c.GetGalgameMessageFeed(ctx, 0, 10); err != nil {
			t.Fatalf("GetGalgameMessageFeed: %v", err)
		}
		if rec.path != "/internal/galgame/messages/feed" {
			t.Errorf("path = %q, want /internal/galgame/messages/feed", rec.path)
		}
		if rec.apiKey != "nm_test_key" {
			t.Errorf("X-API-Key = %q, want nm_test_key on internal feed face", rec.apiKey)
		}
	})
}

// TestV1ReadRouting proves every A-bucket read routes to the {base}/v1 public
// face with the internal-tier key (route-B endgame, wave 07). The composed
// taxonomy detail reads make two /v1 calls; the recorder captures the last
// (the reverse-lookup), which is sufficient to prove the face.
func TestV1ReadRouting(t *testing.T) {
	rec := &faceRecorder{}
	srv := rec.server(t)
	c := NewWithKey(srv.URL, "nm_test_key")
	ctx := context.Background()

	check := func(t *testing.T, wantPath string, call func() error) {
		t.Helper()
		if err := call(); err != nil {
			t.Fatalf("call: %v", err)
		}
		if rec.path != wantPath {
			t.Errorf("path = %q, want %q", rec.path, wantPath)
		}
		if rec.apiKey != "nm_test_key" {
			t.Errorf("X-API-Key = %q, want nm_test_key", rec.apiKey)
		}
	}

	t.Run("batch → v1", func(t *testing.T) {
		check(t, "/v1/galgame/batch", func() error { _, e := c.GalgameBatch(ctx, []int{1}, ""); return e })
	})
	t.Run("search → v1", func(t *testing.T) {
		check(t, "/v1/galgame/search", func() error { _, e := c.SearchGalgame(ctx, SearchGalgameParams{Q: "x"}); return e })
	})
	t.Run("calendar → v1", func(t *testing.T) {
		check(t, "/v1/galgame/calendar", func() error { _, e := c.GetGalgameCalendar(ctx, "", ""); return e })
	})
	t.Run("calendar/pending → v1", func(t *testing.T) {
		check(t, "/v1/galgame/calendar/pending", func() error { _, e := c.GetGalgameCalendarPending(ctx, "", ""); return e })
	})
	t.Run("calendar/tba → v1", func(t *testing.T) {
		check(t, "/v1/galgame/calendar/tba", func() error { _, e := c.GetGalgameCalendarTBA(ctx, ""); return e })
	})
	t.Run("vndb lookup → v1", func(t *testing.T) {
		check(t, "/v1/galgame/lookup", func() error { _, _, e := c.CheckGalgameByVndbID(ctx, "v1"); return e })
	})

	// Taxonomy A-bucket reads through the generic Proxy.
	proxyGet := func(p string) func() error {
		return func() error { _, e := c.Proxy(ctx, http.MethodGet, p, "", nil, ""); return e }
	}
	t.Run("tag list → v1", func(t *testing.T) { check(t, "/v1/galgame/tags", proxyGet("/tag?page=1")) })
	t.Run("tag multi → v1", func(t *testing.T) { check(t, "/v1/galgame/tags/multi", proxyGet("/tag/multi?tag_ids=1,2")) })
	t.Run("official search → v1", func(t *testing.T) { check(t, "/v1/galgame/officials/search", proxyGet("/official/search?q=x")) })
	t.Run("engine list → v1", func(t *testing.T) { check(t, "/v1/galgame/engines", proxyGet("/engine")) })
	t.Run("series list → v1", func(t *testing.T) { check(t, "/v1/galgame/series", proxyGet("/series?page=1")) })
	t.Run("tag detail (composed) → v1 reverse-lookup", func(t *testing.T) {
		check(t, "/v1/galgame/tags/5/galgames", proxyGet("/tag/_?tag_id=5&page=1&limit=24"))
	})
	t.Run("official detail (composed) → v1 reverse-lookup", func(t *testing.T) {
		check(t, "/v1/galgame/officials/9/galgames", proxyGet("/official/_?official_id=9"))
	})
	t.Run("galgame links → v1 detail", func(t *testing.T) { check(t, "/v1/galgame/42", proxyGet("/galgame/42/links")) })
	t.Run("galgame aliases → v1 detail", func(t *testing.T) { check(t, "/v1/galgame/42", proxyGet("/galgame/42/aliases")) })
}

// TestMessageFeedRequiresKey proves the S2S message feed hard-depends on the
// internal-tier key: with no key configured it errors before dialing rather
// than silently falling back (the rollback valve was retired in wave 05).
func TestMessageFeedRequiresKey(t *testing.T) {
	rec := &faceRecorder{}
	srv := rec.server(t)
	c := NewWithKey(srv.URL, "") // no API key
	ctx := context.Background()

	if _, err := c.GetGalgameMessageFeed(ctx, 0, 10); err == nil {
		t.Fatal("GetGalgameMessageFeed with empty key: want error, got nil")
	}
	if rec.path != "" {
		t.Errorf("recorder path = %q, want empty (must not dial without a key)", rec.path)
	}
}
