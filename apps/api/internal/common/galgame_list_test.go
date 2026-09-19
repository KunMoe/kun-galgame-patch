package common

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"

	"github.com/gofiber/fiber/v3"
)

func TestCatalogLibraryRequest_OnlyTheLibraryFlagLeavesTheLocalList(t *testing.T) {
	if catalogLibraryRequest(galgameListRequest{
		SelectedType: "all", SortField: "resource_update_time", Page: 1, Limit: 24,
	}) {
		t.Fatal("bare /galgame is the patch resource list")
	}
	if !catalogLibraryRequest(galgameListRequest{
		SelectedType: "all", SortField: "popularity", Page: 1, Limit: 24, Library: true,
	}) {
		t.Fatal("library=true is the catalog information library")
	}
	if catalogLibraryRequest(galgameListRequest{
		SelectedType: "all", SortField: "created", Library: true, Indexed: true,
	}) {
		t.Fatal("indexed=1 is the sitemap, not the library")
	}
}

func TestCatalogLibrary_DoesNotSendClaimState(t *testing.T) {
	var (
		mu    sync.Mutex
		path  string
		query map[string][]string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mu.Lock()
		path = req.URL.Path
		query = req.URL.Query()
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","items":[],"total":0}`))
	}))
	t.Cleanup(srv.Close)

	h := NewHandler(nil, galgameClient.NewWithKey(srv.URL, "nm_test_key"), nil, nil, nil)
	app := fiber.New()
	app.Get("/galgame", h.GetGalgameList)

	req := httptest.NewRequest(http.MethodGet,
		"/galgame?selected_type=all&sort_field=popularity&sort_order=desc&page=1&limit=24&library=true", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body = %s", resp.StatusCode, body)
	}
	var env struct {
		Code int `json:"code"`
		Data struct {
			Total int `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Code != 0 || env.Data.Total != 0 {
		t.Errorf("envelope = %+v", env)
	}
	mu.Lock()
	defer mu.Unlock()
	if path != "/v2/catalog/works" {
		t.Errorf("path = %q, want /v2/catalog/works", path)
	}
	if got := first(query["claim_state"]); got != "" {
		t.Errorf("claim_state = %q, want it absent", got)
	}
	if got := first(query["sort"]); got != "popularity" {
		t.Errorf("sort = %q, want popularity", got)
	}
}

func first(v []string) string {
	if len(v) == 0 {
		return ""
	}
	return v[0]
}
