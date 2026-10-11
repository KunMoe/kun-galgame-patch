package common

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	commentService "kun-galgame-patch-api/internal/comment/service"
	galgameClient "kun-galgame-patch-api/internal/galgame/client"

	"github.com/gofiber/fiber/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const blockQuery = "exclude_tag_trees=12,6&exclude_tag_ids=9"

type exclusionCatalog struct {
	mu   sync.Mutex
	seen []*url.URL
}

func (f *exclusionCatalog) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.seen = append(f.seen, r.URL)
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v2/catalog/tag-exclusion" {
			_, _ = w.Write([]byte(`{"object":"tag_exclusion","tag_ids":["6","9","12","13"],"work_ids":["11","3021"]}`))
			return
		}
		_, _ = w.Write([]byte(`{"object":"list","items":[],"total":0}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (f *exclusionCatalog) requests(path string) []url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []url.Values
	for _, u := range f.seen {
		if u.Path == path {
			out = append(out, u.Query())
		}
	}
	return out
}

// A dry-run connection renders every statement the handler builds without a
// database, so CI (which has none) still checks what reaches Postgres.
func recordingDB(t *testing.T) (*gorm.DB, func() []string) {
	t.Helper()
	db, err := gorm.Open(
		postgres.New(postgres.Config{DSN: "postgres://u:p@127.0.0.1:1/none", PreferSimpleProtocol: true}),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	var (
		mu    sync.Mutex
		stmts []string
	)
	record := func(tx *gorm.DB) {
		mu.Lock()
		defer mu.Unlock()
		stmts = append(stmts, tx.Dialector.Explain(tx.Statement.SQL.String(), tx.Statement.Vars...))
	}
	if err := db.Callback().Query().After("gorm:query").Register("test:record", record); err != nil {
		t.Fatal(err)
	}
	return db, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), stmts...)
	}
}

type noComments struct{}

func (noComments) SiteFeed(context.Context, string, int, string, commentService.PatchSummaryDB) (*commentService.FeedPage, error) {
	return &commentService.FeedPage{Items: []*commentService.FeedItem{}}, nil
}
func (noComments) AuthorBoard(context.Context, int) []int            { return nil }
func (noComments) AuthorCounts(context.Context, []int) map[int]int64 { return nil }

func exclusionApp(t *testing.T) (*fiber.App, *exclusionCatalog, func() []string) {
	t.Helper()
	cat := &exclusionCatalog{}
	srv := cat.server(t)
	db, stmts := recordingDB(t)
	h := NewHandler(db, galgameClient.NewWithKey(srv.URL, "nm_test_key"), nil, nil, nil, noComments{})
	app := fiber.New()
	app.Get("/home", h.GetHome)
	app.Get("/galgame", h.GetGalgameList)
	app.Get("/galgame/calendar", h.GetGalgameCalendar)
	app.Get("/resource", h.GetGlobalResources)
	app.Get("/ranking/patch", h.GetPatchRanking)
	app.Get("/search", h.SiteSearch)
	app.Get("/search/overview", h.SiteSearchOverview)
	return app, cat, stmts
}

func get(t *testing.T, app *fiber.App, target string) (int, string) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, target, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func withQuery(target, query string) string {
	if strings.Contains(target, "?") {
		return target + "&" + query
	}
	return target + "?" + query
}

// The lists built from moyu's own tables cannot hand a tag to a query catalog
// never sees; they ask catalog which works the tags cover and drop those ids,
// in the builder that produces both the page and its COUNT.
func TestLocalListsDropTheWorksABlockListCovers(t *testing.T) {
	for _, tc := range []struct {
		target string
		want   map[string]int
	}{
		{"/galgame?selected_type=all&sort_field=resource_update_time&sort_order=desc&page=1&limit=24",
			map[string]int{"patch.id": 2}},
		{"/ranking/patch?sort_by=download", map[string]int{"patch.id": 1}},
		{"/resource?sort_field=created&sort_order=desc&page=1&limit=20",
			map[string]int{"patch_resource.galgame_id": 2}},
		{"/home", map[string]int{"patch.id": 1, "patch_resource.galgame_id": 1}},
		{"/search?keywords=kun&type=resource&page=1&limit=12",
			map[string]int{"patch_resource.galgame_id": 2}},
	} {
		t.Run(tc.target, func(t *testing.T) {
			app, cat, stmts := exclusionApp(t)
			if status, body := get(t, app, withQuery(tc.target, blockQuery)); status != http.StatusOK {
				t.Fatalf("status = %d, body = %s", status, body)
			}
			asked := cat.requests("/v2/catalog/tag-exclusion")
			if len(asked) != 1 || asked[0].Get("exclude_tag_tree") != "6,12" || asked[0].Get("exclude_tag_id") != "9" {
				t.Fatalf("tag-exclusion requests = %v, want one for trees 6,12 and tag 9", asked)
			}
			for column, n := range tc.want {
				predicate := column + " NOT IN (SELECT unnest('{11,3021}'::int[]))"
				got := 0
				for _, s := range stmts() {
					if strings.Contains(s, predicate) {
						got++
					}
				}
				if got != n {
					t.Errorf("%d statements carry %s, want %d:\n%s", got, predicate, n, strings.Join(stmts(), "\n"))
				}
			}
		})
	}
}

func TestAReaderWithNoBlocksCostsNothing(t *testing.T) {
	app, cat, stmts := exclusionApp(t)
	for _, target := range []string{
		"/galgame?selected_type=all&sort_field=resource_update_time&sort_order=desc&page=1&limit=24",
		"/ranking/patch",
		"/resource?sort_field=created&sort_order=desc&page=1&limit=20",
		"/home",
		"/search?keywords=kun&type=resource&page=1&limit=12",
	} {
		if status, body := get(t, app, target); status != http.StatusOK {
			t.Fatalf("%s: status = %d, body = %s", target, status, body)
		}
	}
	if asked := cat.requests("/v2/catalog/tag-exclusion"); len(asked) != 0 {
		t.Errorf("tag-exclusion was asked %d times without a block list", len(asked))
	}
	for _, s := range stmts() {
		if strings.Contains(s, "unnest(") {
			t.Errorf("statement carries an exclusion: %s", s)
		}
	}
}

func TestCatalogListsForwardTheBlockList(t *testing.T) {
	for _, tc := range []struct {
		target, path string
	}{
		{"/galgame?selected_type=all&sort_field=popularity&sort_order=desc&page=1&limit=24&library=true", "/v2/catalog/works"},
		{"/galgame/calendar?month=2026-10", "/v2/catalog/calendar"},
		{"/search?keywords=kun&type=galgame&page=1&limit=12", "/v2/catalog/works"},
		{"/search/overview?keywords=kun", "/v2/catalog/works"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			app, cat, _ := exclusionApp(t)
			if status, body := get(t, app, withQuery(tc.target, blockQuery)); status != http.StatusOK {
				t.Fatalf("status = %d, body = %s", status, body)
			}
			got := cat.requests(tc.path)
			if len(got) == 0 {
				t.Fatalf("%s was never asked", tc.path)
			}
			for _, q := range got {
				if q.Get("exclude_tag_tree") != "6,12" || q.Get("exclude_tag_id") != "9" {
					t.Errorf("%s?%s does not carry the block list", tc.path, q.Encode())
				}
			}
		})
	}
}

func TestMoreBlockedTagsThanCatalogTakesIsTheReadersError(t *testing.T) {
	ids := make([]string, 101)
	for i := range ids {
		ids[i] = strconv.Itoa(i + 1)
	}
	app, cat, _ := exclusionApp(t)
	for _, target := range []string{
		"/galgame?selected_type=all&sort_field=resource_update_time&sort_order=desc&page=1&limit=24",
		"/galgame/calendar",
		"/home",
		"/search?keywords=kun&type=galgame&page=1&limit=12",
	} {
		status, body := get(t, app, withQuery(target, "exclude_tag_trees="+strings.Join(ids, ",")))
		if status != http.StatusBadRequest || !strings.Contains(body, "最多屏蔽 100 个标签") {
			t.Errorf("%s: status = %d, body = %s", target, status, body)
		}
	}
	cat.mu.Lock()
	defer cat.mu.Unlock()
	if len(cat.seen) != 0 {
		t.Errorf("catalog was asked %d times for a refused request", len(cat.seen))
	}
}
