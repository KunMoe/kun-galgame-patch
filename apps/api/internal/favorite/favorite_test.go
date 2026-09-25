package favorite

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/usercache"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type face struct {
	mu        sync.Mutex
	paths     []string
	uris      []string
	userCalls int
	body      map[string]string
}

func (f *face) serve(t *testing.T) *galgameClient.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.paths = append(f.paths, r.URL.Path)
		f.uris = append(f.uris, r.URL.RequestURI())
		if r.Header.Get("Authorization") == "Bearer tok" {
			f.userCalls++
		}
		body, ok := f.body[r.URL.Path]
		f.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":"NOT_FOUND","status":404,"detail":"no"}`))
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return galgameClient.NewWithKey(srv.URL, "nmk_test_key")
}

// uncached is the service as the tests without Redis get it: every question
// reaches catalog.
func (f *face) uncached(t *testing.T) *Service {
	return New(f.serve(t), nil)
}

func (f *face) cached(t *testing.T) *Service {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return New(f.serve(t), usercache.New(rdb))
}

func (f *face) seen(path string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.paths {
		if p == path {
			return true
		}
	}
	return false
}

func (f *face) spent() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.userCalls
}

const twoFolders = `{"object":"list","items":[
	{"id":"11","owner_uid":"7","name":"","visibility":"private","is_default":true,"item_count":2},
	{"id":"12","owner_uid":"7","name":"b","visibility":"public","is_default":false,"item_count":1}
],"next_cursor":null}`

func ownShelf() map[string]string {
	return map[string]string{
		"/v2/me/folders": twoFolders,
		"/v2/me/folders/11/items": `{"object":"list","items":[
			{"folder_id":"11","work_id":"285","created_at":"2026-09-03T00:00:00Z"},
			{"folder_id":"11","work_id":"898","created_at":"2026-09-03T00:01:00Z"}
		],"next_cursor":null}`,
		"/v2/me/folders/12/items": `{"object":"list","items":[
			{"folder_id":"12","work_id":"285","created_at":"2026-09-04T00:00:00Z"}
		],"next_cursor":null}`,
	}
}

// A work filed in two folders is one favourite, and the private folder is part
// of the answer only for its owner. Reading a shelf without the owner's token
// is how the 收藏 tab came to show nothing over ten favourites.
func TestWorkIDsOwnerSeesPrivateFoldersAndDedupes(t *testing.T) {
	f := &face{body: ownShelf()}
	got, err := f.uncached(t).WorkIDs(context.Background(), 7, "tok", true)
	if err != nil {
		t.Fatalf("WorkIDs: %v", err)
	}
	if len(got) != 2 || got[0] != 285 || got[1] != 898 {
		t.Fatalf("want [285 898], got %v", got)
	}
}

func TestWorkIDsVisitorSeesOnlyPublicFolders(t *testing.T) {
	f := &face{body: map[string]string{
		"/v2/folders": `{"object":"list","items":[
			{"id":"12","owner_uid":"7","name":"b","visibility":"public","is_default":false,"item_count":1}
		],"next_cursor":null}`,
		"/v2/folders/12/items": `{"object":"list","items":[
			{"folder_id":"12","work_id":"285","created_at":"2026-09-04T00:00:00Z"}
		],"next_cursor":null}`,
	}}
	got, err := f.cached(t).WorkIDs(context.Background(), 7, "tok", false)
	if err != nil {
		t.Fatalf("WorkIDs: %v", err)
	}
	if len(got) != 1 || got[0] != 285 {
		t.Fatalf("want [285], got %v", got)
	}
	if f.seen("/v2/me/folders") {
		t.Fatal("a visitor must not be answered off the owner's own face")
	}
	if f.spent() != 0 {
		t.Fatalf("looking at somebody else's shelf spent %d of the visitor's own calls", f.spent())
	}
}

func TestVisitorWalkIsCachedUntilTheOwnerForgets(t *testing.T) {
	f := &face{body: map[string]string{
		"/v2/folders": `{"object":"list","items":[
			{"id":"12","owner_uid":"7","name":"b","visibility":"public","is_default":false,"item_count":1}
		],"next_cursor":null}`,
		"/v2/folders/12/items": `{"object":"list","items":[
			{"folder_id":"12","work_id":"285","created_at":"2026-09-04T00:00:00Z"}
		],"next_cursor":null}`,
	}}
	svc := f.cached(t)
	ctx := context.Background()
	for range 2 {
		got, err := svc.WorkIDs(ctx, 7, "tok", false)
		if err != nil {
			t.Fatalf("WorkIDs: %v", err)
		}
		if len(got) != 1 || got[0] != 285 {
			t.Fatalf("want [285], got %v", got)
		}
	}
	n := 0
	for _, p := range f.paths {
		if p == "/v2/folders" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("two visitor reads spent %d /v2/folders requests, want 1", n)
	}
	svc.Forget(ctx, 7)
	if _, err := svc.WorkIDs(ctx, 7, "tok", false); err != nil {
		t.Fatal(err)
	}
	n = 0
	for _, p := range f.paths {
		if p == "/v2/folders" {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("a visitor read after Forget spent %d /v2/folders requests, want 2", n)
	}
}

// An empty folder is not worth a request.
func TestWorkIDsSkipsEmptyFolders(t *testing.T) {
	f := &face{body: map[string]string{
		"/v2/me/folders": `{"object":"list","items":[
			{"id":"11","owner_uid":"7","name":"","visibility":"private","is_default":true,"item_count":0}
		],"next_cursor":null}`,
	}}
	got, err := f.uncached(t).WorkIDs(context.Background(), 7, "tok", true)
	if err != nil {
		t.Fatalf("WorkIDs: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want none, got %v", got)
	}
	if f.seen("/v2/me/folders/11/items") {
		t.Fatal("an item_count of 0 should not cost a request")
	}
}

// One walk of the shelf answers every heart, the calendar, and the owner's own
// count and tab until it expires: the game page used to spend one of the
// reader's calls per view, and the profile a folder walk per view, twice.
func TestOneWalkAnswersEveryHeartUntilItExpires(t *testing.T) {
	f := &face{body: ownShelf()}
	svc := f.cached(t)
	ctx := context.Background()

	held, err := svc.Holds(ctx, 7, "tok", 285)
	if err != nil || !held {
		t.Fatalf("Holds(285) = %v, %v", held, err)
	}
	walk := f.spent()
	if walk != 3 {
		t.Fatalf("the walk cost %d calls, want the folder list and two item pages", walk)
	}

	if held, _ := svc.Holds(ctx, 7, "tok", 898); !held {
		t.Fatal("898 is in the private default folder")
	}
	if held, _ := svc.Holds(ctx, 7, "tok", 1); held {
		t.Fatal("1 is in no folder")
	}
	month, _ := svc.HoldsAll(ctx, 7, "tok", []int64{285, 898, 1, 2})
	if !month[285] || !month[898] || month[1] {
		t.Fatalf("calendar set = %v", month)
	}
	if ids, _ := svc.WorkIDs(ctx, 7, "tok", true); len(ids) != 2 {
		t.Fatalf("the owner's own list = %v", ids)
	}
	if folders, _ := svc.OwnFolders(ctx, 7, "tok"); len(folders) != 2 {
		t.Fatalf("the owner's folders = %v", folders)
	}
	if extra := f.spent() - walk; extra != 0 {
		t.Fatalf("questions after the walk spent %d more calls, want 0", extra)
	}
}

// This site's own writes forget the shelf, so the heart the reader just
// pressed is never answered out of the shelf from before the press.
func TestForgetMakesTheNextReadFresh(t *testing.T) {
	f := &face{body: ownShelf()}
	svc := f.cached(t)
	ctx := context.Background()

	if _, err := svc.Holds(ctx, 7, "tok", 285); err != nil {
		t.Fatal(err)
	}
	before := f.spent()
	svc.Forget(ctx, 7)
	if _, err := svc.Holds(ctx, 7, "tok", 285); err != nil {
		t.Fatal(err)
	}
	if f.spent() == before {
		t.Fatal("a read after Forget was answered from the old shelf")
	}
	other := f.spent()
	if _, err := svc.Holds(ctx, 8, "tok", 285); err != nil {
		t.Fatal(err)
	}
	if f.spent() == other {
		t.Fatal("reader 8 was answered out of reader 7's shelf")
	}
}

const largeShelf = `{"object":"list","items":[
	{"id":"11","owner_uid":"7","name":"","visibility":"private","is_default":true,"item_count":5000}
],"next_cursor":null}`

// A shelf past shelfWalkMax pages is not walked for a heart: 51 calls every
// ten minutes costs more than the one call a page view asks.
func TestALargeShelfAsksOneQuestionPerHeart(t *testing.T) {
	f := &face{body: map[string]string{
		"/v2/me/folders": largeShelf,
		"/v2/me/folders/holdings": `{"object":"list","items":[
			{"object":"folder_holding","work_id":"285","folder_ids":["11"]}
		],"next_cursor":null}`,
	}}
	svc := f.cached(t)
	ctx := context.Background()
	for range 2 {
		held, err := svc.Holds(ctx, 7, "tok", 285)
		if err != nil || !held {
			t.Fatalf("Holds = %v, %v", held, err)
		}
	}
	if f.seen("/v2/me/folders/11/items") {
		t.Fatal("a heart walked a 5000-item folder")
	}
	if f.spent() != 3 {
		t.Fatalf("two hearts spent %d calls, want the folder list once and one holdings each", f.spent())
	}
}

func TestHoldsWithoutTokenIsFalse(t *testing.T) {
	f := &face{body: map[string]string{}}
	held, err := f.uncached(t).Holds(context.Background(), 7, "", 285)
	if err != nil || held {
		t.Fatalf("want false/nil, got %v/%v", held, err)
	}
	if len(f.paths) != 0 {
		t.Fatalf("a signed-out heart reached catalog: %v", f.paths)
	}
}

// The catalog leaves a work held nowhere out of its answer rather than
// returning it with an empty folder list, so the map's zero value is the
// negative and a caller must never read length as "none held".
func TestHoldsAllLeavesUnheldWorksOut(t *testing.T) {
	f := &face{body: map[string]string{
		"/v2/me/folders": largeShelf,
		"/v2/me/folders/holdings": `{"object":"list","items":[
			{"object":"folder_holding","work_id":"285","folder_ids":["11","12"]}
		],"next_cursor":null}`,
	}}
	got, err := f.uncached(t).HoldsAll(context.Background(), 7, "tok", []int64{285, 898})
	if err != nil {
		t.Fatalf("HoldsAll: %v", err)
	}
	if !got[285] || got[898] {
		t.Fatalf("want 285 held and 898 not, got %v", got)
	}
}

// The face takes 100 works and 422s above that, so a longer page has to arrive
// as more than one request.
func TestHoldsAllChunksAtTheFaceCap(t *testing.T) {
	f := &face{body: map[string]string{
		"/v2/me/folders":          largeShelf,
		"/v2/me/folders/holdings": `{"object":"list","items":[],"next_cursor":null}`,
	}}
	ids := make([]int64, 250)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	if _, err := f.uncached(t).HoldsAll(context.Background(), 7, "tok", ids); err != nil {
		t.Fatalf("HoldsAll: %v", err)
	}
	n := 0
	for _, p := range f.paths {
		if p == "/v2/me/folders/holdings" {
			n++
		}
	}
	if n != 3 {
		t.Fatalf("250 works over a cap of 100 want 3 requests, got %d", n)
	}
}

// The owner's own tab has to list every favourite, so a large shelf is walked
// for it once and then answers the hearts too.
func TestTheOwnersListWalksALargeShelfOnce(t *testing.T) {
	f := &face{body: map[string]string{
		"/v2/me/folders": largeShelf,
		"/v2/me/folders/11/items": `{"object":"list","items":[
			{"folder_id":"11","work_id":"285","created_at":"2026-09-03T00:00:00Z"}
		],"next_cursor":null}`,
	}}
	svc := f.cached(t)
	ctx := context.Background()
	for range 2 {
		ids, err := svc.WorkIDs(ctx, 7, "tok", true)
		if err != nil || len(ids) != 1 {
			t.Fatalf("WorkIDs = %v, %v", ids, err)
		}
	}
	walked := f.spent()
	if held, _ := svc.Holds(ctx, 7, "tok", 285); !held {
		t.Fatal("285 is on the shelf")
	}
	if f.spent() != walked || f.seen("/v2/me/folders/holdings") {
		t.Fatal("a heart after the full walk asked catalog again")
	}
}

// Catalog moves a folder's updated_at on every item added or removed, so the
// preview is asked again exactly when the folder changed, on whichever site.
func TestPreviewIDsAreKeptUntilTheFolderMoves(t *testing.T) {
	f := &face{body: map[string]string{
		"/v2/me/folders/11/items": `{"object":"list","items":[
			{"folder_id":"11","work_id":"285"},{"folder_id":"11","work_id":"898"}
		],"next_cursor":null}`,
	}}
	svc := f.cached(t)
	ctx := context.Background()
	for range 3 {
		ids, err := svc.PreviewWorkIDs(ctx, "tok", 11, "2026-09-24T00:00:00Z", 4)
		if err != nil || len(ids) != 2 || ids[0] != 285 {
			t.Fatalf("preview = %v, %v", ids, err)
		}
	}
	if f.spent() != 1 {
		t.Fatalf("three shelf views spent %d preview calls, want 1", f.spent())
	}
	if _, err := svc.PreviewWorkIDs(ctx, "tok", 11, "2026-09-24T00:05:00Z", 4); err != nil {
		t.Fatal(err)
	}
	if f.spent() != 2 {
		t.Fatal("a folder that moved kept its old preview")
	}
}

// Holders is the only favourite question answered off the application key, and
// it must not reach for a reader's token: the audience it is asked for includes
// people whose folders are private.
func TestHoldersReadsThePublicHoldersFace(t *testing.T) {
	f := &face{body: map[string]string{
		"/v2/folders/holders": `{"object":"list","items":[
			{"object":"folder_holder","owner_uid":"7"},
			{"object":"folder_holder","owner_uid":"121089"}
		],"next_cursor":null}`,
	}}
	gal := f.serve(t)
	got, err := Holders(context.Background(), gal, 285)
	if err != nil {
		t.Fatalf("Holders: %v", err)
	}
	if len(got) != 2 || got[0] != 7 || got[1] != 121089 {
		t.Fatalf("want [7 121089], got %v", got)
	}
	for _, p := range f.paths {
		if strings.HasPrefix(p, "/v2/me/") {
			t.Fatal("Holders must not ask a me-face")
		}
	}
}
