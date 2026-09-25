package app

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestGlobalErrorHandlerRouterErrors(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: globalErrorHandler})
	app.Get("/api/v1/search", func(c fiber.Ctx) error { return c.SendString("ok") })
	app.Get("/v2/moyu/patches", func(c fiber.Ctx) error { return c.SendString("ok") })

	cases := []struct {
		name, method, path string
		status             int
		code               int
		problem            bool
	}{
		{"unrouted path", "GET", "/api/v1/getGameList", 404, 40400, false},
		{"wrong method", "POST", "/api/v1/search", 405, 40500, false},
		{"face keeps problem+json", "GET", "/v2/moyu/nope", 404, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := app.Test(httptest.NewRequest(tc.method, tc.path, nil))
			if err != nil {
				t.Fatal(err)
			}
			if res.StatusCode != tc.status {
				t.Fatalf("status = %d, want %d", res.StatusCode, tc.status)
			}
			body, _ := io.ReadAll(res.Body)
			if tc.problem {
				if ct := res.Header.Get("Content-Type"); ct != "application/problem+json" {
					t.Fatalf("content-type = %q, body %s", ct, body)
				}
				return
			}
			var env struct {
				Code int `json:"code"`
			}
			if err := json.Unmarshal(body, &env); err != nil || env.Code != tc.code {
				t.Fatalf("envelope code = %d (%v), want %d; body %s", env.Code, err, tc.code, body)
			}
		})
	}
}
