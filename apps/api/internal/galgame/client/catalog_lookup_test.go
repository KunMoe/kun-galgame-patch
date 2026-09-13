package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
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

func v2Work(id int64, gid int) string {
	g := strconv.Itoa(gid)
	return `{"object":"work","id":"` + strconv.FormatInt(id, 10) + `","refs":[` +
		`{"source":"galgame_wiki","external_id":"` + g + `"},` +
		`{"source":"curated","external_id":"` + g + `"}],` +
		`"claim":{"site":"galgame_wiki","site_work_id":"` + g + `","state":"live","content_limit":"sfw"}}`
}

func TestClaimStatesDecodeBranches(t *testing.T) {
	body := v2List(
		`{"object":"work","id":"900","claim":{"site":"galgame_wiki","site_work_id":"7","state":"live","content_limit":"sfw"}}`,
	)
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
	s := &scripted{status: 200, body: v2List(`{"id":"31","display_name":"Brand"}`)}
	id, found, err := s.client(t).ResolveWikiLabel(context.Background(), 31)
	if err != nil || !found || id != 31 {
		t.Fatalf("ResolveWikiLabel = (%d, %v, %v), want (31, true, nil)", id, found, err)
	}
}

func TestCheckGalgameByVndbIDDecodeBranches(t *testing.T) {
	s := &scripted{status: 200, body: v2List(
		`{"object":"work","id":"900","claim":{"site":"galgame_wiki","site_work_id":"7","state":"live","content_limit":"sfw"}}`,
	)}
	exists, gid, err := s.client(t).CheckGalgameByVndbID(context.Background(), "v1")
	if err != nil || !exists || gid != 7 {
		t.Fatalf("CheckGalgameByVndbID = (%v, %d, %v), want (true, 7, nil)", exists, gid, err)
	}
}

func TestCatalogAbsentRequiresTheCatalogsOwnEnvelope(t *testing.T) {
	if !catalogAbsent(&GalgameError{Code: catalogCodeNotFound, HTTPStatus: 404}) {
		t.Fatal("404 + catalog not-found must be absence")
	}
	if catalogAbsent(&GalgameError{Code: 5, HTTPStatus: 500}) {
		t.Fatal("a 500 is never absence")
	}
}
