package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"kun-galgame-patch-api/pkg/catalogv2/catalogv2test"
)

type scripted struct {
	status int
	body   string
	mu     sync.Mutex
	calls  int
}

func (s *scripted) client(t *testing.T) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		s.calls++
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(s.status)
		_, _ = w.Write([]byte(s.body))
	}))
	t.Cleanup(srv.Close)
	return NewWithKey(srv.URL, "nm_test_key")
}

func (s *scripted) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func v2List(items string) string {
	if items == "" {
		return `{"object":"list","items":[]}`
	}
	return `{"object":"list","items":[` + items + `]}`
}

// A basic-view work as handler/map_work.go answers it, with the claim swapped
// in: nothing but the claim differs between the cases these tests draw.
func v2Work(id, claim string) string {
	return `{"object":"work","id":"` + id + `","medium":"galgame","display_name":"W","latin":null,"localized":{},` +
		`"olang":"ja","content_rating":"all_ages","content_limit":"sfw","release_date":null,` +
		`"release_date_precision":null,"release_status":"unknown","cover":null,"banner":null,` +
		`"claim":` + claim + `,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`
}

func TestClaimStatesDecodeBranches(t *testing.T) {
	body := v2List(v2Work("900", `{"site":"kungal","site_work_id":"7","state":"live","content_limit":"sfw"}`))
	s := &scripted{status: 200, body: body}
	got, err := s.client(t).ClaimStates(context.Background(), []int{900})
	if err != nil {
		t.Fatalf("ClaimStates: %v", err)
	}
	// Keyed by the catalog id, which is the page id. The claim's site_work_id
	// is the forum's number and keying on it filed the state under a different
	// game here.
	if got[900] != catalogClaimStateLive {
		t.Fatalf("ClaimStates = %v, want live for 900", got)
	}
}

func TestResolveWikiLabelDecodeBranches(t *testing.T) {
	s := &scripted{status: 200, body: v2List(`{"object":"company","id":"31","display_name":"Brand","latin":null,"lang":"ja",` +
		`"localized":{},"company_kind":"game_brand","work_count":3}`)}
	id, found, err := s.client(t).ResolveWikiLabel(context.Background(), 31)
	if err != nil || !found || id != 31 {
		t.Fatalf("ResolveWikiLabel = (%d, %v, %v), want (31, true, nil)", id, found, err)
	}
}

func TestCheckGalgameByVndbIDAnswersThePageID(t *testing.T) {
	cases := []struct {
		name, work string
		exists     bool
		gid        int
	}{
		{"claimed by the forum", v2Work("900", `{"site":"kungal","site_work_id":"7","state":"live","content_limit":"sfw"}`), true, 900},
		{"unclaimed", v2Work("901", "null"), true, 901},
		{"hidden", v2Work("902", `{"site":"kungal","site_work_id":"8","state":"hidden","content_limit":"sfw"}`), false, 0},
	}
	for _, tc := range cases {
		s := &scripted{status: 200, body: v2List(tc.work)}
		exists, gid, err := s.client(t).CheckGalgameByVndbID(context.Background(), "v1")
		if err != nil || exists != tc.exists || gid != tc.gid {
			t.Errorf("%s: CheckGalgameByVndbID = (%v, %d, %v), want (%v, %d, nil)", tc.name, exists, gid, err, tc.exists, tc.gid)
		}
	}
}

func TestAbsenceRequiresTheCatalogsOwnProblem(t *testing.T) {
	for _, tc := range []struct {
		name   string
		answer func(w http.ResponseWriter, r *http.Request)
		absent bool
	}{
		{"catalog's own NOT_FOUND", func(w http.ResponseWriter, r *http.Request) {
			catalogv2test.Problem(w, r, "NOT_FOUND", "No work with this id.", nil)
		}, true},
		{"a proxy's 404 page", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("404 page not found"))
		}, false},
		{"catalog falling over", func(w http.ResponseWriter, r *http.Request) {
			catalogv2test.Problem(w, r, "INTERNAL_ERROR", "panic recovered", nil)
		}, false},
		{"a merge", func(w http.ResponseWriter, r *http.Request) {
			catalogv2test.Problem(w, r, "ENTITY_MERGED", "merged", map[string]any{"object": "work", "current_id": "9"})
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(tc.answer))
			t.Cleanup(srv.Close)
			_, err := NewWithKey(srv.URL, "nmk_test_key").GetGalgame(context.Background(), 1, "")
			if err == nil || IsAbsent(err) != tc.absent {
				t.Fatalf("IsAbsent = %v, want %v: %v", IsAbsent(err), tc.absent, err)
			}
		})
	}
}
