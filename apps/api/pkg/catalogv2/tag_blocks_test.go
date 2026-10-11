package catalogv2

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"kun-galgame-patch-api/pkg/catalogv2/catalogv2test"
)

type tagBlockCall struct {
	method, path, query, auth, body string
}

func tagBlockFace(t *testing.T, answer func(w http.ResponseWriter, r *http.Request)) (*Client, func() []tagBlockCall) {
	t.Helper()
	var (
		mu    sync.Mutex
		calls []tagBlockCall
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		calls = append(calls, tagBlockCall{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization"), string(body)})
		mu.Unlock()
		answer(w, r)
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, "nmk_live_key"), func() []tagBlockCall {
		mu.Lock()
		defer mu.Unlock()
		return append([]tagBlockCall(nil), calls...)
	}
}

func TestMyTagBlocksReadsTheWholeListWithTheReadersToken(t *testing.T) {
	c, calls := tagBlockFace(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"object":"list","items":[
			{"object":"tag_block","tag_id":"6","mode":"tree","created_at":"2026-10-11T01:00:00Z","updated_at":"2026-10-11T01:00:00Z"},
			{"object":"tag_block","tag_id":"9","mode":"exact","created_at":"2026-10-11T02:00:00Z","updated_at":"2026-10-11T03:00:00Z"}
		],"next_cursor":null}`))
	})

	got, err := c.MyTagBlocks(context.Background(), "user-jwt")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].TagID != 6 || got[0].Mode != TagBlockTree || got[1].TagID != 9 || got[1].Mode != TagBlockExact {
		t.Fatalf("MyTagBlocks = %+v", got)
	}
	call := calls()[0]
	if call.auth != "Bearer user-jwt" || call.path != "/v2/me/blocked-tags" || call.query != "limit=100" {
		t.Fatalf("sent %+v, want the reader's token and one page of 100", call)
	}
}

func TestTagBlockWritesNameTheTagAndTheMode(t *testing.T) {
	c, calls := tagBlockFace(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = w.Write([]byte(`{"object":"tag_block","tag_id":"42","mode":"exact","created_at":"2026-10-11T01:00:00Z","updated_at":"2026-10-11T02:00:00Z"}`))
	})
	ctx := context.Background()

	block, err := c.PutTagBlock(ctx, "user-jwt", 42, TagBlockExact)
	if err != nil {
		t.Fatal(err)
	}
	if block.TagID != 42 || block.Mode != TagBlockExact {
		t.Fatalf("PutTagBlock = %+v", block)
	}
	if err := c.DeleteTagBlock(ctx, "user-jwt", 42); err != nil {
		t.Fatal(err)
	}
	got := calls()
	if got[0].method != http.MethodPut || got[0].path != "/v2/me/blocked-tags/42" || got[0].body != `{"mode":"exact"}` {
		t.Errorf("PUT sent %+v", got[0])
	}
	if got[1].method != http.MethodDelete || got[1].path != "/v2/me/blocked-tags/42" || got[1].auth != "Bearer user-jwt" {
		t.Errorf("DELETE sent %+v", got[1])
	}
}

func TestTagBlockRefusalsKeepTheirKind(t *testing.T) {
	for _, tc := range []struct {
		code     string
		notFound bool
	}{
		{"NOT_FOUND", true},
		{"VALIDATION_FAILED", false},
	} {
		c, _ := tagBlockFace(t, func(w http.ResponseWriter, r *http.Request) {
			catalogv2test.Problem(w, r, tc.code, "refused", nil)
		})
		_, err := c.PutTagBlock(context.Background(), "user-jwt", 42, TagBlockTree)
		if err == nil || IsNotFound(err) != tc.notFound {
			t.Errorf("%s: err = %v, IsNotFound = %v", tc.code, err, IsNotFound(err))
		}
	}
	if _, err := New("http://127.0.0.1:1", "k").MyTagBlocks(context.Background(), ""); err != ErrNoAccessToken {
		t.Errorf("no token: err = %v, want ErrNoAccessToken", err)
	}
}
