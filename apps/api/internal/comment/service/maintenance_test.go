package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"kun-galgame-patch-api/pkg/communityclient"
)

func sweepService(t *testing.T, page func(cursor string) (content, next string)) *Service {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content, next := page(r.URL.Query().Get("cursor"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "message": "ok", "data": map[string]any{
			"posts": []map[string]any{{
				"post":   map[string]any{"id": 1, "content_raw": content},
				"thread": map[string]any{"thread_id": 1, "anchor_kind": communityclient.AnchorSiteGame, "anchor_id": "42"},
			}},
			"next_cursor": next,
		}})
	}))
	t.Cleanup(srv.Close)
	return &Service{community: communityclient.New(communityclient.Config{
		BaseURL: srv.URL, ClientID: "moyu", ClientSecret: "secret",
	})}
}

func token(n int) string {
	h := strconv.Itoa(n)
	return "/image/" + strings.Repeat("0", 64-len(h)) + h
}

func TestImageSweepReadsPastFiveHundredPages(t *testing.T) {
	const pages = 600
	svc := sweepService(t, func(cursor string) (string, string) {
		n, _ := strconv.Atoi(cursor)
		next := ""
		if n+1 < pages {
			next = strconv.Itoa(n + 1)
		}
		return "![](" + token(n) + ")", next
	})
	hashes, err := svc.CollectImageHashes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(hashes) != pages {
		t.Fatalf("collected %d hashes, want %d", len(hashes), pages)
	}
}

func TestImageSweepFailsOnAStuckCursor(t *testing.T) {
	svc := sweepService(t, func(string) (string, string) { return "", "same" })
	if _, err := svc.CollectImageHashes(context.Background()); err == nil {
		t.Fatal("a cursor that does not advance must fail the sweep, not loop or pass")
	}
}
