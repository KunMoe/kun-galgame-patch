package rating

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/catalogv2"
	"kun-galgame-patch-api/pkg/catalogv2/catalogv2test"
)

type libraryRead struct {
	token  string
	userID int
	q      catalogv2.WorkRecordsQuery
}

type fakeLibraryCatalog struct {
	page       catalogv2.WorkRecordPage
	counts     catalogv2.WorkRecordPage
	publicErr  error
	private    bool
	publicRead []libraryRead
	ownRead    []libraryRead
}

func (f *fakeLibraryCatalog) answer(q catalogv2.WorkRecordsQuery) catalogv2.WorkRecordPage {
	if q.Limit == 1 {
		return f.counts
	}
	return f.page
}

func (f *fakeLibraryCatalog) WorkRecords(_ context.Context, userID int, q catalogv2.WorkRecordsQuery) (catalogv2.WorkRecordPage, error) {
	f.publicRead = append(f.publicRead, libraryRead{userID: userID, q: q})
	if f.publicErr != nil {
		return catalogv2.WorkRecordPage{}, f.publicErr
	}
	return f.answer(q), nil
}

func (f *fakeLibraryCatalog) MyWorkRecords(_ context.Context, token string, q catalogv2.WorkRecordsQuery) (catalogv2.WorkRecordPage, error) {
	f.ownRead = append(f.ownRead, libraryRead{token: token, q: q})
	return f.answer(q), nil
}

func (f *fakeLibraryCatalog) PlayRecordsPrivate(context.Context, string) (bool, error) {
	return f.private, nil
}

func (f *fakeLibraryCatalog) SetPlayRecordsPrivate(_ context.Context, _ string, private bool) (bool, error) {
	f.private = private
	return private, nil
}

type fakeLibraryWorks struct {
	known  []int
	asked  [][]int
	limits []string
}

func (f *fakeLibraryWorks) GalgameCardBatch(_ context.Context, ids []int, contentLimit string) ([]galgameClient.GalgameBrief, error) {
	f.asked = append(f.asked, ids)
	f.limits = append(f.limits, contentLimit)
	var out []galgameClient.GalgameBrief
	for _, id := range ids {
		if slices.Contains(f.known, id) {
			out = append(out, galgameClient.GalgameBrief{ID: id, DisplayName: "work"})
		}
	}
	return out, nil
}

type fakePatchRows map[int]patchModel.Patch

func (f fakePatchRows) PatchRows([]int) (map[int]patchModel.Patch, error) { return f, nil }

func privateRefusal(t *testing.T) error {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		catalogv2test.Problem(w, r, catalogv2.CodePlayRecordsPrivate, "this account keeps its play records private.", nil)
	}))
	defer srv.Close()
	_, err := catalogv2.New(srv.URL, "nmk_test_key").WorkRecords(context.Background(), 7, catalogv2.WorkRecordsQuery{})
	if !catalogv2.IsPlayRecordsPrivate(err) {
		t.Fatalf("refusal = %v", err)
	}
	return err
}

func strPtr(s string) *string { return &s }

func libraryPage() catalogv2.WorkRecordPage {
	return catalogv2.WorkRecordPage{
		Items: []catalogv2.ListedWorkRecord{
			{WorkID: 58, WorkRecord: catalogv2.WorkRecord{State: "done", Completion: strPtr("all"), Minutes: 720,
				Rating: &catalogv2.UserRating{ID: 501, WorkID: 58, Overall: 9}}},
			{WorkID: 404, WorkRecord: catalogv2.WorkRecord{State: "doing"}},
			{WorkID: 61, WorkRecord: catalogv2.WorkRecord{State: "wish", Minutes: 5}},
		},
		NextCursor:  "cur_abc",
		Total:       7,
		StateCounts: map[string]int{"wish": 3, "doing": 0, "done": 4, "on_hold": 0, "dropped": 0},
	}
}

func TestAVisitorReadsAPublicLibraryWithTheKey(t *testing.T) {
	cat := &fakeLibraryCatalog{page: libraryPage()}
	works := &fakeLibraryWorks{known: []int{58, 61}}
	lib := NewLibrary(cat, works, fakePatchRows{58: {ID: 58, ResourceCount: 3}})

	page, err := lib.Page(context.Background(), LibraryQuery{UserID: 7, ViewerID: 9, AccessToken: "tok", ContentLimit: "sfw"})
	if err != nil {
		t.Fatalf("Page: %v", err)
	}
	if len(cat.ownRead) != 0 || len(cat.publicRead) != 1 {
		t.Fatalf("reads: public %v, own %v; want one read with the key", cat.publicRead, cat.ownRead)
	}
	r := cat.publicRead[0]
	if r.userID != 7 || r.q.ContentLimit != "sfw" || !r.q.Facets || r.q.Limit != libraryPageSize || len(r.q.States) != 0 {
		t.Errorf("read = %+v", r)
	}
	if page.Visibility != VisibilityPublic || page.NextCursor != "cur_abc" || page.Total != 7 || page.Counts["done"] != 4 {
		t.Errorf("page = %+v", page)
	}
	if works.limits[0] != "sfw" {
		t.Errorf("hydrated under %q, want the reader's sfw", works.limits[0])
	}
	if len(page.Items) != 2 {
		t.Fatalf("items = %+v, want 58 and 61 (404 has no card)", page.Items)
	}
	done, wish := page.Items[0], page.Items[1]
	if done.ID != 58 || done.PlayState != DoneAll || done.Minutes != 720 ||
		done.Overall == nil || *done.Overall != 9 || done.Count.Resource != 3 {
		t.Errorf("first item = %+v", done)
	}
	if wish.ID != 61 || wish.PlayState != Wish || wish.Minutes != 0 || wish.Overall != nil {
		t.Errorf("second item = %+v; a report under the floor reads as no playtime", wish)
	}
}

func TestAVisitorIsToldAPrivateLibraryIsPrivate(t *testing.T) {
	cat := &fakeLibraryCatalog{publicErr: privateRefusal(t)}
	works := &fakeLibraryWorks{}
	lib := NewLibrary(cat, works, fakePatchRows{})

	for _, viewer := range []int{0, 9} {
		page, err := lib.Page(context.Background(), LibraryQuery{UserID: 7, ViewerID: viewer})
		if err != nil {
			t.Fatalf("viewer %d: %v", viewer, err)
		}
		if page.Visibility != VisibilityHidden || len(page.Items) != 0 || page.Items == nil {
			t.Errorf("viewer %d: page = %+v", viewer, page)
		}
	}
	if len(cat.ownRead) != 0 || len(works.asked) != 0 {
		t.Errorf("own reads %v, hydrates %v; want neither", cat.ownRead, works.asked)
	}
}

func TestTheOwnerReadsTheirOwnListWhateverTheSwitch(t *testing.T) {
	for _, private := range []bool{false, true} {
		cat := &fakeLibraryCatalog{page: libraryPage(), private: private, publicErr: privateRefusal(t)}
		lib := NewLibrary(cat, &fakeLibraryWorks{known: []int{58}}, fakePatchRows{})

		page, err := lib.Page(context.Background(), LibraryQuery{UserID: 7, ViewerID: 7, AccessToken: "tok"})
		if err != nil {
			t.Fatalf("private=%v: %v", private, err)
		}
		if len(cat.publicRead) != 0 || len(cat.ownRead) != 1 || cat.ownRead[0].token != "tok" {
			t.Fatalf("private=%v: public %v, own %v; want one read with the owner's token", private, cat.publicRead, cat.ownRead)
		}
		want := VisibilityPublic
		if private {
			want = VisibilityOwnerOnly
		}
		if page.Visibility != want || len(page.Items) != 1 {
			t.Errorf("private=%v: page = %+v, want visibility %s", private, page, want)
		}
	}
}

// facets=state counts after state=, so the tab row would read 0 everywhere
// but the open tab.
func TestAStateTabTakesTheTabCountsFromAnUnfilteredRead(t *testing.T) {
	cat := &fakeLibraryCatalog{
		page:   catalogv2.WorkRecordPage{Total: 4, StateCounts: map[string]int{"done": 4}},
		counts: catalogv2.WorkRecordPage{Total: 7, StateCounts: map[string]int{"wish": 3, "done": 4}},
	}
	lib := NewLibrary(cat, &fakeLibraryWorks{}, fakePatchRows{})

	page, err := lib.Page(context.Background(), LibraryQuery{UserID: 7, State: "done", ContentLimit: "sfw"})
	if err != nil {
		t.Fatalf("Page: %v", err)
	}
	if len(cat.publicRead) != 2 {
		t.Fatalf("reads = %+v, want the list and the counts", cat.publicRead)
	}
	list, counts := cat.publicRead[0].q, cat.publicRead[1].q
	if !slices.Equal(list.States, []string{"done"}) || list.Facets {
		t.Errorf("list read = %+v", list)
	}
	if len(counts.States) != 0 || !counts.Facets || counts.Limit != 1 || counts.ContentLimit != "sfw" {
		t.Errorf("counts read = %+v", counts)
	}
	if page.Total != 7 || page.Counts["wish"] != 3 {
		t.Errorf("page = %+v, want the unfiltered counts", page)
	}

	cat.publicRead = nil
	if _, err := lib.Page(context.Background(), LibraryQuery{UserID: 7, State: "done", Cursor: "cur_abc"}); err != nil {
		t.Fatalf("next page: %v", err)
	}
	if len(cat.publicRead) != 1 || cat.publicRead[0].q.Facets || cat.publicRead[0].q.Cursor != "cur_abc" {
		t.Errorf("next page reads = %+v, want one read without counts", cat.publicRead)
	}
}
