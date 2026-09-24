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
		k.keys[r.Method+" "+r.URL.Path] = append(k.keys[r.Method+" "+r.URL.Path], r.Header.Get("Idempotency-Key"))
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

// Catalog replays the first answer to a repeated key for 24h and writes nothing
// new, which is what makes a reader's retry of a timed-out submit safe. A POST
// that sends no key makes a second record every time it is retried.
func TestEveryCreateSendsAnIdempotencyKey(t *testing.T) {
	k := &keyed{keys: map[string][]string{}}
	c := catalogv2.New(k.server(t).URL, "k")
	ctx := context.Background()
	name := "n"
	fields := map[string]any{"catalog.work.display_name": "夏日口袋"}
	patch := map[string]any{"catalog.work.olang": "ja"}

	for _, actor := range []int{42, 42, 43} {
		if _, err := c.MintClaim(ctx, "tok", actor, fields); err != nil {
			t.Fatal(err)
		}
		if _, err := c.CreateClaim(ctx, "tok", actor, 7, 7); err != nil {
			t.Fatal(err)
		}
		if _, err := c.CreateProposal(ctx, "tok", actor, catalogv2.EntityTypeWork, 7, patch, "note"); err != nil {
			t.Fatal(err)
		}
		if _, err := c.CreateFolder(ctx, "tok", actor, catalogv2.FolderWrite{Name: &name}); err != nil {
			t.Fatal(err)
		}
	}

	claims := k.keys["POST /v2/me/claims"]
	if len(claims) != 6 {
		t.Fatalf("claims: %v", claims)
	}
	for route, keys := range map[string][]string{
		"mint":     {claims[0], claims[2], claims[4]},
		"adopt":    {claims[1], claims[3], claims[5]},
		"proposal": k.keys["POST /v2/me/proposals"],
		"folder":   k.keys["POST /v2/me/folders"],
	} {
		if keys[0] == "" || len(keys[0]) > 255 {
			t.Fatalf("%s sent key %q", route, keys[0])
		}
		if keys[0] != keys[1] {
			t.Errorf("%s: the same write by the same reader must repeat its key", route)
		}
		if keys[0] == keys[2] {
			t.Errorf("%s: two readers must never share a key", route)
		}
	}
	if claims[0] == claims[1] {
		t.Error("a mint and an adoption must never share a key")
	}
}

func TestADifferentScopeIsADifferentWrite(t *testing.T) {
	k := &keyed{keys: map[string][]string{}}
	c := catalogv2.New(k.server(t).URL, "k")
	name := "n"
	for _, scope := range []string{"", "5", "5"} {
		if _, err := c.CreateFolder(context.Background(), "tok", 42, catalogv2.FolderWrite{Name: &name}, scope); err != nil {
			t.Fatal(err)
		}
	}
	keys := k.keys["POST /v2/me/folders"]
	if keys[0] == keys[1] || keys[1] != keys[2] {
		t.Fatalf("keys = %v", keys)
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
