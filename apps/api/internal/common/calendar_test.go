package common

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"

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
	const (
		badMonth   = `{"code":"INVALID_PARAMETER","status":400,"detail":"month must be YYYY-MM"}`
		serverDown = `{"code":"INTERNAL","status":500,"detail":"服务器内部错误"}`
	)

	for _, tc := range []struct {
		name           string
		upstreamStatus int
		upstreamBody   string
		wantStatus     int
		wantBodyHas    string
	}{
		{
			name:           "a malformed month is the CALLER's error, and says which",
			upstreamStatus: http.StatusBadRequest, upstreamBody: badMonth,
			wantStatus: http.StatusBadRequest, wantBodyHas: "month must be YYYY-MM",
		},
		{
			name:           "the registry falling over is still a 50000",
			upstreamStatus: http.StatusInternalServerError, upstreamBody: serverDown,
			wantStatus: http.StatusInternalServerError, wantBodyHas: `"code":50000`,
		},
		{
			name:           "a non-envelope failure is an outage, not a bad request",
			upstreamStatus: http.StatusBadGateway, upstreamBody: `<html>502</html>`,
			wantStatus: http.StatusInternalServerError, wantBodyHas: `"code":50000`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.upstreamStatus)
				_, _ = w.Write([]byte(tc.upstreamBody))
			}))
			t.Cleanup(upstream.Close)

			h := NewHandler(nil, galgameClient.NewWithKey(upstream.URL, "nm_test_key"), nil, nil, nil)
			app := fiber.New()
			app.Get("/galgame/calendar", h.GetGalgameCalendar)

			req, _ := http.NewRequest(http.MethodGet, "http://localhost/galgame/calendar?month=2026-7", nil)
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
		})
	}
}
