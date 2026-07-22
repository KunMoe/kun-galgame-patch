package client

// Face-selection tests: prove the galgame client routes each call to the right
// face (internal rich read face + X-API-Key vs legacy /api) by ROUTE
// membership, not HTTP method. The read face hard-depends on the internal-tier
// key — the empty-key rollback to legacy was retired in open-API phase 2 wave
// 05. Deterministic — a fake service records the last request.

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
// while writes / the image upload proxy / non-GET taxonomy proxies stay on
// {base}/api with no key.
func TestFaceSelection_WithKey(t *testing.T) {
	rec := &faceRecorder{}
	srv := rec.server(t)
	c := NewWithKey(srv.URL, "nm_test_key")
	ctx := context.Background()

	t.Run("anonymous read (get helper) → internal + key", func(t *testing.T) {
		if _, err := c.GetGalgame(ctx, 123, ""); err != nil {
			t.Fatalf("GetGalgame: %v", err)
		}
		if rec.path != "/internal/galgame/123" {
			t.Errorf("path = %q, want /internal/galgame/123", rec.path)
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
		if _, err := c.GetMyWikiMessages(ctx, "user-jwt", 0, 0); err != nil {
			t.Fatalf("GetMyWikiMessages: %v", err)
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

	t.Run("publish search (include_pending) → internal + key + user JWT", func(t *testing.T) {
		if _, err := c.SearchGalgameForPublish(ctx, "user-jwt", "q", 0); err != nil {
			t.Fatalf("SearchGalgameForPublish: %v", err)
		}
		if rec.path != "/internal/galgame/search" {
			t.Errorf("path = %q, want /internal/galgame/search", rec.path)
		}
		if rec.apiKey != "nm_test_key" {
			t.Errorf("X-API-Key = %q, want nm_test_key", rec.apiKey)
		}
		if rec.auth != "Bearer user-jwt" {
			t.Errorf("Authorization = %q, want Bearer user-jwt", rec.auth)
		}
	})

	t.Run("proxy GET (taxonomy read) → internal + key", func(t *testing.T) {
		if _, err := c.Proxy(ctx, http.MethodGet, "/tag/search?q=x", "", nil, ""); err != nil {
			t.Fatalf("Proxy GET: %v", err)
		}
		if rec.path != "/internal/tag/search" {
			t.Errorf("path = %q, want /internal/tag/search", rec.path)
		}
		if rec.apiKey != "nm_test_key" {
			t.Errorf("X-API-Key = %q, want nm_test_key", rec.apiKey)
		}
	})

	t.Run("proxy POST (taxonomy write) → legacy, no key", func(t *testing.T) {
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

	t.Run("submit write → legacy, no key", func(t *testing.T) {
		if _, err := c.SubmitGalgame(ctx, "user-jwt", map[string]any{"x": 1}); err != nil {
			t.Fatalf("SubmitGalgame: %v", err)
		}
		if rec.path != "/api/galgame/submit" {
			t.Errorf("path = %q, want /api/galgame/submit", rec.path)
		}
		if rec.apiKey != "" {
			t.Errorf("X-API-Key = %q, want empty on legacy write face", rec.apiKey)
		}
	})

	t.Run("update write → legacy, no key", func(t *testing.T) {
		if _, err := c.UpdateGalgame(ctx, "user-jwt", 5, map[string]any{"x": 1}); err != nil {
			t.Fatalf("UpdateGalgame: %v", err)
		}
		if rec.path != "/api/galgame/5" {
			t.Errorf("path = %q, want /api/galgame/5", rec.path)
		}
		if rec.apiKey != "" {
			t.Errorf("X-API-Key = %q, want empty on legacy write face", rec.apiKey)
		}
	})

	t.Run("image upload proxy → legacy, no key", func(t *testing.T) {
		if _, err := c.UploadGalgameImage(ctx, "user-jwt", "galgame_cover", "f.png", []byte("x"), "image/png"); err != nil {
			t.Fatalf("UploadGalgameImage: %v", err)
		}
		if rec.path != "/api/galgame/image" {
			t.Errorf("path = %q, want /api/galgame/image", rec.path)
		}
		if rec.apiKey != "" {
			t.Errorf("X-API-Key = %q, want empty on legacy write face", rec.apiKey)
		}
	})

	t.Run("messages feed → internal + key (S2S cron)", func(t *testing.T) {
		if _, err := c.GetWikiMessageFeed(ctx, 0, 10); err != nil {
			t.Fatalf("GetWikiMessageFeed: %v", err)
		}
		if rec.path != "/internal/galgame/messages/feed" {
			t.Errorf("path = %q, want /internal/galgame/messages/feed", rec.path)
		}
		if rec.apiKey != "nm_test_key" {
			t.Errorf("X-API-Key = %q, want nm_test_key on internal feed face", rec.apiKey)
		}
	})
}

// TestMessageFeedRequiresKey proves the S2S message feed hard-depends on the
// internal-tier key: with no key configured it errors before dialing rather
// than silently falling back (the rollback valve was retired in wave 05).
func TestMessageFeedRequiresKey(t *testing.T) {
	rec := &faceRecorder{}
	srv := rec.server(t)
	c := NewWithKey(srv.URL, "") // no API key
	ctx := context.Background()

	if _, err := c.GetWikiMessageFeed(ctx, 0, 10); err == nil {
		t.Fatal("GetWikiMessageFeed with empty key: want error, got nil")
	}
	if rec.path != "" {
		t.Errorf("recorder path = %q, want empty (must not dial without a key)", rec.path)
	}
}
