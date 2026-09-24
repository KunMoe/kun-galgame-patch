package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func mirrorServer(t *testing.T, body string, seen *url.Values) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return NewWithKey(srv.URL, "nmk_test_key")
}

func mirrorWork(id, rating, limit, claim string) string {
	return `{"object":"work","id":"` + id + `","medium":"galgame","display_name":"W","latin":null,"localized":{},` +
		`"olang":"ja","content_rating":"` + rating + `","content_limit":"` + limit + `","release_date":null,` +
		`"release_date_precision":null,"release_status":"unknown","cover":null,"banner":null,"claim":` + claim + `,` +
		`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`
}

func verdictOf(rows []DisplayVerdict, gid int) (string, bool) {
	for _, r := range rows {
		if r.GID == gid {
			return r.ContentLimit, true
		}
	}
	return "", false
}

func TestDisplayVerdictsOpenBothGates(t *testing.T) {
	var q url.Values
	c := mirrorServer(t, `{"object":"list","items":[]}`, &q)
	if _, err := c.DisplayVerdictsByCatalogIDs(context.Background(), []int64{1, 2}); err != nil {
		t.Fatalf("DisplayVerdictsByCatalogIDs: %v", err)
	}
	if q.Get("nsfw") != "true" {
		t.Errorf("nsfw = %q, want true — the reader's gate would hide the works this is meant to mark nsfw", q.Get("nsfw"))
	}
	if got, ok := q["content_limit"]; ok {
		t.Errorf("content_limit = %v, want it absent", got)
	}
	if got, ok := q["include"]; ok {
		t.Errorf("include = %v, want it absent — the row is keyed by its own id now", got)
	}
	if q.Get("ids") != "1,2" {
		t.Errorf("ids = %q, want 1,2", q.Get("ids"))
	}
}

// The feed hands over catalog ids and a page id IS one, so a verdict is filed
// under the id it arrived as. This test used to assert the opposite — that the
// claim's site_work_id won and an unclaimed work fell back to its `curated`
// anchor — because filing a work under its catalog id then meant marking a
// different game nsfw. It later asserted that a work with no kungal claim took
// its verdict from content_rating, which served explicit covers to SFW readers.
func TestDisplayVerdictKeysOnTheCatalogID(t *testing.T) {
	var q url.Values
	c := mirrorServer(t, `{"object":"list","items":[`+
		mirrorWork("501", "all_ages", "nsfw", `{"site":"kungal","site_work_id":"7001","state":"live","content_limit":"nsfw"}`)+`,`+
		mirrorWork("502", "r18", "sfw", "null")+`,`+
		mirrorWork("503", "sensitive", "nsfw", "null")+`,`+
		mirrorWork("504", "all_ages", "nsfw", `{"site":"letmoe","site_work_id":"88","state":"live","content_limit":"nsfw"}`)+
		`]}`, &q)

	rows, err := c.DisplayVerdictsByCatalogIDs(context.Background(), []int64{501, 502, 503, 504})
	if err != nil {
		t.Fatalf("DisplayVerdictsByCatalogIDs: %v", err)
	}

	for gid, want := range map[int]string{501: "nsfw", 502: "sfw", 503: "nsfw", 504: "nsfw"} {
		if cl, ok := verdictOf(rows, gid); !ok || cl != want {
			t.Errorf("%d = %q/%v, want %s (work.content_limit, whatever the rating)", gid, cl, ok, want)
		}
	}
	for _, foreign := range []int{7001, 88} {
		if _, ok := verdictOf(rows, foreign); ok {
			t.Errorf("%d is somebody else's page number and was filed as ours", foreign)
		}
	}
}

func TestDisplayVerdictsRefuseAnOversizedBatch(t *testing.T) {
	var q url.Values
	c := mirrorServer(t, `{"object":"list","items":[]}`, &q)
	ids := make([]int64, CatalogWorksIDsMax+1)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	if _, err := c.DisplayVerdictsByCatalogIDs(context.Background(), ids); err == nil {
		t.Fatal("want an error above the id ceiling, got nil")
	}
}
