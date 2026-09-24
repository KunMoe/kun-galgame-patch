package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"kun-galgame-patch-api/pkg/catalogv2/catalogv2test"
)

func TestMovedTargetClassification(t *testing.T) {
	for _, tc := range []struct {
		name  string
		code  string
		extra map[string]any
		want  int64
	}{
		{"a merge verdict", "ENTITY_MERGED", map[string]any{"object": "company", "current_id": "6935"}, 6935},
		{"a merge verdict with no target is not actionable", "ENTITY_MERGED", map[string]any{"object": "company"}, 0},
		{"a plain miss is not a merge", "NOT_FOUND", nil, 0},
		{"a conflict is not a merge", "INVALID_STATE_TRANSITION", map[string]any{"current_id": "6935"}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				catalogv2test.Problem(w, r, tc.code, "detail", tc.extra)
			}))
			t.Cleanup(srv.Close)
			_, err := NewWithKey(srv.URL, "nmk_test_key").v2.GetCompany(context.Background(), 13323, true)
			to, ok := MovedTarget(err)
			if (tc.want > 0) != ok || to != tc.want {
				t.Fatalf("MovedTarget = (%d, %v), want (%d, %v)", to, ok, tc.want, tc.want > 0)
			}
		})
	}
	if _, ok := MovedTarget(nil); ok {
		t.Fatal("no error is no merge")
	}
}

func TestCompanyMergeDoesNotFollow(t *testing.T) {
	var seen []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Path)
		if r.URL.Path == "/v2/catalog/companies/13323" {
			w.Header().Set("Link", `</v2/catalog/companies/6935>; rel="canonical"`)
			catalogv2test.Problem(w, r, "ENTITY_MERGED", "company 13323 was merged.",
				map[string]any{"object": "company", "current_id": "6935"})
			return
		}
		_, _ = w.Write([]byte(`{"object":"company","id":"6935","display_name":"生存ブランド","latin":null,"lang":"ja",` +
			`"localized":{},"company_kind":"game_brand","work_count":1,"aliases":[],"intros":[],"links":[]}`))
	}))
	t.Cleanup(upstream.Close)

	c := NewWithKey(upstream.URL, "nmk_test_key")
	_, err := c.v2.GetCompany(context.Background(), 13323, true)
	to, ok := MovedTarget(err)
	if !ok || to != 6935 {
		t.Fatalf("MovedTarget = (%d, %v), want (6935, true); err=%v", to, ok, err)
	}
	if len(seen) != 1 || seen[0] != "/v2/catalog/companies/13323" {
		t.Fatalf("client followed the merge: %v", seen)
	}
}
