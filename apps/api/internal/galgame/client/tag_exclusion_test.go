package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"kun-galgame-patch-api/pkg/catalogv2"
)

var testExclusion = catalogv2.TagExclusion{Exact: []int64{9}, Trees: []int64{6, 12}}

func wantExclusion(t *testing.T, name string, q url.Values) {
	t.Helper()
	if q.Get("exclude_tag_id") != "9" || q.Get("exclude_tag_tree") != "6,12" {
		t.Errorf("%s sent exclude_tag_id=%q exclude_tag_tree=%q, want 9 and 6,12",
			name, q.Get("exclude_tag_id"), q.Get("exclude_tag_tree"))
	}
}

// A hydrate by ids= only redraws rows a filtered list already chose, so the
// assertion is on the requests that choose rows.
func TestEveryCatalogListLaneForwardsTheExclusion(t *testing.T) {
	srv := newCatalogFake(t)
	c := NewWithKey(srv.URL, "nm_test_key")
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"tag page", func() error {
			_, _, err := c.TaxonomyBrowse(ctx, "/tag/_?tag_id=11", testExclusion)
			return err
		}},
		{"series page", func() error {
			_, _, err := c.TaxonomyBrowse(ctx, "/series/_?series_id=11", testExclusion)
			return err
		}},
		{"company page walks the rollup", func() error {
			_, _, err := c.TaxonomyBrowse(ctx, "/official/_?official_id=31", testExclusion)
			return err
		}},
		{"library and search", func() error {
			_, err := c.SearchGalgame(ctx, SearchGalgameParams{Q: "kun", Exclude: testExclusion})
			return err
		}},
		{"resource search's title match", func() error {
			_, err := c.SearchGalgameIDs(ctx, "kun", "sfw", 60, testExclusion)
			return err
		}},
		{"calendar", func() error {
			_, err := c.GetGalgameCalendar(ctx, "2026-10", "", testExclusion)
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv.reset()
			if err := tc.call(); err != nil {
				t.Fatal(err)
			}
			chose := 0
			for _, r := range srv.all() {
				listing := r.path == "/v2/catalog/calendar" ||
					(r.path == "/v2/catalog/works" && r.query.Get("ids") == "" && r.query.Get("refs") == "")
				if listing {
					chose++
					wantExclusion(t, r.path, r.query)
				}
			}
			if chose == 0 {
				t.Fatalf("no list request reached catalog: %+v", srv.all())
			}
		})
	}
}

// The credit and appearance sub-faces take no exclusion, so the works they name
// are dropped by the ids= hydrate, which does.
func TestEntityWorksDropBlockedWorksAtTheHydrate(t *testing.T) {
	var (
		mu      sync.Mutex
		hydrate url.Values
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v2/catalog/credit-names/5/credits":
			_, _ = w.Write([]byte(`{"object":"list","items":[{"work":{"object":"work","id":"7","display_name":"x"},"roles":[]}],"next_cursor":null}`))
		case "/v2/catalog/works":
			mu.Lock()
			hydrate = r.URL.Query()
			mu.Unlock()
			_, _ = w.Write([]byte(`{"object":"list","items":[],"missing":["7"],"next_cursor":null}`))
		default:
			_, _ = w.Write([]byte(`{"object":"list","items":[]}`))
		}
	}))
	t.Cleanup(srv.Close)

	page, err := NewWithKey(srv.URL, "nm_test_key").StaffWorks(context.Background(), 5, "sfw", "", 24, testExclusion)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 0 {
		t.Errorf("a work catalog reported missing was still listed: %+v", page.Items)
	}
	mu.Lock()
	defer mu.Unlock()
	if hydrate.Get("ids") != "7" {
		t.Fatalf("hydrate query = %v", hydrate)
	}
	wantExclusion(t, "hydrate", hydrate)
}
