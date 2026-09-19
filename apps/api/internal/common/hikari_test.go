package common

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"kun-galgame-patch-api/internal/middleware"

	"github.com/gofiber/fiber/v3"
)

func TestHikariRetired(t *testing.T) {
	h := NewHandler(nil, nil, nil, nil, nil)
	app := fiber.New()
	api := app.Group("/api/v1")
	api.Use("/hikari", middleware.HikariCORS())
	api.Get("/hikari", h.HikariRetired)

	for _, target := range []string{"/api/v1/hikari?vndb_id=v63478", "/api/v1/hikari"} {
		t.Run(target, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, "http://localhost"+target, nil)
			req.Header.Set("Origin", "https://touchgal.ink")
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			body, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != http.StatusGone {
				t.Fatalf("status = %d, want 410; body=%s", resp.StatusCode, body)
			}
			if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://touchgal.ink" {
				t.Errorf("ACAO = %q, want the partner origin so its page can read the message", got)
			}

			var env map[string]any
			if err := json.Unmarshal(body, &env); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if env["success"] != false {
				t.Errorf("success = %v, want false", env["success"])
			}
			if d, ok := env["data"]; !ok || d != nil {
				t.Errorf("data = %v (present=%v), want null", d, ok)
			}
			msg, _ := env["message"].(string)
			for _, want := range []string{"https://developer.nextmoe.dev", "/v2/moyu/patches?refs=vndb:"} {
				if !strings.Contains(msg, want) {
					t.Errorf("message does not mention %q: %s", want, msg)
				}
			}
		})
	}
}
