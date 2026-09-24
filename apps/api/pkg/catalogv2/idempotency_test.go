package catalogv2_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"kun-galgame-patch-api/pkg/catalogv2"
)

type keyed struct {
	mu   sync.Mutex
	keys map[string][]string
}

func (k *keyed) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		k.mu.Lock()
		route := r.Method + " " + r.URL.Path
		k.keys[route] = append(k.keys[route], r.Header.Get("Idempotency-Key"))
		k.mu.Unlock()
		switch r.URL.Path {
		case "/v2/me/folders":
			_, _ = io.WriteString(w, `{"id":"5","name":"n"}`)
		case "/v2/me/proposals":
			_, _ = io.WriteString(w, `{"id":"32","state":"open"}`)
		default:
			_, _ = io.WriteString(w, `{"object":"claim","id":"7","state":"draft"}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// Catalog replays its first answer to a repeated key for 24h, so every create
// carries the key its caller names for this attempt, and none when it names
// none: a key made up here from the body replays a deleted record to a reader
// who deliberately makes the same thing again.
func TestEveryCreateSendsTheCallersKey(t *testing.T) {
	k := &keyed{keys: map[string][]string{}}
	c := catalogv2.New(k.server(t).URL, "k")
	ctx := context.Background()
	name := "n"

	for _, key := range []string{"moyu-attempt", ""} {
		if _, err := c.MintClaim(ctx, "tok", key, map[string]any{"catalog.work.olang": "ja"}); err != nil {
			t.Fatal(err)
		}
		if _, err := c.CreateClaim(ctx, "tok", key, 7, 7); err != nil {
			t.Fatal(err)
		}
		if _, err := c.CreateProposal(ctx, "tok", key, catalogv2.EntityTypeWork, 7,
			map[string]any{"catalog.work.olang": "ja"}, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := c.CreateFolder(ctx, "tok", key, catalogv2.FolderWrite{Name: &name}); err != nil {
			t.Fatal(err)
		}
	}

	want := map[string][]string{
		"POST /v2/me/claims":    {"moyu-attempt", "moyu-attempt", "", ""},
		"POST /v2/me/proposals": {"moyu-attempt", ""},
		"POST /v2/me/folders":   {"moyu-attempt", ""},
	}
	for route, keys := range want {
		got := k.keys[route]
		if len(got) != len(keys) {
			t.Fatalf("%s: keys %v, want %v", route, got, keys)
		}
		for i := range keys {
			if got[i] != keys[i] {
				t.Fatalf("%s: keys %v, want %v", route, got, keys)
			}
		}
	}
}

func TestReadsAndTransitionsSendNoKey(t *testing.T) {
	k := &keyed{keys: map[string][]string{}}
	c := catalogv2.New(k.server(t).URL, "k")
	ctx := context.Background()
	if _, _, err := c.GetMyClaim(ctx, "tok", 7); err != nil {
		t.Fatal(err)
	}
	if _, err := c.PatchClaim(ctx, "tok", 7, catalogv2.ClaimStateLive, "*"); err != nil {
		t.Fatal(err)
	}
	for route, keys := range k.keys {
		for _, key := range keys {
			if key != "" {
				t.Errorf("%s sent Idempotency-Key %q; infra only honours it on POST", route, key)
			}
		}
	}
}
