package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/testutil"
	"kun-galgame-patch-api/pkg/catalogv2/catalogv2test"
)

func TestTagDescendantsAnswerTheSubtree(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/catalog/tags/6" || r.URL.Query().Get("include") != "descendants" {
			catalogv2test.Problem(w, r, "NOT_FOUND", "no such tag", nil)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"tag","id":"6","display_name":"根","descendants":[` +
			`{"object":"tag","id":"7","display_name":"子","is_sexual":true,"parent_ids":["6"]}]}`))
	}))
	t.Cleanup(srv.Close)
	h := New(nil, galgameClient.NewWithKey(srv.URL, "nm_test_key"), nil, nil, nil)
	ta := testutil.NewTestApp(t)
	ta.App.Get("/tag/:tagId/descendants", h.TagDescendants)

	resp := ta.Request(t, http.MethodGet, "/tag/6/descendants", "", "")
	body := testutil.ReadBody(t, resp)
	want := `"items":[{"id":7,"name":"子","is_sexual":true,"parent_ids":[6]}]`
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, want) {
		t.Fatalf("status = %d body = %s, want %s", resp.StatusCode, body, want)
	}

	resp = ta.Request(t, http.MethodGet, "/tag/8/descendants", "", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown tag: status = %d body = %s", resp.StatusCode, testutil.ReadBody(t, resp))
	}
	resp = ta.Request(t, http.MethodGet, "/tag/x/descendants", "", "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad id: status = %d", resp.StatusCode)
	}
}
