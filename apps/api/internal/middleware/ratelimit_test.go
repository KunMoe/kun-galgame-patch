package middleware_test

import (
	"net/http"
	"testing"
	"time"

	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/internal/testutil"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
)

func TestRateLimit_AllowsWithinLimit(t *testing.T) {
	ta := testutil.NewTestApp(t)

	ta.App.Get("/limited",
		middleware.RateLimit(ta.RDB, "test", 3, time.Minute),
		func(c fiber.Ctx) error {
			return c.JSON(response.Response{Code: 0, Message: "OK"})
		},
	)

	for i := 0; i < 3; i++ {
		resp := ta.Request(t, http.MethodGet, "/limited", "", "")
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}
}

func TestRateLimit_BlocksOverLimit(t *testing.T) {
	ta := testutil.NewTestApp(t)

	ta.App.Get("/limited",
		middleware.RateLimit(ta.RDB, "test2", 2, time.Minute),
		func(c fiber.Ctx) error {
			return c.JSON(response.Response{Code: 0, Message: "OK"})
		},
	)

	// First 2 should pass
	ta.Request(t, http.MethodGet, "/limited", "", "")
	ta.Request(t, http.MethodGet, "/limited", "", "")

	// Third should be rate limited
	resp := ta.Request(t, http.MethodGet, "/limited", "", "")
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)

	r := testutil.ParseResponse(t, resp)
	assert.Equal(t, 42900, r.Code)
}
