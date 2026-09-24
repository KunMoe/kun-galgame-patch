package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"kun-galgame-patch-api/internal/favorite"
	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/internal/patch/service"
	"kun-galgame-patch-api/internal/testutil"
	"kun-galgame-patch-api/internal/usercache"
	"kun-galgame-patch-api/pkg/catalogv2/catalogv2test"
	"kun-galgame-patch-api/pkg/config"

	"github.com/gofiber/fiber/v3"
)

const appKey = "nm_test_key"

// meFace is a catalog that counts what the reader's own token spent: every
// call not made with the application key comes out of the per-user allowance
// the reader shares with every other NextMoe site.
type meFace struct {
	mu     sync.Mutex
	routes map[string]string
	spent  map[string]int
}

func (f *meFace) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		route := r.Method + " " + r.URL.Path
		f.mu.Lock()
		if r.Header.Get("Authorization") != "Bearer "+appKey {
			f.spent[route]++
		}
		body, ok := f.routes[route]
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if !ok {
			catalogv2test.Problem(w, r, "NOT_FOUND", "Nothing visible exists at this URL.", nil)
			return
		}
		if body == "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = w.Write([]byte(body))
	}
}

func (f *meFace) total() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.spent {
		n += c
	}
	return n
}

func (f *meFace) calls(route string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.spent[route]
}

func newQuotaApp(t *testing.T, routes map[string]string) (*meFace, *PatchHandler, *testutil.TestApp, string) {
	t.Helper()
	f := &meFace{routes: routes, spent: map[string]int{}}
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	ta := testutil.NewTestApp(t)
	gal := galgameClient.NewWithKey(srv.URL, appKey)
	mine := usercache.New(ta.RDB)
	svc := service.New(nil, nil, nil, nil, gal, favorite.New(gal, mine), nil, nil, nil)
	return f, New(svc, gal, mine, nil, nil), ta, ta.CreateTestSession(t, 42)
}

func ok(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body = %s", resp.StatusCode, testutil.ReadBody(t, resp))
	}
	data, _ := testutil.ParseResponse(t, resp).Data.(map[string]any)
	return data
}

// GET /patch/:id runs on every navigation, server-side included, so it carries
// nothing about the reader: the heart it used to carry cost one of their calls
// per view.
func TestTheGamePageHeaderCarriesNoViewerState(t *testing.T) {
	raw, err := json.Marshal(headerCard{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"is_favorite"`) {
		t.Fatalf("the header carries viewer state again: %s", raw)
	}
}

func TestTheHeartCostsOneShelfWalkPerTTLNotOneCallPerView(t *testing.T) {
	f, h, ta, session := newQuotaApp(t, map[string]string{
		"GET /v2/me/folders":         `{"object":"list","items":[{"id":"9","is_default":true,"item_count":1}]}`,
		"GET /v2/me/folders/9/items": `{"object":"list","items":[{"folder_id":"9","work_id":"9000"}]}`,
	})
	ta.App.Get("/patch/:id/favorite", middleware.Auth(ta.RDB, config.OAuthConfig{}), h.GetFavorite)

	for _, tc := range []struct {
		id   int
		want bool
	}{{9000, true}, {9001, false}, {9000, true}, {9002, false}} {
		data := ok(t, ta.Request(t, http.MethodGet, fmt.Sprintf("/patch/%d/favorite", tc.id), "", session))
		if data["favorited"] != tc.want {
			t.Fatalf("patch %d favorited = %v, want %v", tc.id, data["favorited"], tc.want)
		}
	}
	if f.total() != 2 {
		t.Fatalf("four game pages spent %d of the reader's calls, want one walk of two", f.total())
	}
}

// A shelf catalog cannot read is drawn as not favourited; the page never fails.
func TestAnUnreadableShelfIsAnEmptyHeartNotAnError(t *testing.T) {
	_, h, ta, session := newQuotaApp(t, map[string]string{})
	ta.App.Get("/patch/:id/favorite", middleware.Auth(ta.RDB, config.OAuthConfig{}), h.GetFavorite)
	data := ok(t, ta.Request(t, http.MethodGet, "/patch/9000/favorite", "", session))
	if data["favorited"] != false {
		t.Fatalf("favorited = %v", data["favorited"])
	}
}

// The claim is read off the application key, so deciding costs the uploader
// nothing, and a work already live here costs them nothing at all. Anything
// else is adopted on every upload, which is what lets a bot-first page or an
// adoption that failed be picked up by the next person who uploads.
func TestAnUploadAdoptsTheWorkUnlessItIsLiveHere(t *testing.T) {
	for _, tc := range []struct {
		name  string
		works string
		want  int
	}{
		{"live on this site", `[{"object":"work","id":"9000","claim":{"site":"kungal","state":"live"}}]`, 0},
		{"live under the legacy site key", `[{"object":"work","id":"9000","claim":{"site":"galgame_wiki","state":"live"}}]`, 0},
		{"banned", `[{"object":"work","id":"9000","claim":{"site":"kungal","state":"hidden"}}]`, 0},
		{"gone from catalog", `[]`, 0},
		{"never claimed", `[{"object":"work","id":"9000","claim":null}]`, 2},
		{"a draft", `[{"object":"work","id":"9000","claim":{"site":"kungal","state":"draft"}}]`, 2},
		{"declined", `[{"object":"work","id":"9000","claim":{"site":"kungal","state":"declined"}}]`, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, h, ta, session := newQuotaApp(t, map[string]string{
				"GET /v2/catalog/works":    `{"object":"list","items":` + tc.works + `}`,
				"POST /v2/me/claims":       `{"object":"claim","id":"9000","state":"draft"}`,
				"PATCH /v2/me/claims/9000": `{"object":"claim","id":"9000","state":"live"}`,
			})
			ta.App.Post("/upload", middleware.Auth(ta.RDB, config.OAuthConfig{}), func(c fiber.Ctx) error {
				h.adoptUnlessLive(c, 9000)
				return c.SendStatus(http.StatusOK)
			})
			ta.Request(t, http.MethodPost, "/upload", "", session)
			if f.total() != tc.want {
				t.Fatalf("the upload spent %d of the uploader's calls (%v), want %d", f.total(), f.spent, tc.want)
			}
			if tc.want == 2 && (f.calls("POST /v2/me/claims") != 1 || f.calls("PATCH /v2/me/claims/9000") != 1) {
				t.Fatalf("want the adopt pair, got %v", f.spent)
			}
		})
	}
}

// product_work_id is the forum's page id. A wizard row linked by it names a
// different game on this site whenever the two numbers differ.
func TestTheWizardNamesAPendingClaimByItsWorkID(t *testing.T) {
	_, h, ta, session := newQuotaApp(t, map[string]string{
		"GET /v2/catalog/works": `{"object":"list","items":[]}`,
		"GET /v2/me/claims": `{"object":"list","items":[{"object":"claim","id":"77","state":"pending",` +
			`"display_name":"白恋サクラ","site":"kungal","product_work_id":"4242"}]}`,
	})
	ta.App.Get("/galgame/search/publish", middleware.Auth(ta.RDB, config.OAuthConfig{}), h.SearchGalgameForPublish)
	data := ok(t, ta.Request(t, http.MethodGet, "/galgame/search/publish?q=", "", session))
	pending, _ := data["pending"].([]any)
	if len(pending) != 1 {
		t.Fatalf("pending = %v", pending)
	}
	if id := pending[0].(map[string]any)["id"]; id != float64(77) {
		t.Fatalf("pending id = %v, want the catalog work id 77, not the forum's 4242", id)
	}
}

func TestThePublishWizardAsksForOwnSubmissionsOncePerMinute(t *testing.T) {
	f, h, ta, session := newQuotaApp(t, map[string]string{
		"GET /v2/catalog/works": `{"object":"list","items":[]}`,
		"GET /v2/me/claims": `{"object":"list","items":[{"object":"claim","id":"77","state":"pending",` +
			`"display_name":"白恋サクラ","site":"kungal"}]}`,
		"GET /v2/me/claims/9000":   `{"object":"claim","id":"9000","state":"live"}`,
		"PATCH /v2/me/claims/9000": `{"object":"claim","id":"9000","state":"draft"}`,
	})
	auth := middleware.Auth(ta.RDB, config.OAuthConfig{})
	ta.App.Get("/galgame/search/publish", auth, h.SearchGalgameForPublish)
	ta.App.Delete("/galgame/:gid", auth, h.WithdrawGalgameSubmission)

	for _, q := range []string{"白", "白恋", "白恋サ", "nothing"} {
		data := ok(t, ta.Request(t, http.MethodGet, "/galgame/search/publish?q="+url.QueryEscape(q), "", session))
		pending, _ := data["pending"].([]any)
		if want := q != "nothing"; (len(pending) == 1) != want {
			t.Fatalf("q=%s pending = %v", q, pending)
		}
	}
	if n := f.calls("GET /v2/me/claims"); n != 1 {
		t.Fatalf("four searches read the reader's claims %d times, want 1", n)
	}

	ok(t, ta.Request(t, http.MethodDelete, "/galgame/9000", "", session))
	ok(t, ta.Request(t, http.MethodGet, "/galgame/search/publish?q="+url.QueryEscape("白"), "", session))
	if n := f.calls("GET /v2/me/claims"); n != 2 {
		t.Fatal("a withdraw left the wizard showing the list from before it")
	}

	ta.MR.FastForward(wizardClaimsTTL + time.Second)
	ok(t, ta.Request(t, http.MethodGet, "/galgame/search/publish?q="+url.QueryEscape("白"), "", session))
	if n := f.calls("GET /v2/me/claims"); n != 3 {
		t.Fatal("the wizard's list outlived its minute")
	}
}

// Opening somebody else's folder spends none of the reader's own allowance;
// only their own private folder needs their token.
func TestOpeningAFolderAsksThePublicLaneFirst(t *testing.T) {
	f, h, ta, session := newQuotaApp(t, map[string]string{
		"GET /v2/folders/5":          `{"id":"5","owner_uid":"7","name":"公开","visibility":"public"}`,
		"GET /v2/folders/5/items":    `{"object":"list","items":[]}`,
		"GET /v2/me/folders/6":       `{"id":"6","owner_uid":"42","name":"私藏","visibility":"private"}`,
		"GET /v2/me/folders/6/items": `{"object":"list","items":[]}`,
	})
	ta.App.Get("/folder/:folderId", middleware.OptionalAuth(ta.RDB, config.OAuthConfig{}), h.FolderDetail)

	data := ok(t, ta.Request(t, http.MethodGet, "/folder/5", "", session))
	if data["folder"].(map[string]any)["name"] != "公开" || f.total() != 0 {
		t.Fatalf("somebody else's folder = %v after %v of the reader's calls", data["folder"], f.spent)
	}
	data = ok(t, ta.Request(t, http.MethodGet, "/folder/6", "", session))
	if data["folder"].(map[string]any)["name"] != "私藏" {
		t.Fatalf("the reader's own private folder = %v", data["folder"])
	}
	if resp := ta.Request(t, http.MethodGet, "/folder/6", "", ""); resp.StatusCode == http.StatusOK {
		t.Fatal("a private folder answered a signed-out reader")
	}
}

func TestReopeningTheEditPageAsksOnlyForProposalsThatChanged(t *testing.T) {
	list := func(updated string) string {
		return `{"object":"list","items":[` +
			`{"id":"31","state":"merged","entity_id":"9000","updated_at":"2026-09-01T00:00:00Z"},` +
			`{"id":"32","state":"open","entity_id":"9000","updated_at":"` + updated + `"}]}`
	}
	routes := map[string]string{
		"GET /v2/me/proposals":    list("2026-09-02T00:00:00Z"),
		"GET /v2/me/proposals/31": `{"id":"31","patch":{"catalog.work.olang":"ja"}}`,
		"GET /v2/me/proposals/32": `{"id":"32","patch":{"catalog.work.display_name":"新"}}`,
	}
	f, h, ta, session := newQuotaApp(t, routes)
	ta.App.Get("/patch/:id/catalog-edit/proposals", middleware.Auth(ta.RDB, config.OAuthConfig{}), h.CatalogEditProposals)

	for range 3 {
		data := ok(t, ta.Request(t, http.MethodGet, "/patch/9000/catalog-edit/proposals", "", session))
		items, _ := data["items"].([]any)
		if len(items) != 2 || items[1].(map[string]any)["patch"] == nil {
			t.Fatalf("items = %v", items)
		}
	}
	if f.calls("GET /v2/me/proposals/31")+f.calls("GET /v2/me/proposals/32") != 2 {
		t.Fatalf("three visits read proposal details %v times, want once each", f.spent)
	}

	f.mu.Lock()
	routes["GET /v2/me/proposals"] = list("2026-09-03T00:00:00Z")
	f.mu.Unlock()
	ok(t, ta.Request(t, http.MethodGet, "/patch/9000/catalog-edit/proposals", "", session))
	if f.calls("GET /v2/me/proposals/31") != 1 || f.calls("GET /v2/me/proposals/32") != 2 {
		t.Fatalf("an amended proposal was not read again, or an unchanged one was: %v", f.spent)
	}
}
