package catalogv2_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kun-galgame-patch-api/pkg/catalogv2"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func cachedClient(t *testing.T, h http.HandlerFunc) (*catalogv2.Client, *miniredis.Miniredis) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return catalogv2.New(srv.URL, "nmk_test_abcdefghijklmnopqrstuvwx12").WithRedis(rdb), mr
}

func emptyWorkList(w http.ResponseWriter) {
	_ = json.NewEncoder(w).Encode(map[string]any{"object": "list", "items": []any{}, "total": 0})
}

func TestCalendarIsServedFromCacheOnTheSecondRead(t *testing.T) {
	hits := 0
	c, mr := cachedClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		emptyWorkList(w)
	})
	ctx := context.Background()
	for range 3 {
		if _, err := c.Calendar(ctx, "2026-09", false, "", 100); err != nil {
			t.Fatal(err)
		}
	}
	if hits != 1 {
		t.Fatalf("upstream hits = %d, want 1", hits)
	}
	if len(mr.Keys()) != 1 {
		t.Fatalf("cache keys = %v", mr.Keys())
	}
	if ttl := mr.TTL(mr.Keys()[0]); ttl != time.Hour {
		t.Fatalf("calendar TTL = %v, want 1h", ttl)
	}
}

// The whole cache rests on this: the S2S GET path sends no header that varies by
// caller, so the URL is the identity of the response. If a face ever stopped
// spelling its content limit into the query, an NSFW body would be handed to a
// reader who asked for the SFW one.
func TestContentLimitsNeverShareACacheKey(t *testing.T) {
	var seen []string
	c, _ := cachedClient(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Query().Get("nsfw"))
		emptyWorkList(w)
	})
	ctx := context.Background()
	for range 2 {
		if _, err := c.Calendar(ctx, "2026-09", false, "", 100); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Calendar(ctx, "2026-09", true, "", 100); err != nil {
			t.Fatal(err)
		}
	}
	if len(seen) != 2 || seen[0] != "false" || seen[1] != "true" {
		t.Fatalf("upstream saw %v, want exactly one false and one true", seen)
	}
}

func TestDetailReadsGetTheShortTTL(t *testing.T) {
	c, mr := cachedClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"object": "company", "id": "7", "display_name": "Key"})
	})
	if _, err := c.GetCompany(context.Background(), 7, false); err != nil {
		t.Fatal(err)
	}
	if len(mr.Keys()) != 1 {
		t.Fatalf("cache keys = %v", mr.Keys())
	}
	if ttl := mr.TTL(mr.Keys()[0]); ttl != 15*time.Second {
		t.Fatalf("detail TTL = %v, want 15s", ttl)
	}
}

// claim-events drives an incremental cursor loop and proposals is what an editor
// reloads after submitting an edit; a cached page on either is a correctness
// bug, not a stale render.
func TestCursorAndEditorLanesAreNeverCached(t *testing.T) {
	hits := 0
	c, mr := cachedClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		_ = json.NewEncoder(w).Encode(map[string]any{"object": "list", "items": []any{}, "total": 0})
	})
	ctx := context.Background()
	for range 2 {
		if _, err := c.ClaimEvents(ctx, 0, 50, "moyu"); err != nil {
			t.Fatal(err)
		}
		if _, err := c.MergedProposalTotal(ctx, 11, "moyu"); err != nil {
			t.Fatal(err)
		}
	}
	if hits != 4 {
		t.Fatalf("upstream hits = %d, want 4 — an excluded lane was cached", hits)
	}
	if len(mr.Keys()) != 0 {
		t.Fatalf("cache keys = %v, want none", mr.Keys())
	}
}

func TestAClientWithoutRedisStillReads(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		emptyWorkList(w)
	}))
	t.Cleanup(srv.Close)
	c := catalogv2.New(srv.URL, "nmk_test_abcdefghijklmnopqrstuvwx12")
	for range 2 {
		if _, err := c.Calendar(context.Background(), "2026-09", false, "", 100); err != nil {
			t.Fatal(err)
		}
	}
	if hits != 2 {
		t.Fatalf("upstream hits = %d, want 2", hits)
	}
}

// An unconfigured deployment carries a nil *Client and expects ErrNotConfigured
// from every read; the first cut of the cache dereferenced the receiver before
// that check and panicked inside the fiber handler instead.
func TestANilClientStillReturnsNotConfigured(t *testing.T) {
	var c *catalogv2.Client
	if _, err := c.GetSchema(context.Background(), "work"); !errors.Is(err, catalogv2.ErrNotConfigured) {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
}
