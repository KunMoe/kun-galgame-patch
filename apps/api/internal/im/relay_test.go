package im

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// Shapes copied from infra's chat openapi (State) and its problem registry
// (apiv2/problem): the relay must not be tested against a body chat never sends.
const (
	stateBody = `{"object":"chat_state","last_update_seq":42,"unread_conversation_count":1,"unread_message_count":3,"request_count":2}`

	problemFmt = `{"type":"https://developer.nextmoe.dev/problems/%s","title":"t","status":%d,"detail":"","instance":"/v2/chat/state","code":"%s","request_id":"req_01J9ZQ3Y7H8K2M4N6P8R0T2V4W","errors":[]}`
)

type seenRequest struct {
	method, path, query, auth, contentType, idempotency, body string
}

func relayApp(t *testing.T, upstream http.HandlerFunc, token string) (*fiber.App, *seenRequest) {
	t.Helper()
	seen := &seenRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*seen = seenRequest{
			r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization"),
			r.Header.Get("Content-Type"), r.Header.Get("Idempotency-Key"), string(b),
		}
		upstream(w, r)
	}))
	t.Cleanup(srv.Close)
	return mount(NewRelay(srv.URL+"/"), token), seen
}

func mount(r *Relay, token string) *fiber.App {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		if token != "" {
			c.Locals("oauth_access_token", token)
		}
		return c.Next()
	})
	for _, m := range []string{fiber.MethodGet, fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch, fiber.MethodDelete} {
		app.Add([]string{m}, "/api/v1/im/*", r.Forward)
	}
	return app
}

type envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func do(t *testing.T, app *fiber.App, req *http.Request) (*http.Response, envelope) {
	t.Helper()
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("not an envelope: %s", raw)
	}
	return resp, env
}

func jsonRequest(method, path, body string) *http.Request {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func problemBody(typ string, status int, code string) string {
	return fmt.Sprintf(problemFmt, typ, status, code)
}

func TestRelayForwardsAsTheSessionUser(t *testing.T) {
	app, seen := relayApp(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(stateBody))
	}, "user-token")

	resp, env := do(t, app, jsonRequest(http.MethodGet, "/api/v1/im/state?content_limit=sfw&include_empty=true", ""))
	if resp.StatusCode != http.StatusOK || env.Code != 0 || string(env.Data) != stateBody {
		t.Fatalf("got %d %+v", resp.StatusCode, env)
	}
	if seen.path != "/v2/chat/state" || seen.query != "" || seen.auth != "Bearer user-token" {
		t.Fatalf("useApi's content_limit and include_empty must not reach chat: %+v", seen)
	}
	if seen.contentType != "" {
		t.Fatalf("a bodiless request carries no Content-Type: %+v", seen)
	}
}

func TestRelayPassesBodyQueryAndIdempotencyKey(t *testing.T) {
	app, seen := relayApp(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"object":"message","id":"9","seq":3}`))
	}, "user-token")

	req := jsonRequest(http.MethodPost, "/api/v1/im/conversations/12/messages?silent=1", `{"text":"hi","client_message_id":"c1"}`)
	req.Header.Set("Idempotency-Key", "k-1")
	resp, env := do(t, app, req)
	if resp.StatusCode != http.StatusCreated || env.Code != 0 {
		t.Fatalf("send: %d %+v", resp.StatusCode, env)
	}
	if seen.method != http.MethodPost || seen.path != "/v2/chat/conversations/12/messages" ||
		seen.query != "silent=1" || seen.body != `{"text":"hi","client_message_id":"c1"}` ||
		seen.contentType != "application/json" || seen.idempotency != "k-1" {
		t.Fatalf("upstream saw %+v", seen)
	}
}

func TestRelayPassesMultipartThrough(t *testing.T) {
	app, seen := relayApp(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"object":"chat_image","url":"u","type":"photo","image_hash":"ab","width":1,"height":1}`))
	}, "user-token")

	body := "--b\r\nContent-Disposition: form-data; name=\"file\"; filename=\"a.png\"\r\n\r\nPNG\r\n--b--\r\n"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/im/images", strings.NewReader(body))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=b")
	if resp, env := do(t, app, req); resp.StatusCode != http.StatusCreated || env.Code != 0 {
		t.Fatalf("upload: %d %+v", resp.StatusCode, env)
	}
	if seen.contentType != "multipart/form-data; boundary=b" || seen.body != body {
		t.Fatalf("upstream saw %+v", seen)
	}
}

func TestRelayAnswersNoContentWithAnEnvelope(t *testing.T) {
	app, _ := relayApp(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, "user-token")
	resp, env := do(t, app, jsonRequest(http.MethodPost, "/api/v1/im/conversations/1/accept", ""))
	if resp.StatusCode != http.StatusOK || env.Code != 0 {
		t.Fatalf("a 204 carries no body, so the page would read undefined: %d %+v", resp.StatusCode, env)
	}
}

func TestRelayTranslatesProblems(t *testing.T) {
	cases := []struct {
		typ      string
		status   int
		code     string
		wantCode int
		wantMsg  string
	}{
		{"platform/scope-required", http.StatusForbidden, "SCOPE_REQUIRED", 40313, "本次登录未授予私信权限，请重新登录后再试"},
		{"chat/chat-not-accepting", http.StatusForbidden, "CHAT_NOT_ACCEPTING", 40300, "对方暂不接受你的私信"},
		{"chat/chat-request-limit", http.StatusForbidden, "CHAT_REQUEST_LIMIT", 40300, "对方接受私信请求前，你只能发最多 3 条不带链接和图片的文字消息"},
		{"platform/invalid-credential", http.StatusUnauthorized, "INVALID_CREDENTIAL", 40101, "Session expired, please log in again"},
		{"platform/not-found", http.StatusNotFound, "NOT_FOUND", 40400, "会话或消息不存在"},
		{"platform/rate-limited", http.StatusTooManyRequests, "RATE_LIMITED", 42900, "操作太频繁，请稍后再试"},
		{"platform/internal-error", http.StatusInternalServerError, "INTERNAL_ERROR", 50322, "私信服务暂不可用，请稍后再试"},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			app, _ := relayApp(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/problem+json")
				if tc.status == http.StatusTooManyRequests {
					w.Header().Set("Retry-After", "7")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(problemBody(tc.typ, tc.status, tc.code)))
			}, "user-token")
			resp, env := do(t, app, jsonRequest(http.MethodGet, "/api/v1/im/state", ""))
			if resp.StatusCode != tc.status || env.Code != tc.wantCode || env.Message != tc.wantMsg {
				t.Fatalf("got %d %+v", resp.StatusCode, env)
			}
			if !strings.Contains(string(env.Data), `"code":"`+tc.code+`"`) {
				t.Fatalf("the problem rides along for the page to act on: %s", env.Data)
			}
			if tc.status == http.StatusTooManyRequests && resp.Header.Get("Retry-After") != "7" {
				t.Fatal("Retry-After must pass through")
			}
		})
	}
}

func TestRelayRefusesBeforeCallingChat(t *testing.T) {
	called := false
	upstream := func(http.ResponseWriter, *http.Request) { called = true }

	app, _ := relayApp(t, upstream, "")
	if resp, env := do(t, app, jsonRequest(http.MethodGet, "/api/v1/im/state", "")); resp.StatusCode != http.StatusUnauthorized || env.Code != 40100 {
		t.Fatalf("no token: %d %+v", resp.StatusCode, env)
	}
	app, _ = relayApp(t, upstream, "user-token")
	for _, path := range []string{
		"/api/v1/im/conversations/..%2F..%2Fadmin",
		"/api/v1/im/%2e%2e/%2e%2e/trust/callback",
		"/api/v1/im/state%3Fx",
	} {
		if resp, _ := do(t, app, jsonRequest(http.MethodGet, path, "")); resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s: %d", path, resp.StatusCode)
		}
	}
	if called {
		t.Fatal("chat must not be reached")
	}

	for name, base := range map[string]string{"unconfigured": "", "unreachable": "http://127.0.0.1:1"} {
		resp, env := do(t, mount(NewRelay(base), "t"), jsonRequest(http.MethodGet, "/api/v1/im/state", ""))
		if resp.StatusCode != http.StatusServiceUnavailable || env.Code != 50322 {
			t.Fatalf("%s: %d %+v", name, resp.StatusCode, env)
		}
	}
}
