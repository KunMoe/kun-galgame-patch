package common

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/pkg/catalogv2/catalogv2test"

	"github.com/gofiber/fiber/v3"
)

func TestCalendarContentLimitsFanOut(t *testing.T) {
	for _, tc := range []struct {
		cl   string
		want []string
	}{
		{"sfw", []string{"sfw"}},
		{"nsfw", []string{"nsfw"}},
		{"all", []string{"sfw", "nsfw"}},
		{"", []string{"sfw"}},
		{"garbage", []string{"sfw"}},
	} {
		got := calendarContentLimits(tc.cl)
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("calendarContentLimits(%q) = %v, want %v", tc.cl, got, tc.want)
		}
	}
}

func TestCalendarUpstreamFailureMapping(t *testing.T) {
	for _, tc := range []struct {
		name        string
		month       string
		answer      func(w http.ResponseWriter, r *http.Request)
		wantStatus  int
		wantBodyHas string
		wantCalls   int32
		retryAfter  string
	}{
		{
			name:  "a malformed month is the reader's error and never reaches catalog",
			month: "2026-7",
			answer: func(w http.ResponseWriter, r *http.Request) {
				catalogv2test.Problem(w, r, "INVALID_PARAMETER", "month must be YYYY-MM.", nil)
			},
			wantStatus: http.StatusBadRequest, wantBodyHas: "YYYY-MM", wantCalls: 0,
		},
		{
			name:  "catalog refusing a month moyu checked is moyu's bug",
			month: "2026-07",
			answer: func(w http.ResponseWriter, r *http.Request) {
				catalogv2test.Problem(w, r, "INVALID_PARAMETER", "month must be YYYY-MM.", nil)
			},
			wantStatus: http.StatusInternalServerError, wantBodyHas: `"code":50000`, wantCalls: 1,
		},
		{
			name:  "catalog falling over is an outage",
			month: "2026-07",
			answer: func(w http.ResponseWriter, r *http.Request) {
				catalogv2test.Problem(w, r, "INTERNAL_ERROR", "panic recovered", nil)
			},
			wantStatus: http.StatusServiceUnavailable, wantBodyHas: `"code":50320`, wantCalls: 1,
		},
		{
			name:  "a gateway page is an outage, not a bad request",
			month: "2026-07",
			answer: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte(`<html>502</html>`))
			},
			wantStatus: http.StatusServiceUnavailable, wantBodyHas: `"code":50320`, wantCalls: 1,
		},
		{
			name:  "a rate limit passes its wait through",
			month: "2026-07",
			answer: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "30")
				catalogv2test.Problem(w, r, "RATE_LIMITED", "slow down", nil)
			},
			wantStatus: http.StatusTooManyRequests, wantBodyHas: `"code":42900`, wantCalls: 1, retryAfter: "30",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				tc.answer(w, r)
			}))
			t.Cleanup(upstream.Close)

			h := NewHandler(nil, galgameClient.NewWithKey(upstream.URL, "nm_test_key"), nil, nil, nil)
			app := fiber.New()
			app.Get("/galgame/calendar", h.GetGalgameCalendar)

			req, _ := http.NewRequest(http.MethodGet, "http://localhost/galgame/calendar?month="+tc.month, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body=%s)", resp.StatusCode, tc.wantStatus, body)
			}
			if !strings.Contains(string(body), tc.wantBodyHas) {
				t.Errorf("body = %s, want it to carry %s", body, tc.wantBodyHas)
			}
			if strings.Contains(string(body), "month must be") {
				t.Errorf("catalog's English reached the reader: %s", body)
			}
			if got := calls.Load(); got != tc.wantCalls {
				t.Errorf("catalog was called %d times, want %d", got, tc.wantCalls)
			}
			if got := resp.Header.Get("Retry-After"); got != tc.retryAfter {
				t.Errorf("Retry-After = %q, want %q", got, tc.retryAfter)
			}
		})
	}
}
