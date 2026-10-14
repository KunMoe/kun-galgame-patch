package app

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	faceHandler "kun-galgame-patch-api/internal/face/handler"

	"github.com/gofiber/fiber/v3"
)

func TestFaceFileRouteNeedsTheFilesScope(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: globalErrorHandler})
	mountFace(app, faceHandler.New(nil))

	cases := []struct {
		name, scopes string
		status       int
		code         string
	}{
		{"no scopes header", "", 403, "SCOPE_REQUIRED"},
		{"other scopes only", "catalog:read", 403, "SCOPE_REQUIRED"},
		{"a longer scope with the same prefix", "catalog:read moyu:filesystem", 403, "SCOPE_REQUIRED"},
		// A malformed id is refused before the service is reached, so a 400
		// here proves the scope check let the request through.
		{"granted", "catalog:read moyu:files", 400, "INVALID_PARAMETER"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v2/moyu/resources/abc/file", nil)
			if tc.scopes != "" {
				req.Header.Set("X-NextMoe-Scopes", tc.scopes)
			}
			res, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(res.Body)
			if res.StatusCode != tc.status {
				t.Fatalf("status = %d, want %d: %s", res.StatusCode, tc.status, body)
			}
			var p struct{ Code string }
			if err := json.Unmarshal(body, &p); err != nil || p.Code != tc.code {
				t.Errorf("code = %q, want %q: %s", p.Code, tc.code, body)
			}
			if cc := res.Header.Get("Cache-Control"); cc != "private, no-store" {
				t.Errorf("Cache-Control = %q, want private, no-store", cc)
			}
		})
	}
}
