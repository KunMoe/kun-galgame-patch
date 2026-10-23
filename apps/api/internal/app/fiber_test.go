package app

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// go test -race does not see this class of bug: fasthttp recycles the buffer
// through sync.Pool, which is a happens-before edge, so only the values can tell.
func TestRequestValuesSurviveTheHandler(t *testing.T) {
	app := newFiber()
	var kept [][3]string
	app.Get("/", func(c fiber.Ctx) error {
		kept = append(kept, [3]string{c.Query("q"), c.Cookies("k"), c.Get("X-V")})
		return c.SendStatus(fiber.StatusNoContent)
	})

	for i := range 3 {
		v := strconv.Itoa(i) + "-value-padded"
		req := httptest.NewRequest(http.MethodGet, "/?q=q"+v, nil)
		req.AddCookie(&http.Cookie{Name: "k", Value: "k" + v})
		req.Header.Set("X-V", "h"+v)
		if _, err := app.Test(req); err != nil {
			t.Fatal(err)
		}
	}
	for i, got := range kept {
		v := strconv.Itoa(i) + "-value-padded"
		if want := [3]string{"q" + v, "k" + v, "h" + v}; got != want {
			t.Errorf("request %d kept %q, want %q", i, got, want)
		}
	}
}
