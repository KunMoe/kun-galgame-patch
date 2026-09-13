package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/internal/testutil"
	"kun-galgame-patch-api/pkg/catalogv2"
	"kun-galgame-patch-api/pkg/config"

	"github.com/gofiber/fiber/v3"
)

// The page id IS the catalog work id since migration 037, so the request path
// and the work the handler edits are the same number. They used to differ and a
// fake resolved one to the other.
const catalogEditWorkID = 9000

type catalogEditFake struct {
	t          *testing.T
	userAuth   []string
	lastQuery  map[string]string
	lastCreate map[string]any
	sawCreate  bool
	schema     string
	createBody string
	// Every include= the detail face was asked for, in order.
	proposalIncludes []string
	status           int
	errBody          string
}

func (f *catalogEditFake) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		p := r.URL.Path
		if strings.HasPrefix(p, "/v2/me/") || strings.HasPrefix(p, "/v2/moderation/") {
			f.userAuth = append(f.userAuth, r.Header.Get("Authorization"))
			if f.lastQuery == nil {
				f.lastQuery = map[string]string{}
			}
			for k, v := range r.URL.Query() {
				f.lastQuery[k] = v[0]
			}
			if f.status != 0 {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(f.status)
				_, _ = w.Write([]byte(f.errBody))
				return
			}
		}

		switch {
		case p == "/v2/catalog/works":
			_, _ = w.Write([]byte(`{"object":"list","items":[{"object":"work","id":"9000","refs":[{"source":"galgame_wiki","external_id":"9000"},{"source":"curated","external_id":"9000"}]}]}`))
		case strings.HasPrefix(p, "/v2/catalog/works/"):
			_, _ = w.Write([]byte(`{"object":"work","id":"9000"}`))
		case p == "/v2/catalog/schemas/work":
			_, _ = w.Write([]byte(f.schema))
		case strings.HasPrefix(p, "/v2/moderation/snapshots/"):
			_, _ = w.Write([]byte(`{"object":"snapshot","entity_type":"catalog.work","entity_id":"9000","field_values":{` +
				`"catalog.work.display_name":"作品名","catalog.work.olang":"ja","catalog.work.content_rating":2,` +
				`"catalog.work.display_nsfw":true,` +
				`"catalog.work.titles":[{"lang":"ja","title":"作品名","kind":0},{"lang":"","title":"略称","kind":1,"latin":"ryakusho"}]}}`))
		case p == "/v2/me/proposals" && r.Method == http.MethodPost:
			f.sawCreate = true
			raw, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(raw, &f.lastCreate); err != nil {
				f.t.Errorf("create body is not JSON: %v", err)
			}
			_, _ = w.Write([]byte(f.createBody))
		// The list lane's spec advertises no include at all and its rows are
		// built without a patch; only the detail face carries one. Answering a
		// patch here would hide the hydration this handler now does.
		case p == "/v2/me/proposals" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"object":"list","items":[{"id":"32","state":"open","base_revision_seq":7,` +
				`"proposer_uid":"41","site":"kungal","created_at":"2026-09-01T00:00:00Z"}],"total":1}`))
		case strings.HasPrefix(p, "/v2/me/proposals/") && r.Method == http.MethodGet:
			w.Header().Set("ETag", `"p32"`)
			f.proposalIncludes = append(f.proposalIncludes, r.URL.Query().Get("include"))
			_, _ = w.Write([]byte(`{"id":"32","state":"open","base_revision_seq":7,"proposer_uid":"41","site":"kungal",` +
				`"patch":{"catalog.work.olang":"ja"}}`))
		case strings.HasPrefix(p, "/v2/me/proposals/") && r.Method == http.MethodPatch:
			_, _ = w.Write([]byte(`{"id":"32","state":"withdrawn"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":"NOT_FOUND","status":404}`))
		}
	}
}

const catalogEditSchemaReply = `{"object":"object_schema","entity_type":"catalog.work","fields":[` +
	`{"key":"catalog.work.display_name","field_type":"text","diff_hint":"inline","deprecated":false},` +
	`{"key":"catalog.work.olang","field_type":"enum","diff_hint":"inline","deprecated":false},` +
	`{"key":"catalog.work.content_rating","field_type":"enum","diff_hint":"inline","deprecated":false},` +
	`{"key":"catalog.work.titles","field_type":"list","diff_hint":"items","deprecated":false,"max_elements":40,"max_suppressed":200},` +
	`{"key":"catalog.work.covers","field_type":"list","diff_hint":"items","deprecated":false},` +
	`{"key":"catalog.work.retired","field_type":"text","diff_hint":"inline","deprecated":true}]}`

func newCatalogEditFake(t *testing.T) *catalogEditFake {
	t.Helper()
	return &catalogEditFake{
		t:          t,
		schema:     catalogEditSchemaReply,
		createBody: `{"id":"32","state":"open","entity_id":"9000"}`,
	}
}

func newCatalogEditApp(t *testing.T, fake *catalogEditFake) (*testutil.TestApp, string) {
	t.Helper()
	srv := httptest.NewServer(fake.handler())
	t.Cleanup(srv.Close)

	h := New(nil, galgameClient.NewWithKey(srv.URL, "nm_test_key"), nil, nil)

	ta := testutil.NewTestApp(t)
	auth := middleware.Auth(ta.RDB, config.OAuthConfig{})
	ta.App.Get("/patch/:id/catalog-edit", auth, h.CatalogEditBootstrap)
	ta.App.Post("/patch/:id/catalog-edit", auth, h.CatalogEditSubmit)
	ta.App.Get("/patch/:id/catalog-edit/proposals", auth, h.CatalogEditProposals)
	ta.App.Post("/catalog-proposal/:id/withdraw", auth, h.CatalogProposalWithdraw)
	return ta, ta.CreateTestSession(t, 42)
}

func (f *catalogEditFake) assertUserPlaneSpokeAsTheUser(t *testing.T) {
	t.Helper()
	if len(f.userAuth) == 0 {
		t.Fatal("no user-plane call was made")
	}
	for _, h := range f.userAuth {
		if !strings.HasPrefix(h, "Bearer ") || len(h) <= len("Bearer ") {
			t.Fatalf("user-plane Authorization = %q, want the session user's Bearer token", h)
		}
	}
}

func editData(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	r := testutil.ParseResponse(t, resp)
	data, ok := r.Data.(map[string]any)
	if !ok {
		t.Fatalf("data is not an object: %+v", r)
	}
	return data
}

func TestCatalogEditBootstrapKeepsEveryWireKeyOfTheFourFields(t *testing.T) {
	fake := newCatalogEditFake(t)
	ta, session := newCatalogEditApp(t, fake)

	resp := ta.Request(t, http.MethodGet, "/patch/9000/catalog-edit", "", session)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	data := editData(t, resp)
	if data["work_id"] != float64(catalogEditWorkID) || data["can_edit"] != true {
		t.Fatalf("bootstrap flags: %v", data)
	}

	fields, ok := data["fields"].([]any)
	if !ok {
		t.Fatalf("fields: %v", data["fields"])
	}
	byKey := map[string]map[string]any{}
	for _, f := range fields {
		row := f.(map[string]any)
		byKey[row["key"].(string)] = row
	}
	if len(byKey) != 4 {
		t.Fatalf("the exposed surface is exactly four fields, got %v", byKey)
	}
	for _, key := range catalogEditFieldKeys {
		if _, ok := byKey[key]; !ok {
			t.Fatalf("%s missing from the bootstrap: %v", key, byKey)
		}
	}
	for _, key := range []string{"catalog.work.covers", "catalog.work.retired"} {
		if _, ok := byKey[key]; ok {
			t.Fatalf("%s must never reach the page: %v", key, byKey)
		}
	}

	titles := byKey["catalog.work.titles"]
	for key, want := range map[string]any{
		"key": "catalog.work.titles", "kind": "list", "diff_hint": "items",
		"locked": false, "can_propose": true,
		"max_elements": float64(40), "max_suppressed": float64(200),
	} {
		got, present := titles[key]
		if !present || got != want {
			t.Fatalf("titles field lost %q in the passthrough (got %v, want %v): %v", key, got, want, titles)
		}
	}
	if _, present := titles["deprecated"]; present {
		t.Fatalf("deprecated is omitempty on the wire and must stay absent when false: %v", titles)
	}

	values := data["values"].(map[string]any)
	if values["display_name"] != "作品名" || values["olang"] != "ja" || values["content_rating"] != float64(2) {
		t.Fatalf("bootstrap values must be the engine snapshot, got %v", values)
	}
	rows := values["titles"].([]any)
	if len(rows) != 2 || rows[1].(map[string]any)["latin"] != "ryakusho" {
		t.Fatalf("bootstrap titles: %v", rows)
	}
	fake.assertUserPlaneSpokeAsTheUser(t)
}

func TestCatalogEditBootstrapCanEditFollowsTheSchema(t *testing.T) {
	fake := newCatalogEditFake(t)
	fake.schema = `{"object":"object_schema","entity_type":"catalog.work","fields":[` +
		`{"key":"catalog.work.display_name","field_type":"text","diff_hint":"inline","deprecated":true},` +
		`{"key":"catalog.work.titles","field_type":"list","diff_hint":"items","deprecated":true}]}`
	ta, session := newCatalogEditApp(t, fake)

	data := editData(t, ta.Request(t, http.MethodGet, "/patch/9000/catalog-edit", "", session))
	if data["can_edit"] != false {
		t.Fatalf("a schema that permits nothing must not report can_edit: %v", data)
	}
}

func TestCatalogEditSubmitBuildsTheFieldKeyPatch(t *testing.T) {
	fake := newCatalogEditFake(t)
	ta, session := newCatalogEditApp(t, fake)

	body := `{"display_name":"新名","olang":"zh-Hans","content_rating":0,` +
		`"titles":[{"lang":"ja","title":"新標題","kind":0},{"lang":"","title":"略称","latin":"","kind":1}],` +
		`"note":"fix","cover":"should-be-ignored"}`
	resp := ta.Request(t, http.MethodPost, "/patch/9000/catalog-edit", body, session)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body = %s", resp.StatusCode, testutil.ReadBody(t, resp))
	}
	data := editData(t, resp)
	if data["merged"] != false || data["proposal"] == nil {
		t.Fatalf("submit result: %v", data)
	}
	if _, ok := data["revision"]; ok {
		t.Fatalf("a proposal that did not merge carries no revision: %v", data)
	}

	if !fake.sawCreate {
		t.Fatal("no create reached the catalog")
	}
	if fake.lastCreate["entity_type"] != catalogv2.EntityTypeWork {
		t.Fatalf("create body: %v", fake.lastCreate)
	}
	switch id := fake.lastCreate["entity_id"].(type) {
	case string:
		if id != "9000" {
			t.Fatalf("create entity_id = %q", id)
		}
	case float64:
		if int(id) != catalogEditWorkID {
			t.Fatalf("create entity_id = %v", id)
		}
	default:
		t.Fatalf("create entity_id type %T: %v", id, fake.lastCreate)
	}
	for _, k := range []string{"actor", "site", "user_id", "trust_tier", "proposer_uid"} {
		if _, ok := fake.lastCreate[k]; ok {
			t.Fatalf("the user plane must not carry %q: %v", k, fake.lastCreate)
		}
	}
	patch := fake.lastCreate["patch"].(map[string]any)
	for key := range patch {
		if !slices.Contains(catalogEditFieldKeys, key) {
			t.Fatalf("%s escaped the four-key surface: %v", key, patch)
		}
	}
	if len(patch) != 4 {
		t.Fatalf("patch keys: %v", patch)
	}
	rows := patch["catalog.work.titles"].([]any)
	if len(rows) != 2 {
		t.Fatalf("titles: %v", rows)
	}
	if _, ok := rows[1].(map[string]any)["latin"]; ok {
		t.Fatalf("an empty latin must be omitted, not sent blank: %v", rows[1])
	}
}

func TestCatalogEditSubmitRefusesAnEmptyPatch(t *testing.T) {
	fake := newCatalogEditFake(t)
	ta, session := newCatalogEditApp(t, fake)

	resp := ta.Request(t, http.MethodPost, "/patch/9000/catalog-edit", `{"note":"nothing"}`, session)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
	if fake.sawCreate {
		t.Fatal("an empty patch must never reach the catalog")
	}
}

func TestCatalogEditScopeDenialLandsOnTheRelogInCode(t *testing.T) {
	fake := newCatalogEditFake(t)
	fake.status = http.StatusForbidden
	fake.errBody = `{"code":"SCOPE_REQUIRED","status":403,"title":"Forbidden","detail":"the access token is missing the catalog:edit scope"}`
	ta, session := newCatalogEditApp(t, fake)

	resp := ta.Request(t, http.MethodPost, "/patch/9000/catalog-edit", `{"display_name":"新名"}`, session)
	r := testutil.ParseResponse(t, resp)
	if r.Code != 40399 || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("scope denial = %d/%d, want 403/40399", resp.StatusCode, r.Code)
	}
}

func TestCatalogEditUpstreamStatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       string
		wantStatus int
		wantCode   int
	}{
		{"permission", http.StatusForbidden, `{"code":"FORBIDDEN","status":403,"detail":"编辑该条目需要更高的信任等级"}`,
			http.StatusForbidden, 40300},
		{"validation", http.StatusUnprocessableEntity,
			`{"code":"VALIDATION_FAILED","status":422,"detail":"element 0: kind must be 0 (official), 1 (alias) or 2 (abbreviation)"}`,
			http.StatusUnprocessableEntity, 42200},
		{"conflict", http.StatusConflict, `{"code":"CONFLICT","status":409,"detail":"rebase conflict"}`,
			http.StatusConflict, 40900},
		{"token rejected", http.StatusUnauthorized, `{"code":"INVALID_CREDENTIAL","status":401,"detail":"token expired"}`,
			http.StatusForbidden, 40399},
		{"upstream down", http.StatusBadGateway, `{"code":"BAD_GATEWAY","status":502,"detail":"bad gateway"}`,
			http.StatusServiceUnavailable, 50320},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := newCatalogEditFake(t)
			fake.status, fake.errBody = tc.status, tc.body
			ta, session := newCatalogEditApp(t, fake)

			resp := ta.Request(t, http.MethodPost, "/patch/9000/catalog-edit", `{"display_name":"新名"}`, session)
			r := testutil.ParseResponse(t, resp)
			if resp.StatusCode != tc.wantStatus || r.Code != tc.wantCode {
				t.Fatalf("got %d/%d, want %d/%d (%s)", resp.StatusCode, r.Code, tc.wantStatus, tc.wantCode, r.Message)
			}
			if tc.name == "validation" && !strings.Contains(r.Message, "kind must be") {
				t.Fatalf("a 422 must pass the engine's wording through: %s", r.Message)
			}
		})
	}
}

func TestCatalogEditRelaysTheFieldLevelRejection(t *testing.T) {
	fake := newCatalogEditFake(t)
	fake.status = http.StatusUnprocessableEntity
	fake.errBody = `{"code":"VALIDATION_FAILED","status":422,"detail":"editing: patch rejected","errors":[` +
		`{"pointer":"/catalog.work.titles","reason":"UNKNOWN_VALUE","detail":"element 0: kind must be 0 (official), 1 (alias) or 2 (abbreviation)"},` +
		`{"pointer":"/patch/catalog.work.olang","reason":"IMMUTABLE","detail":"field is locked"}]}`
	ta, session := newCatalogEditApp(t, fake)

	resp := ta.Request(t, http.MethodPost, "/patch/9000/catalog-edit", `{"display_name":"新名"}`, session)
	data := editData(t, resp)
	rows, _ := data["errors"].([]any)
	if len(rows) != 2 {
		t.Fatalf("the field-level errors did not survive the envelope: %v", data)
	}
	first, _ := rows[0].(map[string]any)
	if first["pointer"] != "/catalog.work.titles" || first["reason"] != "UNKNOWN_VALUE" {
		t.Fatalf("field error lost its pointer or reason: %v", first)
	}
	if data["detail"] != "editing: patch rejected" {
		t.Fatalf("the top-level detail is the fallback for a refusal that names no field: %v", data)
	}
}

func TestCatalogEditProposalsAndWithdraw(t *testing.T) {
	fake := newCatalogEditFake(t)
	ta, session := newCatalogEditApp(t, fake)

	data := editData(t, ta.Request(t, http.MethodGet, "/patch/9000/catalog-edit/proposals", "", session))
	items, _ := data["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("proposals: %v", data)
	}
	if fake.lastQuery["entity_id"] != "9000" {
		t.Fatalf("my-proposals query: %v", fake.lastQuery)
	}
	row, _ := items[0].(map[string]any)
	patch, _ := row["patch"].(map[string]any)
	if _, ok := patch["catalog.work.olang"]; !ok {
		t.Fatalf("the listed proposal was not hydrated from the detail face: %v", row)
	}
	if row["base_revision_seq"] != float64(7) || row["proposer_uid"] != float64(41) {
		t.Fatalf("proposal view dropped the fields the card reads: %v", row)
	}
	if len(fake.proposalIncludes) == 0 || fake.proposalIncludes[0] != "patch" {
		t.Fatalf("the detail face was not asked for the patch: %v", fake.proposalIncludes)
	}
	if _, ok := fake.lastQuery["proposer_uid"]; ok {
		t.Fatalf("the user plane names no uid: %v", fake.lastQuery)
	}

	data = editData(t, ta.Request(t, http.MethodPost, "/catalog-proposal/32/withdraw", "", session))
	if data["status"] != "withdrawn" || data["id"] != float64(32) {
		t.Fatalf("withdraw: %v", data)
	}
	fake.assertUserPlaneSpokeAsTheUser(t)
}

func TestCatalogEditNeedsASession(t *testing.T) {
	fake := newCatalogEditFake(t)
	ta, _ := newCatalogEditApp(t, fake)

	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/patch/9000/catalog-edit", ""},
		{http.MethodPost, "/patch/9000/catalog-edit", `{"display_name":"x"}`},
		{http.MethodGet, "/patch/9000/catalog-edit/proposals", ""},
		{http.MethodPost, "/catalog-proposal/32/withdraw", ""},
	} {
		resp := ta.Request(t, tc.method, tc.path, tc.body, "")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s: status %d, want 401", tc.method, tc.path, resp.StatusCode)
		}
	}
	if len(fake.userAuth) != 0 {
		t.Fatal("an anonymous request must never reach the catalog user plane")
	}
}

func TestCatalogEditWithoutACatalogClient(t *testing.T) {
	ta := testutil.NewTestApp(t)
	h := New(nil, nil, nil, nil)
	auth := middleware.Auth(ta.RDB, config.OAuthConfig{})
	ta.App.Get("/patch/:id/catalog-edit", auth, h.CatalogEditBootstrap)
	session := ta.CreateTestSession(t, 42)

	resp := ta.Request(t, http.MethodGet, "/patch/9000/catalog-edit", "", session)
	r := testutil.ParseResponse(t, resp)
	if resp.StatusCode != fiber.StatusServiceUnavailable || r.Code != 50320 {
		t.Fatalf("got %d/%d, want 503/50320", resp.StatusCode, r.Code)
	}
}
