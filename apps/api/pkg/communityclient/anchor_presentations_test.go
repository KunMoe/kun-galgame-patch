package communityclient_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"kun-galgame-patch-api/pkg/communityclient"
)

// Shapes follow infra's dto/anchor_presentation_dto.go (#324).
func TestWriteAnchorPresentationsSendsTombstonesBare(t *testing.T) {
	var body map[string][]map[string]any
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/anchor-presentations" {
			t.Errorf("wrote to %s %s", r.Method, r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("body: %v", err)
		}
		envelope(w, map[string]any{"results": []any{
			map[string]any{"anchor_kind": 1, "anchor_id": "42", "outcome": "created"},
			map[string]any{"anchor_kind": 2, "anchor_id": "7", "outcome": "invalid", "reason": "url host"},
		}})
	})

	work := int64(42)
	res, err := c.WriteAnchorPresentations(context.Background(), []communityclient.AnchorPresentation{
		{
			AnchorKind: communityclient.AnchorSiteGame, AnchorID: "42", Revision: 1758859200123456,
			Title: "t", URL: "https://www.moyu.moe/galgame/42?tab=comment", WorkID: &work, ContentLimit: "sfw",
		},
		{AnchorKind: communityclient.AnchorSiteResource, AnchorID: "7", Revision: 1758859200123456, Removed: true},
	})
	if err != nil {
		t.Fatalf("WriteAnchorPresentations: %v", err)
	}
	if len(res.Results) != 2 || res.Results[0].AnchorID != "42" || res.Results[0].Outcome != communityclient.ActivityCreated ||
		res.Results[1].AnchorKind != communityclient.AnchorSiteResource || res.Results[1].Reason != "url host" {
		t.Errorf("results = %+v", res.Results)
	}

	live, tomb := body["items"][0], body["items"][1]
	if live["anchor_kind"] != float64(1) || live["anchor_id"] != "42" || live["work_id"] != float64(42) ||
		live["revision"] != float64(1758859200123456) {
		t.Errorf("live item = %v", live)
	}
	for _, f := range []string{"removed", "cover_image_hash"} {
		if _, ok := live[f]; ok {
			t.Errorf("a live item without a cover carried %q: %v", f, live)
		}
	}
	for _, f := range []string{"title", "url", "work_id", "content_limit", "cover_image_hash"} {
		if _, ok := tomb[f]; ok {
			t.Errorf("tombstone carried %q: %v", f, tomb)
		}
	}
	if tomb["removed"] != true {
		t.Errorf("tombstone = %v", tomb)
	}
}

func TestListAnchorPresentationsDecodesEveryStoredField(t *testing.T) {
	var gotQuery string
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/anchor-presentations" {
			t.Errorf("path = %q", r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		envelope(w, map[string]any{
			"presentations": []any{
				map[string]any{
					"anchor_kind": 1, "anchor_id": "42", "title": "t", "url": "https://www.moyu.moe/galgame/42?tab=comment",
					"cover_image_hash": strings.Repeat("a", 64), "work_id": 42, "content_limit": "sfw",
					"revision": 1758859200123456, "removed": false, "updated_at": "2026-09-27T04:00:01Z", "removed_at": nil,
				},
				map[string]any{
					"anchor_kind": 2, "anchor_id": "7", "title": "", "url": "", "cover_image_hash": nil, "work_id": nil,
					"content_limit": "nsfw", "revision": 1758859200999999, "removed": true,
					"updated_at": "2026-09-27T05:00:01Z", "removed_at": "2026-09-27T05:00:01Z",
				},
			},
			"next_cursor": "MTo0Mg",
		})
	})

	page, err := c.ListAnchorPresentations(context.Background(), "MTo0MQ", 1000)
	if err != nil {
		t.Fatalf("ListAnchorPresentations: %v", err)
	}
	for _, want := range []string{"cursor=MTo0MQ", "limit=1000"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
	a, b := page.Presentations[0], page.Presentations[1]
	if a.AnchorKind != 1 || a.AnchorID != "42" || a.WorkID == nil || *a.WorkID != 42 || a.CoverImageHash == nil ||
		a.ContentLimit != "sfw" || a.Revision != 1758859200123456 || a.Removed {
		t.Errorf("live = %+v", a)
	}
	if !b.Removed || b.RemovedAt == nil || b.WorkID != nil || b.CoverImageHash != nil || page.NextCursor != "MTo0Mg" {
		t.Errorf("tombstone = %+v, next = %q", b, page.NextCursor)
	}
}
