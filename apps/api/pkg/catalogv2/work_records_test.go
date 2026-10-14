package catalogv2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"kun-galgame-patch-api/pkg/catalogv2/catalogv2test"
)

const workRecordsJSON = `{"object":"list","items":[
	{"object":"work_record","work_id":"58",
	 "work_state":{"state":"done","completion":"all","created_at":"2026-10-01T08:00:00Z","updated_at":"2026-10-13T08:00:00Z"},
	 "playtime":{"minutes":720},"rating":` + ratingJSON + `},
	{"object":"work_record","work_id":"61",
	 "work_state":{"state":"wish","completion":null,"created_at":"2026-10-02T08:00:00Z","updated_at":"2026-10-12T08:00:00Z"},
	 "playtime":null,"rating":null}],
	"next_cursor":"cur_abc","total":7,
	"facets":{"state":[{"value":"wish","display_name":"wish","count":3},{"value":"doing","display_name":"doing","count":0},
		{"value":"done","display_name":"done","count":4},{"value":"on_hold","display_name":"on_hold","count":0},
		{"value":"dropped","display_name":"dropped","count":0}]}}`

func TestWorkRecordsReadsSomeoneElsesLibraryWithTheKey(t *testing.T) {
	var got capture
	c := ratingServer(t, 200, workRecordsJSON, &got)

	page, err := c.WorkRecords(context.Background(), 7, WorkRecordsQuery{ContentLimit: "sfw", Limit: 24, Facets: true})
	if err != nil {
		t.Fatalf("WorkRecords: %v", err)
	}
	want := "/v2/work-records?content_limit=sfw&facets=state&include_total=true&limit=24&user_id=7"
	if got.path != want || got.auth != "Bearer nmk_test_key" {
		t.Fatalf("request = %s (%s), want %s with the application key", got.path, got.auth, want)
	}
	if len(page.Items) != 2 || page.NextCursor != "cur_abc" || page.Total != 7 {
		t.Fatalf("page = %+v", page)
	}
	done, wish := page.Items[0], page.Items[1]
	if done.WorkID != 58 || done.State != "done" || done.Completion == nil || *done.Completion != "all" ||
		done.Minutes != 720 || done.Rating == nil || done.Rating.Overall != 9 {
		t.Errorf("first item = %+v", done)
	}
	if wish.WorkID != 61 || wish.State != "wish" || wish.Completion != nil || wish.Minutes != 0 || wish.Rating != nil {
		t.Errorf("second item = %+v", wish)
	}
	if page.StateCounts["wish"] != 3 || page.StateCounts["done"] != 4 || len(page.StateCounts) != 5 {
		t.Errorf("state counts = %v", page.StateCounts)
	}
}

func TestMyWorkRecordsReadsTheOwnersListWithTheirToken(t *testing.T) {
	var got capture
	c := ratingServer(t, 200, `{"object":"list","items":[],"next_cursor":null}`, &got)

	page, err := c.MyWorkRecords(context.Background(), "user_tok", WorkRecordsQuery{States: []string{"done"}, Cursor: "cur_abc", Limit: 24})
	if err != nil {
		t.Fatalf("MyWorkRecords: %v", err)
	}
	if got.path != "/v2/me/work-records?cursor=cur_abc&limit=24&state=done" || got.auth != "Bearer user_tok" {
		t.Fatalf("request = %s (%s)", got.path, got.auth)
	}
	if len(page.Items) != 0 || page.NextCursor != "" || page.StateCounts != nil {
		t.Errorf("page = %+v", page)
	}
	if _, err := c.MyWorkRecords(context.Background(), "", WorkRecordsQuery{}); err != ErrNoAccessToken {
		t.Errorf("no token: err = %v, want ErrNoAccessToken", err)
	}
}

func TestAPrivateLibraryIsTellableFromOtherRefusals(t *testing.T) {
	for _, tc := range []struct {
		code string
		want bool
	}{
		{CodePlayRecordsPrivate, true},
		{CodeScopeRequired, false},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			catalogv2test.Problem(w, r, tc.code, "refused", nil)
		}))
		_, err := New(srv.URL, "nmk_test_key").WorkRecords(context.Background(), 7, WorkRecordsQuery{})
		srv.Close()
		if err == nil || IsPlayRecordsPrivate(err) != tc.want {
			t.Errorf("%s: err = %v, IsPlayRecordsPrivate = %v, want %v", tc.code, err, IsPlayRecordsPrivate(err), tc.want)
		}
	}
}

func TestSetPlayRecordsPrivatePatchesTheSwitch(t *testing.T) {
	var got capture
	c := ratingServer(t, 200, `{"object":"privacy","play_records":"private","updated_at":"2026-10-14T08:00:00Z"}`, &got)

	private, err := c.SetPlayRecordsPrivate(context.Background(), "user_tok", true)
	if err != nil || !private {
		t.Fatalf("SetPlayRecordsPrivate = %v, %v", private, err)
	}
	if got.method != http.MethodPatch || got.path != "/v2/me/privacy" || got.auth != "Bearer user_tok" ||
		string(got.body["play_records"]) != `"private"` || len(got.body) != 1 {
		t.Errorf("request = %s %s (%s) %v", got.method, got.path, got.auth, got.body)
	}
}
