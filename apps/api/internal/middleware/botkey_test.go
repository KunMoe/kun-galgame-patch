package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"kun-galgame-patch-api/internal/middleware"

	"github.com/gofiber/fiber/v3"
)

func TestBotKey(t *testing.T) {
	app := fiber.New()
	app.Post("/bot/x", middleware.BotKey("secret"), func(c fiber.Ctx) error {
		return c.SendString("ok")
	})
	cases := []struct {
		name   string
		header string
		code   int
	}{
		{"missing", "", http.StatusUnauthorized},
		{"wrong", "Bearer other", http.StatusUnauthorized},
		{"ok", "Bearer secret", http.StatusOK},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodPost, "/bot/x", nil)
		if c.header != "" {
			req.Header.Set("Authorization", c.header)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != c.code {
			t.Errorf("%s: status %d, want %d", c.name, resp.StatusCode, c.code)
		}
	}
}
