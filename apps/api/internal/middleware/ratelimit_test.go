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

	ta.Request(t, http.MethodGet, "/limited", "", "")
	ta.Request(t, http.MethodGet, "/limited", "", "")

	resp := ta.Request(t, http.MethodGet, "/limited", "", "")
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)

	r := testutil.ParseResponse(t, resp)
	assert.Equal(t, 42900, r.Code)
}

func TestRateLimit_AKeyWithoutExpiryHeals(t *testing.T) {
	ta := testutil.NewTestApp(t)
	ta.App.Get("/limited",
		middleware.RateLimit(ta.RDB, "test3", 2, time.Minute),
		func(c fiber.Ctx) error {
			return c.JSON(response.Response{Code: 0, Message: "OK"})
		},
	)
	key := "ratelimit:test3:ip:0.0.0.0"
	assert.NoError(t, ta.MR.Set(key, "1429391"))

	resp := ta.Request(t, http.MethodGet, "/limited", "", "")
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, time.Minute, ta.MR.TTL(key))

	ta.MR.FastForward(time.Minute)
	resp = ta.Request(t, http.MethodGet, "/limited", "", "")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRateLimit_LaterHitsDoNotExtendTheWindow(t *testing.T) {
	ta := testutil.NewTestApp(t)
	ta.App.Get("/limited",
		middleware.RateLimit(ta.RDB, "test4", 5, time.Minute),
		func(c fiber.Ctx) error {
			return c.JSON(response.Response{Code: 0, Message: "OK"})
		},
	)
	key := "ratelimit:test4:ip:0.0.0.0"
	ta.Request(t, http.MethodGet, "/limited", "", "")
	ta.MR.FastForward(40 * time.Second)
	ta.Request(t, http.MethodGet, "/limited", "", "")
	assert.Equal(t, 20*time.Second, ta.MR.TTL(key))
}
