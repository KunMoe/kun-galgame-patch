package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/internal/testutil"
	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuth_NoSession(t *testing.T) {
	ta := testutil.NewTestApp(t)
	oauthCfg := config.OAuthConfig{}

	ta.App.Get("/protected", middleware.Auth(ta.RDB, oauthCfg), func(c *fiber.Ctx) error {
		return c.JSON(response.Response{Code: 0, Message: "OK"})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	resp, err := ta.App.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	r := testutil.ParseResponse(t, resp)
	assert.Equal(t, 40100, r.Code)
}

func TestAuth_ValidSession(t *testing.T) {
	ta := testutil.NewTestApp(t)
	oauthCfg := config.OAuthConfig{}

	ta.App.Get("/protected", middleware.Auth(ta.RDB, oauthCfg), func(c *fiber.Ctx) error {
		user := middleware.MustGetUser(c)
		return c.JSON(response.Response{Code: 0, Message: "OK", Data: user.UID})
	})

	sessionID := ta.CreateTestSession(t, 1, "user")
	resp := ta.Request(t, http.MethodGet, "/protected", "", sessionID)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := testutil.ParseResponse(t, resp)
	assert.Equal(t, 0, r.Code)
}

func TestAuth_ExpiredSession(t *testing.T) {
	ta := testutil.NewTestApp(t)
	oauthCfg := config.OAuthConfig{}

	ta.App.Get("/protected", middleware.Auth(ta.RDB, oauthCfg), func(c *fiber.Ctx) error {
		return c.JSON(response.Response{Code: 0, Message: "OK"})
	})

	resp := ta.Request(t, http.MethodGet, "/protected", "", "nonexistent-session")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	r := testutil.ParseResponse(t, resp)
	assert.Equal(t, 40101, r.Code)
}

func TestOptionalAuth_NoSession(t *testing.T) {
	ta := testutil.NewTestApp(t)
	oauthCfg := config.OAuthConfig{}

	ta.App.Get("/optional", middleware.OptionalAuth(ta.RDB, oauthCfg), func(c *fiber.Ctx) error {
		uid := middleware.GetUID(c)
		return c.JSON(response.Response{Code: 0, Data: uid})
	})

	req := httptest.NewRequest(http.MethodGet, "/optional", nil)
	resp, err := ta.App.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := testutil.ParseResponse(t, resp)
	assert.Equal(t, 0, r.Code)
	assert.Equal(t, float64(0), r.Data)
}

func TestOptionalAuth_ValidSession(t *testing.T) {
	ta := testutil.NewTestApp(t)
	oauthCfg := config.OAuthConfig{}

	ta.App.Get("/optional", middleware.OptionalAuth(ta.RDB, oauthCfg), func(c *fiber.Ctx) error {
		uid := middleware.GetUID(c)
		return c.JSON(response.Response{Code: 0, Data: uid})
	})

	sessionID := ta.CreateTestSession(t, 42, "user")
	resp := ta.Request(t, http.MethodGet, "/optional", "", sessionID)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := testutil.ParseResponse(t, resp)
	assert.Equal(t, float64(42), r.Data)
}

func TestRequireRole_InsufficientRole(t *testing.T) {
	ta := testutil.NewTestApp(t)
	oauthCfg := config.OAuthConfig{}

	ta.App.Get("/admin",
		middleware.Auth(ta.RDB, oauthCfg),
		middleware.RequireRole("admin"),
		func(c *fiber.Ctx) error {
			return c.JSON(response.Response{Code: 0, Message: "admin"})
		},
	)

	sessionID := ta.CreateTestSession(t, 1, "user")
	resp := ta.Request(t, http.MethodGet, "/admin", "", sessionID)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	r := testutil.ParseResponse(t, resp)
	assert.Equal(t, 40300, r.Code)
}

func TestRequireRole_SufficientRole(t *testing.T) {
	ta := testutil.NewTestApp(t)
	oauthCfg := config.OAuthConfig{}

	ta.App.Get("/admin",
		middleware.Auth(ta.RDB, oauthCfg),
		middleware.RequireRole("admin", "moderator"),
		func(c *fiber.Ctx) error {
			return c.JSON(response.Response{Code: 0, Message: "admin"})
		},
	)

	sessionID := ta.CreateTestSession(t, 1, "moderator")
	resp := ta.Request(t, http.MethodGet, "/admin", "", sessionID)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := testutil.ParseResponse(t, resp)
	assert.Equal(t, 0, r.Code)
}

func TestCreateSession_And_DestroySession(t *testing.T) {
	ta := testutil.NewTestApp(t)

	var capturedCookie string

	ta.App.Post("/login", func(c *fiber.Ctx) error {
		session := &middleware.SessionData{
			UserInfo: middleware.UserInfo{UID: 99, Sub: "test-sub"},
		}
		return middleware.CreateSession(c, ta.RDB, session)
	})

	ta.App.Post("/logout", func(c *fiber.Ctx) error {
		return middleware.DestroySession(c, ta.RDB)
	})

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	resp, err := ta.App.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	for _, cookie := range resp.Cookies() {
		if cookie.Name == middleware.SessionCookieName {
			capturedCookie = cookie.Value
		}
	}
	assert.NotEmpty(t, capturedCookie)

	val, err := ta.RDB.Get(context.Background(), middleware.SessionPrefix+capturedCookie).Result()
	require.NoError(t, err)
	assert.NotEmpty(t, val)

	var session middleware.SessionData
	json.Unmarshal([]byte(val), &session)
	assert.Equal(t, 99, session.UID)

	logoutReq := httptest.NewRequest(http.MethodPost, "/logout", nil)
	logoutReq.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: capturedCookie})
	logoutResp, err := ta.App.Test(logoutReq, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, logoutResp.StatusCode)

	_, err = ta.RDB.Get(context.Background(), middleware.SessionPrefix+capturedCookie).Result()
	assert.Error(t, err)
}

func TestGetUser_Helpers(t *testing.T) {
	ta := testutil.NewTestApp(t)
	oauthCfg := config.OAuthConfig{}

	ta.App.Get("/helpers", middleware.Auth(ta.RDB, oauthCfg), func(c *fiber.Ctx) error {
		user := middleware.GetUser(c)
		must := middleware.MustGetUser(c)
		uid := middleware.GetUID(c)
		roles := middleware.GetRoles(c)
		isMod := middleware.HasRole(c, "moderator")

		return c.JSON(map[string]any{
			"user_nil": user == nil,
			"must_nil": must == nil,
			"uid":      uid,
			"roles":    roles,
			"is_mod":   isMod,
		})
	})

	sessionID := ta.CreateTestSession(t, 7, "moderator")
	resp := ta.Request(t, http.MethodGet, "/helpers", "", sessionID)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body := testutil.ReadBody(t, resp)
	var result map[string]any
	json.Unmarshal([]byte(body), &result)
	assert.Equal(t, false, result["user_nil"])
	assert.Equal(t, false, result["must_nil"])
	assert.Equal(t, float64(7), result["uid"])
	assert.Equal(t, true, result["is_mod"])
}
