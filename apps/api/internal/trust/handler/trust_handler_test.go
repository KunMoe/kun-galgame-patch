package handler

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/internal/trust/dto"
	"kun-galgame-patch-api/internal/trust/service"
	"kun-galgame-patch-api/pkg/trustclient"

	"github.com/gofiber/fiber/v3"
)

type fakeEnforcer struct {
	got []dto.TrustCallback
	err error
}

func (f *fakeEnforcer) Apply(_ context.Context, cb dto.TrustCallback) error {
	f.got = append(f.got, cb)
	return f.err
}

const secret = "cb-secret"

func trustServer(t *testing.T, status int, body string) *service.TrustService {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return service.NewTrustService(trustclient.New(trustclient.Config{
		BaseURL: srv.URL, ClientID: "moyu", ClientSecret: "secret",
	}), "moyu")
}

func do(t *testing.T, app *fiber.App, req *http.Request) (int, int) {
	t.Helper()
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	var body struct {
		Code int `json:"code"`
	}
	_ = json.Unmarshal(raw, &body)
	return res.StatusCode, body.Code
}

func callback(body []byte, sign string) *http.Request {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	req := httptest.NewRequest(http.MethodPost, "/cb", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Trust-Timestamp", ts)
	req.Header.Set("X-Trust-Signature", trustclient.SignPayload(sign, ts, body))
	return req
}

func TestCallback(t *testing.T) {
	body := []byte(`{"disposition_id":11,"subject_kind":"patch_resource","subject_id":"7","action":2,"reason_code":"spam"}`)

	t.Run("applies a signed disposition", func(t *testing.T) {
		enf := &fakeEnforcer{}
		app := fiber.New()
		app.Post("/cb", middleware.BodyLimit(16*1024), NewTrustHandler(nil, enf, secret).Callback)
		if status, _ := do(t, app, callback(body, secret)); status != 200 {
			t.Fatalf("status = %d", status)
		}
		if len(enf.got) != 1 || enf.got[0].DispositionID != 11 || enf.got[0].Action != 2 {
			t.Fatalf("applied %+v", enf.got)
		}
	})

	t.Run("refuses a forged one", func(t *testing.T) {
		enf := &fakeEnforcer{}
		app := fiber.New()
		app.Post("/cb", NewTrustHandler(nil, enf, secret).Callback)
		if status, _ := do(t, app, callback(body, "other")); status != 401 || len(enf.got) != 0 {
			t.Fatalf("status = %d, applied %d", status, len(enf.got))
		}
	})

	t.Run("a failed enforcement asks infra to redeliver", func(t *testing.T) {
		app := fiber.New()
		app.Post("/cb", NewTrustHandler(nil, &fakeEnforcer{err: stderrors.New("db down")}, secret).Callback)
		if status, _ := do(t, app, callback(body, secret)); status != 500 {
			t.Fatalf("status = %d, want 500", status)
		}
	})

	t.Run("an oversized body never reaches the verifier", func(t *testing.T) {
		enf := &fakeEnforcer{}
		app := fiber.New()
		app.Post("/cb", middleware.BodyLimit(16*1024), NewTrustHandler(nil, enf, secret).Callback)
		big := []byte(`{"reason_code":"` + strings.Repeat("x", 20*1024) + `"}`)
		if status, _ := do(t, app, callback(big, secret)); status != 413 || len(enf.got) != 0 {
			t.Fatalf("status = %d, applied %d", status, len(enf.got))
		}
	})
}

func TestAdminFailuresNeverLogTheModeratorOut(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   int
		code   int
	}{
		{"trust rejects the token", 401, `{"code":10003,"message":"令牌已过期，请重新登录"}`, 500, 50000},
		{"moyu's client is not bound", 403, `{"code":5,"message":"site-scoped moderator's client is not bound to a site"}`, 500, 50000},
		{"the moderator lacks the queue permission", 403, `{"code":5,"message":"访问被拒绝"}`, 403, 40300},
		{"trust is down", 502, `<html>Bad Gateway</html>`, 503, 50300},
		{"no such item", 404, `{"code":4,"message":"资源不存在"}`, 404, 40400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewTrustHandler(trustServer(t, tc.status, tc.body), &fakeEnforcer{}, secret)
			app := fiber.New()
			app.Get("/items/:id", h.GetReviewItem)
			status, code := do(t, app, httptest.NewRequest(http.MethodGet, "/items/3", nil))
			if status != tc.want || code != tc.code {
				t.Fatalf("got %d/%d, want %d/%d", status, code, tc.want, tc.code)
			}
		})
	}
}

func TestReasons(t *testing.T) {
	t.Run("a credential failure is a 500, not the seeded list", func(t *testing.T) {
		h := NewTrustHandler(trustServer(t, 401, `{"code":10001,"message":"未授权，请先登录"}`), &fakeEnforcer{}, secret)
		app := fiber.New()
		app.Get("/reasons", h.GetReasons)
		if status, code := do(t, app, httptest.NewRequest(http.MethodGet, "/reasons", nil)); status != 500 || code != 50000 {
			t.Fatalf("got %d/%d", status, code)
		}
	})
	t.Run("an outage serves the seeded list", func(t *testing.T) {
		h := NewTrustHandler(trustServer(t, 503, `<html>unavailable</html>`), &fakeEnforcer{}, secret)
		app := fiber.New()
		app.Get("/reasons", h.GetReasons)
		if status, code := do(t, app, httptest.NewRequest(http.MethodGet, "/reasons", nil)); status != 200 || code != 0 {
			t.Fatalf("got %d/%d", status, code)
		}
	})
}
