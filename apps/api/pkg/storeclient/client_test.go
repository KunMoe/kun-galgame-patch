package storeclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kun-galgame-patch-api/pkg/upstream"
)

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(Config{BaseURL: srv.URL, APIKey: "nmk_live_test"})
}

func TestPurchaseLinks_NoCampaign(t *testing.T) {
	var gotPath, gotAuth string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"object":"purchase_links","product_id":"RJ297925",
			"purchase_url":"https://s.example/s/abc","coupon_url":null,"campaign":null}`))
	})

	links, err := c.PurchaseLinks(context.Background(), "RJ297925")
	if err != nil {
		t.Fatalf("PurchaseLinks: %v", err)
	}
	if gotPath != "/v2/store/purchase-links/RJ297925" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAuth != "Bearer nmk_live_test" {
		t.Errorf("auth = %q", gotAuth)
	}
	if links.PurchaseURL != "https://s.example/s/abc" {
		t.Errorf("purchase_url = %q", links.PurchaseURL)
	}
	if links.CouponURL != "" || links.Campaign != nil {
		t.Errorf("no campaign should leave both empty, got %q / %+v", links.CouponURL, links.Campaign)
	}
}

func TestPurchaseLinks_Campaign(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"object":"purchase_links","product_id":"VJ013550",
			"purchase_url":"https://s.example/s/abc","coupon_url":"https://s.example/s/def",
			"campaign":{"object":"campaign","id":"7","name":"夏日特惠"}}`))
	})

	links, err := c.PurchaseLinks(context.Background(), "VJ013550")
	if err != nil {
		t.Fatalf("PurchaseLinks: %v", err)
	}
	if links.CouponURL != "https://s.example/s/def" {
		t.Errorf("coupon_url = %q", links.CouponURL)
	}
	// id is a string in v2, and a client that re-typed it as an integer would
	// fail the whole record rather than just that field.
	if links.Campaign == nil || links.Campaign.ID != "7" || links.Campaign.Name != "夏日特惠" {
		t.Errorf("campaign = %+v", links.Campaign)
	}
}

// problem mirrors infra's apiv2 problem.New body for a registry code.
func problem(status int, code, domain, detail string) string {
	b, _ := json.Marshal(map[string]any{
		"type":       "https://developer.nextmoe.dev/problems/" + domain + "/" + strings.ToLower(strings.ReplaceAll(code, "_", "-")),
		"title":      "t",
		"status":     status,
		"detail":     detail,
		"instance":   "/v2/store/purchase-links/RJ297925",
		"code":       code,
		"request_id": "req_01J8ZQ5W8Y3N2V6T4R7K9M0P1S",
		"errors":     []any{},
	})
	return string(b)
}

func TestPurchaseLinks_ProblemCodes(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       string
		retryAfter string
		want       error
		kind       upstream.Kind
		wait       time.Duration
	}{
		// 403 is also what a missing scope answers, so only the code separates a
		// refusal that retrying can fix from one it never will.
		{"lifetime link quota", 403, problem(403, "STORE_QUOTA_EXCEEDED", "store", ""), "", ErrQuotaExceeded, upstream.Internal, 0},
		{"shortener down", 502, problem(502, "STORE_LINK_UNAVAILABLE", "store", ""), "", ErrUnavailable, upstream.Unavailable, 0},
		{"face off", 503, problem(503, "SERVICE_UNAVAILABLE", "platform", ""), "", ErrUnavailable, upstream.Unavailable, 0},
		{"bad scope", 403, problem(403, "SCOPE_REQUIRED", "platform", "store:read"), "", ErrUnauthorized, upstream.Internal, 0},
		{"no key", 401, problem(401, "MISSING_CREDENTIAL", "platform", ""), "", ErrUnauthorized, upstream.Internal, 0},
		{"rejected id", 422, problem(422, "VALIDATION_FAILED", "platform", ""), "", ErrInvalidProduct, upstream.Rejected, 0},
		{"rate limited", 429, problem(429, "RATE_LIMITED", "platform", "Short-window rate limit exceeded."), "42", ErrRateLimited, upstream.RateLimited, 42 * time.Second},
		{"daily quota spent", 429, problem(429, "QUOTA_EXCEEDED", "platform", "Daily quota exceeded."), "36000", ErrRateLimited, upstream.RateLimited, 10 * time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/problem+json")
				w.Header().Set("X-Request-ID", "req_01J8ZQ5W8Y3N2V6T4R7K9M0P1S")
				if tc.retryAfter != "" {
					w.Header().Set("Retry-After", tc.retryAfter)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})
			_, err := c.PurchaseLinks(context.Background(), "RJ297925")
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
			e, ok := upstream.As(err)
			if !ok || e.Kind != tc.kind || e.RetryAfter != tc.wait {
				t.Errorf("upstream error = %+v, want kind %d wait %v", e, tc.kind, tc.wait)
			}
		})
	}
}

func TestPurchaseLinks_RejectsMalformedProductLocally(t *testing.T) {
	called := false
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	if _, err := c.PurchaseLinks(context.Background(), "RJ12345"); !errors.Is(err, ErrInvalidProduct) {
		t.Errorf("err = %v, want ErrInvalidProduct", err)
	}
	if called {
		t.Error("a workno the face's own pattern rejects must not cost a round trip")
	}
}

func TestNotConfigured(t *testing.T) {
	if _, err := New(Config{BaseURL: "https://example.test"}).PurchaseLinks(context.Background(), "RJ297925"); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("a base URL with no key must be ErrNotConfigured, got %v", err)
	}
	var nilClient *Client
	if nilClient.Configured() {
		t.Error("nil client must report unconfigured")
	}
}

func TestOriginTrimsVersionSuffix(t *testing.T) {
	for _, in := range []string{"http://catalog:9281", "http://catalog:9281/", "http://catalog:9281/v2"} {
		if got := origin(in); got != "http://catalog:9281" {
			t.Errorf("origin(%q) = %q", in, got)
		}
	}
}

// Catalog lists every DLsite edition a work was ever sold as, and the first is
// regularly an RE id — DLsite's English store, which the affiliate path does not
// serve. Position therefore decides nothing; shape does.
func TestPickProductID(t *testing.T) {
	cases := []struct {
		name string
		ids  []string
		want string
	}{
		{"english edition first", []string{"RE144678", "RJ144678", "RJ205284"}, "RJ144678"},
		{"several commercial editions", []string{"VJ013975", "VJ014001"}, "VJ013975"},
		{"nothing buyable", []string{"RE307838"}, ""},
		{"empty", nil, ""},
		{"too short for the face", []string{"RJ12345", "VJ009599"}, "VJ009599"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PickProductID(tc.ids); got != tc.want {
				t.Errorf("PickProductID(%v) = %q, want %q", tc.ids, got, tc.want)
			}
		})
	}
}
