package common

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// Every dependency is nil on purpose: the guards have to answer before a lane
// touches GORM or the catalog client, so a regression shows up as a panic
// rather than a 500 nobody reads.
func newSearchTestApp() *fiber.App {
	h := NewHandler(nil, nil, nil, nil, nil)
	app := fiber.New()
	app.Get("/search", h.SiteSearch)
	app.Get("/search/overview", h.SiteSearchOverview)
	app.Get("/search/quick", h.SiteSearchQuick)
	app.Get("/search/entity", h.SiteSearchEntity)
	app.Get("/search/entity/resolve", h.SiteSearchEntityResolve)
	return app
}

func assertSearchBadRequest(t *testing.T, app *fiber.App, url, want string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "http://localhost"+url, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", resp.StatusCode, body)
	}
	if want != "" && !strings.Contains(string(body), want) {
		t.Errorf("body = %s, want it to contain %q", body, want)
	}
}

// `required` only rejects the empty string, so a query of nothing but spaces
// reaches the lanes as a keyword list of length zero — and the resource lane
// would then ILIKE nothing while holding a nil *gorm.DB.
func TestSiteSearchRejectsWhitespaceOnlyKeywords(t *testing.T) {
	app := newSearchTestApp()
	for _, url := range []string{
		"/search?keywords=%20%20&type=galgame&page=1&limit=12",
		"/search?keywords=%20%20&type=resource&page=1&limit=12",
		"/search?keywords=%20%20&type=user&page=1&limit=12",
		"/search/overview?keywords=%20%20",
		"/search/quick?keywords=%20%20",
		"/search/entity?keywords=%20%20&page=1&limit=8",
	} {
		t.Run(url, func(t *testing.T) {
			assertSearchBadRequest(t, app, url, "搜索关键词不能为空")
		})
	}
}

func TestSiteSearchRejectsAnUnknownType(t *testing.T) {
	assertSearchBadRequest(t, newSearchTestApp(),
		"/search?keywords=kun&type=topic&page=1&limit=12", "Type")
}

// engine is a catalog family moyu has no page for, so it must not reach the
// lane — a card that cannot be clicked is worse than one that is missing.
func TestSiteSearchEntityRejectsAFamilyWithNoPage(t *testing.T) {
	assertSearchBadRequest(t, newSearchTestApp(),
		"/search/entity?keywords=kun&family=engine&page=1&limit=8", "Family")
}

// Both are closed vocabularies catalog would answer 400 for anyway; refusing
// them here keeps the reason readable and the upstream request unsent.
func TestSiteSearchRejectsUnknownLaneFilters(t *testing.T) {
	app := newSearchTestApp()
	for _, tc := range []struct{ url, want string }{
		{"/search?keywords=kun&type=galgame&page=1&limit=12&sort=rating", "Sort"},
		{"/search?keywords=kun&type=resource&page=1&limit=12&scope=note", "Scope"},
		{"/search/entity/resolve?family=character&ids=1", "Family"},
	} {
		t.Run(tc.url, func(t *testing.T) {
			assertSearchBadRequest(t, app, tc.url, tc.want)
		})
	}
}

func TestParseSearchIDsDropsWhatCatalogCannotUse(t *testing.T) {
	got := parseSearchIDs(" 78 ,78, 0, -3, abc,1093,")
	if len(got) != 2 || got[0] != 78 || got[1] != 1093 {
		t.Fatalf("parseSearchIDs = %v, want [78 1093]", got)
	}
}

func TestSearchKeywordsSplitsOnTheSpaceChineseKeyboardsType(t *testing.T) {
	// U+3000, what a full-width IME emits for the space bar. strings.Fields
	// splits on it because unicode.IsSpace does; a hand-rolled strings.Split on
	// " " would hand the whole phrase over as one keyword.
	got := searchKeywords("　汉化　补丁　")
	if len(got) != 2 || got[0] != "汉化" || got[1] != "补丁" {
		t.Fatalf("searchKeywords = %q, want [汉化 补丁]", got)
	}
}
