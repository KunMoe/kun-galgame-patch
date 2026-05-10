// Package testutil provides helpers for setting up integration tests.
// It creates a Fiber app with real route wiring, miniredis for Redis,
// and a real or mocked PostgreSQL database.
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
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

// TestApp holds a Fiber app and test dependencies
type TestApp struct {
	App *fiber.App
	RDB *redis.Client
	MR  *miniredis.Miniredis
}

// NewTestApp creates a minimal Fiber app with miniredis
func NewTestApp(t *testing.T) *TestApp {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mr.Close() })

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(500).JSON(response.Response{
				Code:    50000,
				Message: err.Error(),
			})
		},
	})

	return &TestApp{App: app, RDB: rdb, MR: mr}
}

// CreateTestSession creates a Redis session and returns the cookie value.
// roles is the OAuth roles set; pass e.g. "admin" / "moderator" to grant
// privileged access. The fake access_token is a JWT-shaped string with the
// given roles in its claims, so middleware.GetRoles works in tests too.
func (ta *TestApp) CreateTestSession(t *testing.T, uid int, roles ...string) string {
	t.Helper()
	sessionID := fmt.Sprintf("test-session-%d-%d", uid, time.Now().UnixNano())
	session := middleware.SessionData{
		UserInfo: middleware.UserInfo{
			UID: uid,
			Sub: fmt.Sprintf("test-sub-%d", uid),
		},
		OAuthAccessToken: fakeJWTWithRoles(roles),
	}
	data, _ := json.Marshal(session)
	ta.RDB.Set(context.Background(), middleware.SessionPrefix+sessionID, data, middleware.SessionTTL)
	return sessionID
}

// fakeJWTWithRoles builds a header.payload.sig JWT-shaped string whose
// payload encodes {"roles": [...]}. Signature is dummy; middleware decodes
// without verifying.
func fakeJWTWithRoles(roles []string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadJSON, _ := json.Marshal(map[string]any{"roles": roles})
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	return header + "." + payload + ".sig"
}

// Request sends an HTTP request to the Fiber app and returns the response
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

	resp, err := ta.App.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// ParseResponse reads and parses the JSON response body
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

// PaginatedResponseBody matches the on-wire shape { code, message, data: { items, total } }.
type PaginatedResponseBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Items json.RawMessage `json:"items"`
		Total int64           `json:"total"`
	} `json:"data"`
}

// ParsePaginatedResponse reads and parses the paginated JSON response body.
func ParsePaginatedResponse(t *testing.T, resp *http.Response) PaginatedResponseBody {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var r PaginatedResponseBody
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("failed to parse paginated response: %s, body: %s", err, string(body))
	}
	return r
}

// ReadBody reads the raw response body as string
func ReadBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	return string(body)
}
