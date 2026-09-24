package catalogv2_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kun-galgame-patch-api/pkg/catalogv2"
	"kun-galgame-patch-api/pkg/catalogv2/catalogv2test"
)

func TestOriginStripsLegacySuffix(t *testing.T) {
	c := catalogv2.New("http://catalog:9281/api/v1", "nmk_test_x")
	if !c.Configured() {
		t.Fatal("configured")
	}
}

func TestListWorksUsesBearerAndNoEnvelope(t *testing.T) {
	var gotAuth, gotPath, gotNSFW string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotNSFW = r.URL.Query().Get("nsfw")
		if r.Header.Get("X-API-Key") != "" {
			t.Error("X-API-Key must not be sent")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"items": []map[string]any{{
				"object": "work", "id": "4", "display_name": "Summer Pockets REFLECTION BLUE",
			}},
			"total": 1,
		})
	}))
	t.Cleanup(srv.Close)

	c := catalogv2.New(srv.URL, "nmk_test_abcdefghijklmnopqrstuvwx12")
	page, err := c.ListWorks(context.Background(), catalogv2.WorksQuery{
		Q: "夏", Sort: "relevance", NSFW: true, IncludeTotal: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer nmk_test_abcdefghijklmnopqrstuvwx12" {
		t.Fatalf("auth %q", gotAuth)
	}
	if gotPath != "/v2/catalog/works" {
		t.Fatalf("path %q", gotPath)
	}
	if gotNSFW != "true" {
		t.Fatalf("nsfw %q", gotNSFW)
	}
	if len(page.Items) != 1 || page.Items[0].ID != "4" || page.Count() != 1 {
		t.Fatalf("%+v", page)
	}
}

func TestGetWork404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		catalogv2test.Problem(w, r, "NOT_FOUND", "No work with this id.", nil)
	}))
	t.Cleanup(srv.Close)
	c := catalogv2.New(srv.URL, "nmk_test_x")
	_, err := c.GetWork(context.Background(), 1, false)
	if !catalogv2.IsNotFound(err) {
		t.Fatalf("%v", err)
	}
}

func TestMergedProblem(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		catalogv2test.Problem(w, r, "ENTITY_MERGED", "company 13323 was merged into 6935.",
			map[string]any{"object": "company", "current_id": "6935"})
	}))
	t.Cleanup(srv.Close)
	c := catalogv2.New(srv.URL, "k")
	_, err := c.GetCompany(context.Background(), 13323, true)
	if to, ok := catalogv2.MergedInto(err); !ok || to != 6935 {
		t.Fatalf("MergedInto = (%d, %v): %v", to, ok, err)
	}
	if catalogv2.IsNotFound(err) {
		t.Fatal("a merged id has somewhere to go and must not read as a plain miss")
	}
}

func TestPageCursor(t *testing.T) {
	if catalogv2.PageCursor(1) != "" {
		t.Fatal("page 1")
	}
	if !strings.HasPrefix(catalogv2.PageCursor(2), "cur_") {
		t.Fatal("page 2")
	}
}

func TestUnconfigured(t *testing.T) {
	c := catalogv2.New("", "")
	if _, err := c.ListWorks(context.Background(), catalogv2.WorksQuery{}); !errors.Is(err, catalogv2.ErrNotConfigured) {
		t.Fatalf("%v", err)
	}
}

func TestCreateClaimPostsWorkID(t *testing.T) {
	var gotPath, gotAuth, gotMatch string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth, gotMatch = r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("If-Match")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "claim", "id": "7", "state": "draft", "display_name": "A",
		})
	}))
	t.Cleanup(srv.Close)

	out, err := catalogv2.New(srv.URL, "nmk_test_x").CreateClaim(context.Background(), "tok", 42, 7, 7)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v2/me/claims" || gotAuth != "Bearer tok" {
		t.Fatalf("%s %s", gotPath, gotAuth)
	}
	if gotMatch != "" {
		t.Fatalf("create must not send If-Match, got %q", gotMatch)
	}
	if gotBody["work_id"] != "7" || gotBody["site_work_id"] != "7" {
		t.Fatalf("%v", gotBody)
	}
	if out.WorkID() != 7 || out.State != "draft" {
		t.Fatalf("%+v", out)
	}
}

func TestPatchClaimSendsTheStateAndTheCallersIfMatch(t *testing.T) {
	var gotPath, gotMatch string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMatch = r.URL.Path, r.Header.Get("If-Match")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "claim", "id": "7", "state": "live",
		})
	}))
	t.Cleanup(srv.Close)

	out, err := catalogv2.New(srv.URL, "nmk_test_x").PatchClaim(context.Background(), "tok", 7, "live", `"c7.draft"`)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v2/me/claims/7" || gotMatch != `"c7.draft"` || gotBody["state"] != "live" {
		t.Fatalf("path=%s match=%s body=%v", gotPath, gotMatch, gotBody)
	}
	if out.State != "live" {
		t.Fatalf("%+v", out)
	}
}

func TestMergedProposalTotalUsesPublicV2(t *testing.T) {
	var gotPath, gotAuth, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth, gotQuery = r.URL.Path, r.Header.Get("Authorization"), r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "list", "items": []any{}, "total": 12,
		})
	}))
	t.Cleanup(srv.Close)

	n, err := catalogv2.New(srv.URL, "nmk_test_x").MergedProposalTotal(context.Background(), 2, "kungal")
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer nmk_test_x" || gotPath != "/v2/catalog/proposals" {
		t.Fatalf("%s %s", gotAuth, gotPath)
	}
	if !strings.Contains(gotQuery, "proposer_uid=2") || !strings.Contains(gotQuery, "state=merged") ||
		!strings.Contains(gotQuery, "include_total=true") {
		t.Fatalf("query %s", gotQuery)
	}
	if !strings.Contains(gotQuery, "site=kungal") {
		t.Fatalf("without site= the tally counts every tenant: %s", gotQuery)
	}
	if n != 12 {
		t.Fatalf("total %d", n)
	}
}

func TestMintClaimSendsTheWizardMapAsFieldValues(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "claim", "id": "51", "state": "pending",
		})
	}))
	t.Cleanup(srv.Close)

	fields := map[string]any{
		"catalog.work.display_name": "夏日口袋",
		"catalog.work.olang":        "ja",
		"catalog.work.titles":       []any{map[string]any{"lang": "zh-Hans", "title": "夏日口袋", "kind": 0}},
	}
	out, err := catalogv2.New(srv.URL, "nmk_test_x").MintClaim(context.Background(), "tok", 42, fields)
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/v2/me/claims" {
		t.Fatalf("%s %s", gotMethod, gotPath)
	}
	if _, sent := gotBody["work_id"]; sent {
		t.Fatalf("work_id beside field_values is 422 VALIDATION_FAILED: %v", gotBody)
	}
	if _, sent := gotBody["display_name"]; sent {
		t.Fatalf("a top-level display_name pins the seed the olang title would rewrite: %v", gotBody)
	}
	values, ok := gotBody["field_values"].(map[string]any)
	if !ok || len(values) != len(fields) || values["catalog.work.olang"] != "ja" {
		t.Fatalf("field_values must carry the wizard map verbatim: %v", gotBody)
	}
	if out.WorkID() != 51 || out.State != "pending" {
		t.Fatalf("%+v", out)
	}
}

func TestMyClaimsDecodesTheRichClaimRow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("claim_state"); got != "pending,declined" {
			t.Errorf("claim_state = %q", got)
		}
		_, _ = io.WriteString(w, `{"object":"list","items":[{"object":"claim","id":"7",`+
			`"state":"declined","product_work_id":"4242","last_event":{"reason":"资料不足"}}]}`)
	}))
	t.Cleanup(srv.Close)

	page, err := catalogv2.New(srv.URL, "nmk_test_x").MyClaims(context.Background(), "tok",
		catalogv2.MyClaimsQuery{ClaimStates: []string{"pending", "declined"}, Site: "kungal"})
	if err != nil {
		t.Fatal(err)
	}
	row := page.Items[0]
	gid := row.ProductID()
	if gid == nil || *gid != 4242 {
		t.Fatalf("product_work_id is the patch id the page links to: %+v", row)
	}
	if reason := row.LastReason(); reason == nil || *reason != "资料不足" {
		t.Fatalf("the decline reason lives on last_event: %+v", row)
	}
}
