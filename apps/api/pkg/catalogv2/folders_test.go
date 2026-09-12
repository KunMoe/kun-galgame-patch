package catalogv2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

type folderFace struct {
	mu    sync.Mutex
	hits  []folderCall
	pages map[string][]string
}

type folderCall struct {
	Method string
	Path   string
	Query  string
	Auth   string
}

func (f *folderFace) server(t *testing.T) *httptest.Server {
	t.Helper()
	seen := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.hits = append(f.hits, folderCall{
			Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery,
			Auth: r.Header.Get("Authorization"),
		})
		n := seen[r.URL.Path]
		seen[r.URL.Path]++
		f.mu.Unlock()

		bodies := f.pages[r.URL.Path]
		if len(bodies) == 0 {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":"NOT_FOUND","status":404,"detail":"no"}`))
			return
		}
		if n >= len(bodies) {
			n = len(bodies) - 1
		}
		_, _ = w.Write([]byte(bodies[n]))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (f *folderFace) calls() []folderCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]folderCall, len(f.hits))
	copy(out, f.hits)
	return out
}

func folderListBody(next string, ids ...int64) string {
	items := make([]string, 0, len(ids))
	for _, id := range ids {
		items = append(items, `{"object":"folder","id":"`+strconv.FormatInt(id, 10)+
			`","owner_uid":"7","name":"f","description":"","visibility":"public",`+
			`"is_default":false,"item_count":1,"created_at":"2026-01-01T00:00:00Z",`+
			`"updated_at":"2026-01-02T00:00:00Z"}`)
	}
	cursor := "null"
	if next != "" {
		cursor = `"` + next + `"`
	}
	return `{"object":"list","items":[` + strings.Join(items, ",") + `],"next_cursor":` + cursor + `}`
}

func itemListBody(next string, workIDs ...int64) string {
	items := make([]string, 0, len(workIDs))
	for _, id := range workIDs {
		items = append(items, `{"object":"folder_item","folder_id":"5","work_id":"`+
			strconv.FormatInt(id, 10)+`","created_at":"2026-01-01T00:00:00Z",`+
			`"updated_at":"2026-01-01T00:00:00Z"}`)
	}
	cursor := "null"
	if next != "" {
		cursor = `"` + next + `"`
	}
	return `{"object":"list","items":[` + strings.Join(items, ",") + `],"next_cursor":` + cursor + `}`
}

// The two lanes take different credentials and that is the whole point: a
// public folder is public to signed-out readers, while folder:read is consent
// to read the bearer's OWN folders and says nothing about anybody else's.
// Sending the wrong one is the shape of the cover-vote outage — a silent 403.
func TestFolderLanesUseTheRightCredential(t *testing.T) {
	face := &folderFace{pages: map[string][]string{
		"/v2/me/folders":      {folderListBody("", 11)},
		"/v2/folders":         {folderListBody("", 12)},
		"/v2/folders/5/items": {itemListBody("", 900)},
	}}
	srv := face.server(t)
	c := New(srv.URL, "nmk_live_key")
	ctx := context.Background()

	if _, err := c.MyFolders(ctx, "user-jwt"); err != nil {
		t.Fatalf("MyFolders: %v", err)
	}
	if _, err := c.PublicFolders(ctx, 7); err != nil {
		t.Fatalf("PublicFolders: %v", err)
	}
	if _, err := c.PublicFolderItems(ctx, 5); err != nil {
		t.Fatalf("PublicFolderItems: %v", err)
	}

	for _, call := range face.calls() {
		mine := strings.HasPrefix(call.Path, "/v2/me/")
		switch {
		case mine && call.Auth != "Bearer user-jwt":
			t.Errorf("%s sent %q, want the user token", call.Path, call.Auth)
		case !mine && !strings.Contains(call.Auth, "nmk_live_key"):
			t.Errorf("%s sent %q, want the application key", call.Path, call.Auth)
		}
	}
}

func TestPublicFolderListCarriesTheOwner(t *testing.T) {
	face := &folderFace{pages: map[string][]string{"/v2/folders": {folderListBody("", 11)}}}
	srv := face.server(t)
	c := New(srv.URL, "k")

	if _, err := c.PublicFolders(context.Background(), 4242); err != nil {
		t.Fatalf("PublicFolders: %v", err)
	}
	if q := face.calls()[0].Query; !strings.Contains(q, "owner_uid=4242") {
		t.Fatalf("query %q carries no owner_uid — upstream answers 400 without it", q)
	}
}

func TestFoldersHoldingSendsTheFilter(t *testing.T) {
	face := &folderFace{pages: map[string][]string{"/v2/me/folders": {folderListBody("", 11)}}}
	srv := face.server(t)
	c := New(srv.URL, "k")

	got, err := c.MyFoldersHolding(context.Background(), "tok", 900)
	if err != nil {
		t.Fatalf("MyFoldersHolding: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d folders", len(got))
	}
	if q := face.calls()[0].Query; !strings.Contains(q, "contains_work_id=900") {
		t.Fatalf("query %q carries no contains_work_id — the picker would read every folder", q)
	}
}

// The whole folder is read: the site orders and pages locally because the
// catalog's keyset is a sync watermark, so a walk that stopped early would
// silently drop the tail of a big folder.
func TestFolderWalkFollowsEveryCursor(t *testing.T) {
	face := &folderFace{pages: map[string][]string{
		"/v2/me/folders/5/items": {
			itemListBody("cur_a", 901, 902),
			itemListBody("cur_b", 903),
			itemListBody("", 904),
		},
	}}
	srv := face.server(t)
	c := New(srv.URL, "k")

	items, err := c.MyFolderItems(context.Background(), "tok", 5)
	if err != nil {
		t.Fatalf("MyFolderItems: %v", err)
	}
	if len(items) != 4 {
		t.Fatalf("walked %d items, want 4", len(items))
	}
	calls := face.calls()
	if len(calls) != 3 {
		t.Fatalf("made %d requests, want 3", len(calls))
	}
	if strings.Contains(calls[0].Query, "cursor=") {
		t.Errorf("first page sent a cursor: %q", calls[0].Query)
	}
	if !strings.Contains(calls[1].Query, "cursor=cur_a") || !strings.Contains(calls[2].Query, "cursor=cur_b") {
		t.Errorf("cursors not threaded: %q then %q", calls[1].Query, calls[2].Query)
	}
	if items[0].WorkID != 901 || items[3].WorkID != 904 {
		t.Errorf("decoded work ids wrong: %+v", items)
	}
}

func TestFolderWritesAddressTheRightRoutes(t *testing.T) {
	body := `{"object":"folder","id":"5","owner_uid":"7","name":"n","description":"","visibility":"private","is_default":false,"item_count":0,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`
	face := &folderFace{pages: map[string][]string{
		"/v2/me/folders":             {body},
		"/v2/me/folders/5":           {body},
		"/v2/me/folders/5/items/900": {`{}`},
	}}
	srv := face.server(t)
	c := New(srv.URL, "k")
	ctx := context.Background()
	name := "n"

	if _, err := c.CreateFolder(ctx, "tok", FolderWrite{Name: &name}); err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	if _, err := c.PatchFolder(ctx, "tok", 5, FolderWrite{Name: &name}); err != nil {
		t.Fatalf("PatchFolder: %v", err)
	}
	if err := c.PutFolderItem(ctx, "tok", 5, 900); err != nil {
		t.Fatalf("PutFolderItem: %v", err)
	}
	if err := c.DeleteFolderItem(ctx, "tok", 5, 900); err != nil {
		t.Fatalf("DeleteFolderItem: %v", err)
	}
	if err := c.DeleteFolder(ctx, "tok", 5); err != nil {
		t.Fatalf("DeleteFolder: %v", err)
	}

	want := []struct{ method, path string }{
		{"POST", "/v2/me/folders"},
		{"PATCH", "/v2/me/folders/5"},
		{"PUT", "/v2/me/folders/5/items/900"},
		{"DELETE", "/v2/me/folders/5/items/900"},
		{"DELETE", "/v2/me/folders/5"},
	}
	calls := face.calls()
	if len(calls) != len(want) {
		t.Fatalf("made %d calls, want %d: %+v", len(calls), len(want), calls)
	}
	for i, w := range want {
		if calls[i].Method != w.method || calls[i].Path != w.path {
			t.Errorf("call %d was %s %s, want %s %s", i, calls[i].Method, calls[i].Path, w.method, w.path)
		}
	}
}

// A grant minted before folder:write was requested comes back 403
// SCOPE_REQUIRED. The only cure is signing in again, and the handler maps it to
// the re-login this site already had for catalog:edit, so it has to survive as
// a Problem carrying the code.
func TestFolderWriteSurfacesTheScopeRefusal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":"SCOPE_REQUIRED","status":403,` +
			`"detail":"this operation requires the folder:write scope."}`))
	}))
	t.Cleanup(srv.Close)
	c := New(srv.URL, "k")

	name := "x"
	_, err := c.CreateFolder(context.Background(), "tok", FolderWrite{Name: &name})
	if err == nil {
		t.Fatal("a scope refusal returned no error")
	}
	p, ok := err.(*Problem)
	if !ok {
		t.Fatalf("got %T (%v), want *Problem", err, err)
	}
	if p.Code != "SCOPE_REQUIRED" {
		t.Fatalf("code %q, want SCOPE_REQUIRED", p.Code)
	}
}

func TestFolderCreateOmitsUnsetFields(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 512)
		n, _ := r.Body.Read(buf)
		seen = string(buf[:n])
		_, _ = w.Write([]byte(`{"object":"folder","id":"5","owner_uid":"7","name":"n","description":"","visibility":"private","is_default":false,"item_count":0,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`))
	}))
	t.Cleanup(srv.Close)
	c := New(srv.URL, "k")

	name := "n"
	if _, err := c.CreateFolder(context.Background(), "tok", FolderWrite{Name: &name}); err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	if strings.Contains(seen, "visibility") {
		t.Errorf("an unset field was sent anyway: %s", seen)
	}
	if !strings.Contains(seen, `"name":"n"`) {
		t.Errorf("name did not travel: %s", seen)
	}
}

// A preview is one page and stops there. The full walk exists next to it and
// follows next_cursor to the end; a shelf card drawing four covers must not,
// or a person with a 400-item folder pays four requests for a thumbnail.
func TestFolderPreviewItemsTakeOnePageOnTheRightLane(t *testing.T) {
	face := &folderFace{pages: map[string][]string{
		"/v2/me/folders/5/items": {itemListBody("cur_2", 900, 901)},
		"/v2/folders/6/items":    {itemListBody("cur_2", 902)},
	}}
	srv := face.server(t)
	c := New(srv.URL, "nmk_live_key")
	ctx := context.Background()

	mine, err := c.FolderPreviewItems(ctx, "user-jwt", 5, 4)
	if err != nil {
		t.Fatalf("FolderPreviewItems(own): %v", err)
	}
	if len(mine) != 2 {
		t.Fatalf("want the one page's 2 items, got %d", len(mine))
	}
	if _, err = c.FolderPreviewItems(ctx, "", 6, 4); err != nil {
		t.Fatalf("FolderPreviewItems(public): %v", err)
	}

	calls := face.calls()
	if len(calls) != 2 {
		t.Fatalf("want one request each, got %d: %v", len(calls), calls)
	}
	for _, call := range calls {
		if !strings.Contains(call.Query, "limit=4") {
			t.Errorf("%s asked %q, want limit=4", call.Path, call.Query)
		}
		if strings.Contains(call.Query, "cursor=") {
			t.Errorf("%s followed the cursor: %q", call.Path, call.Query)
		}
	}
	if calls[0].Auth != "Bearer user-jwt" {
		t.Errorf("own lane sent %q, want the user token", calls[0].Auth)
	}
	if !strings.Contains(calls[1].Auth, "nmk_live_key") {
		t.Errorf("public lane sent %q, want the application key", calls[1].Auth)
	}
}
