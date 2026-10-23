package testutil

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/response"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
)

type TestApp struct {
	App *fiber.App
	RDB *redis.Client
	MR  *miniredis.Miniredis
}

func NewTestApp(t *testing.T) *TestApp {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mr.Close() })

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	app := fiber.New(fiber.Config{
		Immutable: true,
		ErrorHandler: func(c fiber.Ctx, err error) error {
			return c.Status(500).JSON(response.Response{
				Code:    50000,
				Message: err.Error(),
			})
		},
	})

	return &TestApp{App: app, RDB: rdb, MR: mr}
}

func (ta *TestApp) CreateTestSession(t *testing.T, userID int, roles ...string) string {
	t.Helper()
	sessionID := fmt.Sprintf("test-session-%d-%d", userID, time.Now().UnixNano())
	session := middleware.SessionData{
		UserInfo: middleware.UserInfo{
			ID:  userID,
			Sub: fmt.Sprintf("test-sub-%d", userID),
		},
		OAuthAccessToken: fakeJWTWithRoles(roles),
	}
	data, _ := json.Marshal(session)
	ta.RDB.Set(context.Background(), middleware.SessionPrefix+sessionID, data, middleware.SessionTTL)
	return sessionID
}

func (ta *TestApp) CreateTestSessionSiteRoles(t *testing.T, userID int, roles, siteRoles []string) string {
	t.Helper()
	sessionID := fmt.Sprintf("test-session-%d-%d", userID, time.Now().UnixNano())
	session := middleware.SessionData{
		UserInfo: middleware.UserInfo{
			ID:  userID,
			Sub: fmt.Sprintf("test-sub-%d", userID),
		},
		OAuthAccessToken: fakeJWTWithClaims(roles, siteRoles),
	}
	data, _ := json.Marshal(session)
	ta.RDB.Set(context.Background(), middleware.SessionPrefix+sessionID, data, middleware.SessionTTL)
	return sessionID
}

func fakeJWTWithRoles(roles []string) string {
	return fakeJWTWithClaims(roles, nil)
}

func fakeJWTWithClaims(roles, siteRoles []string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claims := map[string]any{"roles": roles}
	if siteRoles != nil {
		claims["site_roles"] = siteRoles
	}
	payloadJSON, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	return header + "." + payload + ".sig"
}

func (ta *TestApp) Request(t *testing.T, method, path string, body string, sessionID string) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	if sessionID != "" {
		req.AddCookie(&http.Cookie{
			Name:  middleware.SessionCookieName,
			Value: sessionID,
		})
	}

	resp, err := ta.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func ParseResponse(t *testing.T, resp *http.Response) response.Response {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var r response.Response
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("failed to parse response: %s, body: %s", err, string(body))
	}
	return r
}

func ReadBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	return string(body)
}
