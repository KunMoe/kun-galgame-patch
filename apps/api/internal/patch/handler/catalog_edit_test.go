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
	"kun-galgame-patch-api/pkg/catalogv2/catalogv2test"
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
	createKeys []string
	schema     string
	createBody string
	// Every include= the detail face was asked for, in order.
	proposalIncludes []string
	errCode          string
	errDetail        string
	errExtra         map[string]any
	errHeader        map[string]string
	schemaErrCode    string
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
			if f.errCode != "" {
				for k, v := range f.errHeader {
					w.Header().Set(k, v)
				}
				if f.errCode == "BAD_GATEWAY_PAGE" {
					w.WriteHeader(http.StatusBadGateway)
					_, _ = w.Write([]byte("<html><body>502 Bad Gateway</body></html>"))
					return
				}
				catalogv2test.Problem(w, r, f.errCode, f.errDetail, f.errExtra)
				return
			}
		}
		if p == "/v2/catalog/schemas/work" && f.schemaErrCode != "" {
			catalogv2test.Problem(w, r, f.schemaErrCode, "this operation requires the catalog:read scope.", nil)
			return
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
			f.createKeys = append(f.createKeys, r.Header.Get("Idempotency-Key"))
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
			catalogv2test.Problem(w, r, "NOT_FOUND", "Nothing visible exists at this URL.", nil)
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
	fake.errCode, fake.errDetail = "SCOPE_REQUIRED", "this operation requires the catalog:edit scope."
	ta, session := newCatalogEditApp(t, fake)

	resp := ta.Request(t, http.MethodPost, "/patch/9000/catalog-edit", `{"display_name":"新名"}`, session)
	r := testutil.ParseResponse(t, resp)
	if r.Code != 40399 || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("scope denial = %d/%d, want 403/40399", resp.StatusCode, r.Code)
	}
}

// The schema is read with moyu's application key. A scope that key lacks is
// fixed by an operator, not by the reader signing in again, which is what the
// old mapping told them.
func TestCatalogEditAppKeyScopeIsMoyusOwn500(t *testing.T) {
	fake := newCatalogEditFake(t)
	fake.schemaErrCode = "SCOPE_REQUIRED"
	ta, session := newCatalogEditApp(t, fake)

	resp := ta.Request(t, http.MethodGet, "/patch/9000/catalog-edit", "", session)
	r := testutil.ParseResponse(t, resp)
	if resp.StatusCode != http.StatusInternalServerError || r.Code != 50000 {
		t.Fatalf("got %d/%d, want 500/50000 (%s)", resp.StatusCode, r.Code, r.Message)
	}
}

func TestCatalogEditUpstreamStatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		code       string
		detail     string
		extra      map[string]any
		header     map[string]string
		wantStatus int
		wantCode   int
	}{
		{"permission", "PERMISSION_REQUIRED",
			"only the proposer, or someone with review standing on this entity, may change this proposal.", nil, nil,
			http.StatusForbidden, 40300},
		{"validation", "VALIDATION_FAILED",
			"element 0: kind must be 0 (official), 1 (alias) or 2 (abbreviation)", nil, nil,
			http.StatusUnprocessableEntity, 42200},
		{"stale ETag", "PRECONDITION_FAILED", "If-Match did not match the current representation.", nil, nil,
			http.StatusConflict, 40900},
		{"decided already", "DECISION_ALREADY_MADE", "proposal 32 is not open.", nil, nil,
			http.StatusConflict, 40900},
		{"token rejected", "INVALID_CREDENTIAL", "Authorization Bearer token is invalid.", nil, nil,
			http.StatusForbidden, 40399},
		{"moyu's client has no site", "SITE_NOT_BOUND", "the access token's client is not bound to a catalog site.", nil, nil,
			http.StatusInternalServerError, 50000},
		{"an app key on the user face", "USER_IDENTITY_REQUIRED", "this operation requires a user access token.", nil, nil,
			http.StatusInternalServerError, 50000},
		{"rate limited", "RATE_LIMITED", "The short-window rate limit was exceeded.", nil,
			map[string]string{"Retry-After": "12"}, http.StatusTooManyRequests, 42900},
		{"catalog down", "SERVICE_UNAVAILABLE", "proposals are not bound.", nil, nil,
			http.StatusServiceUnavailable, 50320},
		{"a gateway page", "BAD_GATEWAY_PAGE", "", nil, nil,
			http.StatusServiceUnavailable, 50320},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := newCatalogEditFake(t)
			fake.errCode, fake.errDetail, fake.errExtra, fake.errHeader = tc.code, tc.detail, tc.extra, tc.header
			ta, session := newCatalogEditApp(t, fake)

			resp := ta.Request(t, http.MethodPost, "/patch/9000/catalog-edit", `{"display_name":"新名"}`, session)
			r := testutil.ParseResponse(t, resp)
			if resp.StatusCode != tc.wantStatus || r.Code != tc.wantCode {
				t.Fatalf("got %d/%d, want %d/%d (%s)", resp.StatusCode, r.Code, tc.wantStatus, tc.wantCode, r.Message)
			}
			if tc.name == "validation" {
				if !strings.Contains(r.Message, "kind must be") {
					t.Fatalf("a 422 must pass the engine's wording through: %s", r.Message)
				}
				return
			}
			if tc.detail != "" && strings.Contains(r.Message, tc.detail) {
				t.Fatalf("catalog's English reached the reader: %s", r.Message)
			}
			if tc.header != nil && resp.Header.Get("Retry-After") != tc.header["Retry-After"] {
				t.Fatalf("Retry-After = %q", resp.Header.Get("Retry-After"))
			}
		})
	}
}

func TestCatalogEditRelaysTheFieldLevelRejection(t *testing.T) {
	fake := newCatalogEditFake(t)
	fake.errCode, fake.errDetail = "VALIDATION_FAILED", "editing: patch rejected"
	fake.errExtra = map[string]any{"errors": []any{
		map[string]any{"pointer": "/catalog.work.titles", "reason": "UNKNOWN_VALUE",
			"detail": "element 0: kind must be 0 (official), 1 (alias) or 2 (abbreviation)"},
		map[string]any{"pointer": "/patch/catalog.work.olang", "reason": "IMMUTABLE", "detail": "field is locked"},
	}}
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

// infra's proposalErr answers a field the proposer may not touch with
// PERMISSION_REQUIRED and a NOT_PERMITTED row pointing at it
// (apps/api/internal/platform/apiv2/handler/me_proposals.go), which the form
// pins under that field.
func TestCatalogEditKeepsTheFieldOfAPermissionRefusal(t *testing.T) {
	fake := newCatalogEditFake(t)
	fake.errCode, fake.errDetail = "PERMISSION_REQUIRED", "editing: catalog.work.olang needs review standing"
	fake.errExtra = map[string]any{"errors": []any{
		map[string]any{"pointer": "/patch/catalog.work.olang", "reason": "NOT_PERMITTED",
			"detail": "editing: catalog.work.olang needs review standing"},
	}}
	ta, session := newCatalogEditApp(t, fake)

	resp := ta.Request(t, http.MethodPost, "/patch/9000/catalog-edit", `{"olang":"zh-Hans"}`, session)
	r := testutil.ParseResponse(t, resp)
	if resp.StatusCode != http.StatusForbidden || r.Code != 40300 {
		t.Fatalf("got %d/%d, want 403/40300", resp.StatusCode, r.Code)
	}
	rows, _ := r.Data.(map[string]any)["errors"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["reason"] != "NOT_PERMITTED" {
		t.Fatalf("the refused field did not travel: %v", r.Data)
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

// The page mints submit_key once per press of 保存 and keeps it across that
// press's retries; the key catalog dedupes on is derived from it, never from
// the patch, which would replay a withdrawn proposal to a reader proposing the
// same edit again.
func TestCatalogEditSubmitKeysOnThePress(t *testing.T) {
	fake := newCatalogEditFake(t)
	ta, session := newCatalogEditApp(t, fake)

	for _, body := range []string{
		`{"display_name":"新名","submit_key":"press-1"}`,
		`{"display_name":"新名","submit_key":"press-1"}`,
		`{"display_name":"新名","submit_key":"press-2"}`,
		`{"display_name":"新名"}`,
	} {
		resp := ta.Request(t, http.MethodPost, "/patch/9000/catalog-edit", body, session)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d", resp.StatusCode)
		}
	}
	k := fake.createKeys
	if len(k) != 4 || k[0] == "" || k[0] != k[1] || k[1] == k[2] || k[3] != "" {
		t.Fatalf("keys = %q", k)
	}
	if k[0] == "press-1" {
		t.Fatal("the page's value must not be forwarded as the key itself")
	}
}
