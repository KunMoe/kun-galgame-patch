package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

type wizardRecorder struct {
	mu       sync.Mutex
	catalogQ url.Values
	wikiHits int
}

func (r *wizardRecorder) client(t *testing.T) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		body := `{"object":"list","items":[]}`
		switch {
		case req.URL.Path == "/v2/catalog/works":
			r.catalogQ = req.URL.Query()
			body = `{"object":"list","total":2,"items":[
			  {"id":"11","display_name":"A","content_rating":"r18",
			   "claim":{"site":"galgame_wiki","site_work_id":"292","state":"live","content_limit":"nsfw"},
			   "localized":{"ja":{"value":"白恋サクラ","is_machine":false}},"refs":[{"source":"vndb","external_id":"v22610"}]},
			  {"id":"12","display_name":"B","content_rating":"r18",
			   "claim":{"site":"galgame_wiki","site_work_id":"9978","state":"draft","content_limit":"nsfw"}},
			  {"id":"13","display_name":"withdrawn","content_rating":"r18",
			   "claim":{"site":"galgame_wiki","site_work_id":"404","state":"hidden","content_limit":"nsfw"}},
			  {"id":"14","display_name":"unclaimed","content_rating":"r18","claim":null}
			]}`
		case strings.HasSuffix(req.URL.Path, "/galgame/search"):
			r.wikiHits++
		}
		r.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return NewWithKey(srv.URL, "nm_test_key")
}

type wizardItems struct {
	Items []GalgameHit
	Total int64
}

func (r *wizardRecorder) search(t *testing.T) wizardItems {
	t.Helper()
	items, total, err := r.client(t).SearchPublishItems(context.Background(), "sakura", 12)
	if err != nil {
		t.Fatalf("SearchPublishItems: %v", err)
	}
	return wizardItems{Items: items, Total: total}
}

func TestPublishWizard_ItemsComeFromTheCatalog(t *testing.T) {
	rec := &wizardRecorder{}
	out := rec.search(t)

	if got := rec.catalogQ.Get("claim_state"); got != "" {
		t.Errorf("claim_state = %q, want it absent — the wizard searches the whole catalog", got)
	}
	if got := rec.catalogQ.Get("claimed"); got != "" {
		t.Errorf("claimed = %q, want it absent — unclaimed rows are actionable", got)
	}
	if got := rec.catalogQ.Get("q"); got != "sakura" {
		t.Errorf("q = %q, want sakura", got)
	}
	if got := rec.catalogQ.Get("limit"); got != "12" {
		t.Errorf("limit = %q, want 12", got)
	}
	if got := rec.catalogQ.Get("nsfw"); got != "true" {
		t.Errorf("nsfw = %q, want true", got)
	}
	if got := rec.catalogQ.Get("content_limit"); got != "" {
		t.Errorf("content_limit = %q, want it absent on the wizard lane", got)
	}
	if !strings.Contains(rec.catalogQ.Get("include"), "refs") {
		t.Errorf("include = %q, want refs (the row prints the VNDB id)", rec.catalogQ.Get("include"))
	}
	if out.Total != 2 {
		t.Errorf("total = %d, want the catalog total 2", out.Total)
	}
}

// The row the wizard hands back is keyed by the catalog id, which is the page
// the publish flow then creates. It used to be keyed by the claim's
// site_work_id, so picking a game in the wizard opened a page id that belonged
// to a different game here.
func TestPublishWizard_ItemsAreCatalogKeyedAndDropWithdrawnRows(t *testing.T) {
	rec := &wizardRecorder{}
	out := rec.search(t)

	if len(out.Items) != 3 {
		t.Fatalf("items = %d, want 3 (hidden claims drop; unclaimed rows are the library)", len(out.Items))
	}
	if out.Items[0].ID != 11 || out.Items[1].ID != 12 || out.Items[2].ID != 14 {
		t.Errorf("ids = %d,%d,%d, want 11,12,14", out.Items[0].ID, out.Items[1].ID, out.Items[2].ID)
	}
	if out.Items[0].ClaimState != "live" || out.Items[1].ClaimState != "draft" || out.Items[2].ClaimState != "" {
		t.Errorf("claim states = %q,%q,%q, want live,draft,empty",
			out.Items[0].ClaimState, out.Items[1].ClaimState, out.Items[2].ClaimState)
	}
	if out.Items[0].VndbID != "v22610" {
		t.Errorf("vndb_id = %q, want v22610", out.Items[0].VndbID)
	}
}

func TestPublishWizard_NeverTouchesTheWikiFace(t *testing.T) {
	rec := &wizardRecorder{}
	rec.search(t)

	if rec.wikiHits != 0 {
		t.Errorf("wiki face hits = %d, want 0 — the caller's own submissions come "+
			"from the registry's per-user claim face now, composed in by the BFF", rec.wikiHits)
	}
}

func TestPublishWizard_EmptyResultIsAnArrayNotNull(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{}}`))
	}))
	t.Cleanup(srv.Close)

	items, _, err := NewWithKey(srv.URL, "nm_test_key").
		SearchPublishItems(context.Background(), "nothing", 12)
	if err != nil {
		t.Fatalf("SearchPublishItems: %v", err)
	}
	if items == nil {
		t.Error("items = nil, want an empty slice")
	}
}
