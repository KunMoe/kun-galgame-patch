package catalogv2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func redirectsServer(t *testing.T, body string, seen *url.Values) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, "nmk_test_key")
}

func TestRedirectsReadsTheMergeFeed(t *testing.T) {
	ctx := context.Background()

	t.Run("a merged id arrives beside its survivor", func(t *testing.T) {
		var q url.Values
		c := redirectsServer(t, `{"object":"list","items":[
			{"object":"redirect","target_object":"work","old_id":"222199","current_id":"16269","merged_at":"2026-09-14T02:00:00Z"}
		],"next_cursor":"cur_next"}`, &q)
		page, err := c.Redirects(ctx, "", 100)
		if err != nil {
			t.Fatalf("Redirects: %v", err)
		}
		if len(page.Items) != 1 {
			t.Fatalf("items = %d, want 1", len(page.Items))
		}
		got := page.Items[0]
		if got.OldID != 222199 || got.CurrentID != 16269 {
			t.Errorf("item = %+v, want 222199 -> 16269", got)
		}
		if page.NextCursor != "cur_next" {
			t.Errorf("next_cursor = %q, want cur_next", page.NextCursor)
		}
		if q.Get("object") != "work" {
			t.Errorf("object = %q, want the request pinned to work", q.Get("object"))
		}
	})

	t.Run("a non-work family is dropped, not folded into a page", func(t *testing.T) {
		var q url.Values
		c := redirectsServer(t, `{"object":"list","items":[
			{"object":"redirect","target_object":"person","old_id":"5","current_id":"9","merged_at":"2026-09-14T02:00:00Z"},
			{"object":"redirect","target_object":"work","old_id":"6","current_id":"7","merged_at":"2026-09-14T02:00:01Z"}
		],"next_cursor":null}`, &q)
		page, err := c.Redirects(ctx, "", 100)
		if err != nil {
			t.Fatalf("Redirects: %v", err)
		}
		if len(page.Items) != 1 || page.Items[0].OldID != 6 {
			t.Fatalf("items = %+v, want only work 6", page.Items)
		}
		if page.NextCursor != "" {
			t.Errorf("next_cursor = %q, want empty on a short page", page.NextCursor)
		}
	})

	t.Run("an unusable id is dropped rather than read as zero", func(t *testing.T) {
		var q url.Values
		c := redirectsServer(t, `{"object":"list","items":[
			{"object":"redirect","target_object":"work","old_id":"","current_id":"7","merged_at":"2026-09-14T02:00:00Z"},
			{"object":"redirect","target_object":"work","old_id":"8","current_id":"nope","merged_at":"2026-09-14T02:00:01Z"}
		]}`, &q)
		page, err := c.Redirects(ctx, "", 100)
		if err != nil {
			t.Fatalf("Redirects: %v", err)
		}
		if len(page.Items) != 0 {
			t.Fatalf("items = %+v, want both dropped", page.Items)
		}
	})

	t.Run("an empty cursor is omitted and the limit stays under the ceiling", func(t *testing.T) {
		var q url.Values
		c := redirectsServer(t, `{"object":"list","items":[]}`, &q)
		if _, err := c.Redirects(ctx, "", 0); err != nil {
			t.Fatalf("Redirects: %v", err)
		}
		if _, ok := q["cursor"]; ok {
			t.Errorf("cursor = %q, want it absent on the bootstrap read", q.Get("cursor"))
		}
		if q.Get("limit") != "100" {
			t.Errorf("limit = %q, want 100", q.Get("limit"))
		}
		// Above the ceiling the call is 400 LIMIT_TOO_LARGE, not clamped.
		if _, err := c.Redirects(ctx, "cur_abc", 500); err != nil {
			t.Fatalf("Redirects: %v", err)
		}
		if q.Get("limit") != "100" {
			t.Errorf("limit = %q, want it capped at 100 before the wire", q.Get("limit"))
		}
		if q.Get("cursor") != "cur_abc" {
			t.Errorf("cursor = %q, want it handed back verbatim", q.Get("cursor"))
		}
	})
}
