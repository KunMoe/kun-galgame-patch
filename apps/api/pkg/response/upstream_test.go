package response_test

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/upstream"

	"github.com/gofiber/fiber/v3"
)

func answer(t *testing.T, err error, msg string) (*http.Response, response.Response) {
	t.Helper()
	app := fiber.New()
	app.Get("/x", func(c fiber.Ctx) error { return response.Upstream(c, err, msg) })
	res, rerr := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if rerr != nil {
		t.Fatal(rerr)
	}
	raw, _ := io.ReadAll(res.Body)
	var body response.Response
	if jerr := json.Unmarshal(raw, &body); jerr != nil {
		t.Fatalf("body %q: %v", raw, jerr)
	}
	return res, body
}

func TestUpstreamAnswers(t *testing.T) {
	const detail = "API key lacks scope catalog:read"
	for _, tc := range []struct {
		name       string
		err        error
		msg        string
		status     int
		code       int
		retryAfter string
	}{
		{"our credential is a 500, not the reader's 403",
			&upstream.Error{Service: "catalog", Kind: upstream.Internal, Status: 403, Code: "SCOPE_REQUIRED", Detail: detail},
			"没有权限", 500, 50000, ""},
		{"an unclassified local error is a 500", stderrors.New("boom"), "", 500, 50000, ""},
		{"catalog keeps the code its editor reads",
			fmt.Errorf("get work: %w", &upstream.Error{Service: "catalog", Kind: upstream.Unavailable, Status: 502}),
			"", 503, 50320, ""},
		{"an unnamed service is a generic 503", &upstream.Error{Service: "store", Kind: upstream.Unavailable}, "", 503, 50300, ""},
		{"rate limits pass their wait through",
			&upstream.Error{Service: "catalog", Kind: upstream.RateLimited, Status: 429, RetryAfter: 30 * time.Second},
			"", 429, 42900, "30"},
		{"not found takes the handler's wording", &upstream.Error{Service: "catalog", Kind: upstream.NotFound, Status: 404}, "游戏不存在", 404, 40400, ""},
		{"a reader's forbidden stays 403", &upstream.Error{Service: "community", Kind: upstream.Rejected, Status: 403}, "只能编辑自己的评论", 403, 40300, ""},
		{"a rejected 401 never becomes the logout code", &upstream.Error{Service: "catalog", Kind: upstream.Rejected, Status: 401}, "", 400, 40000, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, body := answer(t, tc.err, tc.msg)
			if res.StatusCode != tc.status || body.Code != tc.code {
				t.Errorf("got %d/%d, want %d/%d", res.StatusCode, body.Code, tc.status, tc.code)
			}
			if got := res.Header.Get("Retry-After"); got != tc.retryAfter {
				t.Errorf("Retry-After = %q, want %q", got, tc.retryAfter)
			}
			if strings.Contains(body.Message, detail) {
				t.Errorf("message %q leaks the upstream detail", body.Message)
			}
			if tc.msg != "" && (tc.code == 40400 || tc.code == 40300) && body.Message != tc.msg {
				t.Errorf("message = %q, want the handler's %q", body.Message, tc.msg)
			}
		})
	}
}
