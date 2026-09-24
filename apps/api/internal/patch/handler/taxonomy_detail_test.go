package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/pkg/catalogv2/catalogv2test"

	"github.com/gofiber/fiber/v3"
)

func TestGalgameTaxonomyDetailStatusCodes(t *testing.T) {
	for _, tc := range []struct {
		name        string
		path        string
		code        string
		detail      string
		extra       map[string]any
		wantStatus  int
		wantBodyHas string
	}{
		{
			name: "a tag the registry has no row for is a real 404",
			path: "/api/v1/tag/_?tag_id=99999999", code: "NOT_FOUND", detail: "No tag with this id.",
			wantStatus: http.StatusNotFound, wantBodyHas: `"code":40400`,
		},
		{
			name: "an official the registry has no row for is a real 404",
			path: "/api/v1/official/_?official_id=99999999", code: "NOT_FOUND", detail: "No company with this id.",
			wantStatus: http.StatusNotFound, wantBodyHas: `"code":40400`,
		},
		{
			name:       "an id that is not a number never reaches the catalog, and is still a miss",
			path:       "/api/v1/tag/_?tag_id=abc",
			wantStatus: http.StatusNotFound, wantBodyHas: `"code":40400`,
		},
		{
			name: "a merged official forwards to its survivor",
			path: "/api/v1/official/_?official_id=13323", code: "ENTITY_MERGED", detail: "company 13323 was merged.",
			extra:      map[string]any{"object": "company", "current_id": "6935"},
			wantStatus: http.StatusOK, wantBodyHas: `"moved_to":6935`,
		},
		{
			name: "the registry falling over is an outage, not a missing tag",
			path: "/api/v1/tag/_?tag_id=11", code: "INTERNAL_ERROR", detail: "panic recovered",
			wantStatus: http.StatusServiceUnavailable, wantBodyHas: `"code":50320`,
		},
		{
			name: "moyu's key losing a scope is moyu's 500, not the reader's 4xx",
			path: "/api/v1/tag/_?tag_id=11", code: "SCOPE_REQUIRED", detail: "this operation requires the catalog:read scope.",
			wantStatus: http.StatusInternalServerError, wantBodyHas: `"code":50000`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				catalogv2test.Problem(w, r, tc.code, tc.detail, tc.extra)
			}))
			t.Cleanup(upstream.Close)

			h := New(nil, galgameClient.NewWithKey(upstream.URL, "nm_test_key"), nil, nil)
			app := fiber.New()
			app.Get("/api/v1/tag/:name", h.GalgameTaxonomyDetailProxy)
			app.Get("/api/v1/official/:name", h.GalgameTaxonomyDetailProxy)

			req, _ := http.NewRequest(http.MethodGet, "http://localhost"+tc.path, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body=%s)", resp.StatusCode, tc.wantStatus, body)
			}
			if tc.wantBodyHas != "" && !strings.Contains(string(body), tc.wantBodyHas) {
				t.Errorf("body = %s, want it to carry %s", body, tc.wantBodyHas)
			}
			if tc.detail != "" && strings.Contains(string(body), tc.detail) {
				t.Errorf("catalog's English reached the reader: %s", body)
			}
		})
	}
}
