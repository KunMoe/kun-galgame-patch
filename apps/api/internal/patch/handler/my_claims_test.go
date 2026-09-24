package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/internal/testutil"
	"kun-galgame-patch-api/pkg/config"
)

const myClaimsReply = `{"object":"list","total":1,"next_cursor":"cur_OTk5",` +
	`"items":[{"object":"claim","id":"7","state":"declined","display_name":"作品名",` +
	`"site":"kungal","product_work_id":"4242","first_acted_at":"2026-08-20T03:04:05Z",` +
	`"acted_count":2,"last_event":{"object":"claim_event","id":"999","from_state":"pending",` +
	`"to_state":"declined","reason":"资料不足","actor_uid":"3","created_at":"2026-08-21T00:00:00Z"}}]}`

func TestListMyGalgamesReadsTheTenantedV2Face(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/me/claims" {
			t.Errorf("path = %s", r.URL.Path)
		}
		got = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(myClaimsReply))
	}))
	t.Cleanup(srv.Close)

	h := New(nil, galgameClient.NewWithKey(srv.URL, "nm_test_key"), nil, nil)
	ta := testutil.NewTestApp(t)
	ta.App.Get("/galgame/mine", middleware.Auth(ta.RDB, config.OAuthConfig{}), h.ListMyGalgames)
	session := ta.CreateTestSession(t, 42)

	resp := ta.Request(t, http.MethodGet, "/galgame/mine?limit=50", "", session)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body = %s", resp.StatusCode, testutil.ReadBody(t, resp))
	}
	if got.Get("site") != "kungal" {
		t.Errorf("site = %q; unpinned, the list answers other tenants' claims", got.Get("site"))
	}
	if got.Get("claim_state") != "pending,declined" || got.Get("include_total") != "true" {
		t.Errorf("query = %v", got)
	}

	data, ok := testutil.ParseResponse(t, resp).Data.(map[string]any)
	if !ok {
		t.Fatalf("data is not an object")
	}
	if data["total"] != float64(1) || data["next_cursor"] != "cur_OTk5" {
		t.Fatalf("page = %v", data)
	}
	row := data["items"].([]any)[0].(map[string]any)
	for key, want := range map[string]any{
		"work_id": float64(7), "display_name": "作品名", "claim_state": "declined",
		"product_work_id": float64(4242), "last_reason": "资料不足",
		"first_acted_at": "2026-08-20T03:04:05Z",
	} {
		if row[key] != want {
			t.Errorf("%s = %v, want %v", key, row[key], want)
		}
	}
}

// catalog answers an unknown claim_state 400, which is moyu's bug by then, so
// the reader's value is checked here and never forwarded.
func TestListMyGalgamesRefusesAnUnknownStateItself(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(myClaimsReply))
	}))
	t.Cleanup(srv.Close)

	h := New(nil, galgameClient.NewWithKey(srv.URL, "nm_test_key"), nil, nil)
	ta := testutil.NewTestApp(t)
	ta.App.Get("/galgame/mine", middleware.Auth(ta.RDB, config.OAuthConfig{}), h.ListMyGalgames)
	session := ta.CreateTestSession(t, 42)

	resp := ta.Request(t, http.MethodGet, "/galgame/mine?claim_state=pending,bogus", "", session)
	if resp.StatusCode != http.StatusBadRequest || calls != 0 {
		t.Fatalf("status = %d after %d catalog calls, want 400 after none", resp.StatusCode, calls)
	}
}
