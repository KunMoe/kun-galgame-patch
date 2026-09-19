package common

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestResourceDetailRejectsANonNumericIDBeforeTheDB(t *testing.T) {
	for _, id := range []string{"NaN", "undefined", "1%20OR%201=1", "0", "-3"} {
		t.Run(id, func(t *testing.T) {
			// db is nil on purpose: the guard has to answer before anything
			// touches GORM, so a regression shows up as a panic, not a 404.
			h := NewHandler(nil, nil, nil, nil, nil)
			app := fiber.New()
			app.Get("/resource/:id", h.GetResourceDetail)

			req, _ := http.NewRequest(http.MethodGet, "http://localhost/resource/"+id, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body=%s)", resp.StatusCode, body)
			}
			if !strings.Contains(string(body), "invalid resource id") {
				t.Errorf("body = %s, want it to name the bad id", body)
			}
		})
	}
}
