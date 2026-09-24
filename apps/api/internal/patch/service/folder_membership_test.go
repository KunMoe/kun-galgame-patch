package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/pkg/catalogv2"
	"kun-galgame-patch-api/pkg/catalogv2/catalogv2test"
)

type shelfAnswer func(w http.ResponseWriter, r *http.Request)

type shelf struct {
	mu      sync.Mutex
	answers map[string][]shelfAnswer
	seen    []string
	bodies  map[string]map[string]any
	keys    map[string]string
}

func (s *shelf) client(t *testing.T) *PatchService {
	t.Helper()
	s.bodies, s.keys = map[string]map[string]any{}, map[string]string{}
	count := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := r.Method + " " + r.URL.Path
		s.mu.Lock()
		s.seen = append(s.seen, route)
		n := count[route]
		count[route]++
		if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
			var body map[string]any
			_ = json.Unmarshal(raw, &body)
			s.bodies[route] = body
		}
		s.keys[route] = r.Header.Get("Idempotency-Key")
		answers := s.answers[route]
		s.mu.Unlock()
		if len(answers) == 0 {
			catalogv2test.Problem(w, r, "NOT_FOUND", "Nothing visible exists at this URL.", nil)
			return
		}
		answers[min(n, len(answers)-1)](w, r)
	}))
	t.Cleanup(srv.Close)
	return &PatchService{galgame: galgameClient.NewWithKey(srv.URL, "nmk_test_key")}
}

func (s *shelf) count(route string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, r := range s.seen {
		if r == route {
			n++
		}
	}
	return n
}

func body(s string) shelfAnswer {
	return func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, s) }
}

func problem(code string) shelfAnswer {
	return func(w http.ResponseWriter, r *http.Request) { catalogv2test.Problem(w, r, code, "detail", nil) }
}

func noContent(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }

const (
	emptyShelf   = `{"object":"list","items":[]}`
	plainFolder  = `{"object":"list","items":[{"id":"5","name":"二周目","is_default":false}]}`
	defaultShelf = `{"object":"list","items":[{"id":"9","name":"默认收藏夹","is_default":true}]}`
)

// The picker used to create the default folder on its GET.
func TestThePickerNeverWrites(t *testing.T) {
	s := &shelf{answers: map[string][]shelfAnswer{
		"GET /v2/me/folders": {body(emptyShelf)},
	}}
	out, err := s.client(t).FoldersForPatch(context.Background(), "tok", 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("folders = %+v", out)
	}
	for _, route := range s.seen {
		if route[:4] == "POST" {
			t.Fatalf("a read made %s", route)
		}
	}
}

func TestTheDefaultFolderIsNamedAndKeyedOnTheShelfItSaw(t *testing.T) {
	s := &shelf{answers: map[string][]shelfAnswer{
		"GET /v2/me/folders":  {body(plainFolder)},
		"POST /v2/me/folders": {body(`{"id":"11","name":"默认收藏夹","is_default":true}`)},
	}}
	id, err := s.client(t).defaultFolderID(context.Background(), "tok", 42)
	if err != nil || id != 11 {
		t.Fatalf("defaultFolderID = %d, %v", id, err)
	}
	sent := s.bodies["POST /v2/me/folders"]
	if sent["name"] == "" || sent["is_default"] != true {
		t.Fatalf("catalog refuses a blank name with 422: %v", sent)
	}
	if s.keys["POST /v2/me/folders"] == "" {
		t.Fatal("the create carried no Idempotency-Key")
	}
}

// The losing heart is told IDEMPOTENCY_REQUEST_IN_PROGRESS while the winner's
// create runs (infra protocol/idempotency.go); the same key sent again once it
// has finished is answered with the winner's folder.
func TestARacingDefaultCreateReusesTheWinner(t *testing.T) {
	defaultFolderRetryDelay = 0
	s := &shelf{answers: map[string][]shelfAnswer{
		"GET /v2/me/folders": {body(emptyShelf)},
		"POST /v2/me/folders": {
			problem("IDEMPOTENCY_REQUEST_IN_PROGRESS"),
			body(`{"id":"9","name":"默认收藏夹","is_default":true}`),
		},
	}}
	svc := s.client(t)
	id, err := svc.defaultFolderID(context.Background(), "tok", 42)
	if err != nil || id != 9 {
		t.Fatalf("defaultFolderID = %d, %v; want the folder the winner made", id, err)
	}
	if s.count("POST /v2/me/folders") != 2 {
		t.Fatalf("posts = %v", s.seen)
	}
}

func TestTwoHeartsOnOneShelfShareAKey(t *testing.T) {
	seen := []catalogv2.Folder{{ID: 5}}
	if defaultFolderKey(42, seen) != defaultFolderKey(42, seen) {
		t.Fatal("two hearts on the same shelf must send one create between them")
	}
	if defaultFolderKey(42, seen) == defaultFolderKey(42, nil) {
		t.Fatal("a shelf that changed is a new create, not a replay of the old one")
	}
	if defaultFolderKey(42, seen) == defaultFolderKey(43, seen) {
		t.Fatal("two readers must never share a key")
	}
}

func TestMoveFolderItemsKeepsWhatLanded(t *testing.T) {
	for _, tc := range []struct {
		name     string
		answers  map[string][]shelfAnswer
		held     map[int64]bool
		want     map[int64]bool
		wantHeld map[int64]bool
		wantErr  bool
	}{
		{
			name: "a refused PUT leaves the shelf as it was",
			answers: map[string][]shelfAnswer{
				"PUT /v2/me/folders/2/items/7": {problem("SERVICE_UNAVAILABLE")},
			},
			held: map[int64]bool{1: true}, want: map[int64]bool{2: true},
			wantHeld: map[int64]bool{1: true}, wantErr: true,
		},
		{
			name: "a PUT that landed before a refused DELETE is counted",
			answers: map[string][]shelfAnswer{
				"PUT /v2/me/folders/2/items/7":    {noContent},
				"DELETE /v2/me/folders/1/items/7": {problem("RATE_LIMITED")},
			},
			held: map[int64]bool{1: true}, want: map[int64]bool{2: true},
			wantHeld: map[int64]bool{1: true, 2: true}, wantErr: true,
		},
		{
			name: "a move that went through",
			answers: map[string][]shelfAnswer{
				"PUT /v2/me/folders/2/items/7":    {noContent},
				"DELETE /v2/me/folders/1/items/7": {noContent},
			},
			held: map[int64]bool{1: true}, want: map[int64]bool{2: true},
			wantHeld: map[int64]bool{2: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &shelf{answers: tc.answers}
			err := s.client(t).moveFolderItems(context.Background(), "tok", 7, tc.held, tc.want)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v", err)
			}
			if len(tc.held) != len(tc.wantHeld) {
				t.Fatalf("held = %v, want %v", tc.held, tc.wantHeld)
			}
			for id := range tc.wantHeld {
				if !tc.held[id] {
					t.Fatalf("held = %v, want %v", tc.held, tc.wantHeld)
				}
			}
		})
	}
}
