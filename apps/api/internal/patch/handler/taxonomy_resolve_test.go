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

func TestResolveTaxonomyIDStatusCodes(t *testing.T) {
	const (
		labelHit  = `{"object":"list","items":[{"id":"8801","display_name":"Brand"}]}`
		labelMiss = `{"object":"list","items":[],"missing":["curated:31"]}`
		routeGone = "NOT_FOUND"
	)

	cases := []struct {
		name           string
		path           string
		upstreamStatus int
		upstreamBody   string
		wantStatus     int
		wantBodyHas    string
	}{
		{
			name:       "a mapped wiki tag resolves to its successor",
			path:       "/taxonomy/resolve/tag/1",
			wantStatus: http.StatusOK, wantBodyHas: `"catalog_id":55`,
		},
		{
			name:       "a parked wiki tag is GONE, not missing",
			path:       "/taxonomy/resolve/tag/15",
			wantStatus: http.StatusGone,
		},
		{
			name:       "an id that was never a wiki tag is a plain 404",
			path:       "/taxonomy/resolve/tag/99999999",
			wantStatus: http.StatusNotFound,
		},
		{
			name:           "a registered official resolves through the live lookup",
			path:           "/taxonomy/resolve/official/31",
			upstreamStatus: http.StatusOK, upstreamBody: labelHit,
			wantStatus: http.StatusOK, wantBodyHas: `"catalog_id":8801`,
		},
		{
			name:           "an official the registry has no anchor for is a 404",
			path:           "/taxonomy/resolve/official/31",
			upstreamStatus: http.StatusOK, upstreamBody: labelMiss,
			wantStatus: http.StatusNotFound,
		},
		{
			name:           "an upstream route failure is a miss on v2",
			path:           "/taxonomy/resolve/official/31",
			upstreamStatus: http.StatusNotFound, upstreamBody: routeGone,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "an unknown family is a bad request",
			path:       "/taxonomy/resolve/character/1",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "a non-numeric id is a bad request",
			path:       "/taxonomy/resolve/tag/abc",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.upstreamStatus >= 400 {
					catalogv2test.Problem(w, r, tc.upstreamBody, "No company with this ref.", nil)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.upstreamBody))
			}))
			t.Cleanup(upstream.Close)

			h := New(nil, galgameClient.NewWithKey(upstream.URL, "nm_test_key"), nil, nil)
			app := fiber.New()
			app.Get("/taxonomy/resolve/:kind/:id", h.ResolveTaxonomyID)

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
		})
	}
}
