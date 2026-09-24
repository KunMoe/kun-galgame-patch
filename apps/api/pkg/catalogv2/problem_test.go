package catalogv2_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kun-galgame-patch-api/pkg/catalogv2"
	"kun-galgame-patch-api/pkg/catalogv2/catalogv2test"
	"kun-galgame-patch-api/pkg/upstream"
)

type lane int

const (
	appKeyRead lane = iota
	readerRead
	readerWrite
)

func (l lane) call(c *catalogv2.Client) error {
	ctx := context.Background()
	switch l {
	case appKeyRead:
		_, err := c.GetWork(ctx, 1, true)
		return err
	case readerRead:
		_, _, err := c.GetMyClaim(ctx, "tok", 7)
		return err
	default:
		_, err := c.CreateProposal(ctx, "tok", 42, catalogv2.EntityTypeWork, 7,
			map[string]any{"catalog.work.olang": "ja"}, "")
		return err
	}
}

func TestEveryFailureIsClassified(t *testing.T) {
	merged := map[string]any{"object": "work", "current_id": "6935"}
	for _, tc := range []struct {
		name   string
		lane   lane
		code   string
		extra  map[string]any
		want   upstream.Kind
		reauth bool
	}{
		{"moyu's key missing a scope is moyu's", appKeyRead, "SCOPE_REQUIRED", nil, upstream.Internal, false},
		{"moyu's key refused is moyu's", appKeyRead, "INVALID_CREDENTIAL", nil, upstream.Internal, false},
		{"a reader's token minted before a scope asks for a new sign-in", readerRead, "SCOPE_REQUIRED", nil, upstream.Rejected, true},
		{"a reader's token refused asks for a new sign-in", readerRead, "INVALID_CREDENTIAL", nil, upstream.Rejected, true},
		{"no token sent at all is moyu's", readerRead, "MISSING_CREDENTIAL", nil, upstream.Internal, false},
		{"an app key on a user face is moyu's", readerWrite, "USER_IDENTITY_REQUIRED", nil, upstream.Internal, false},
		{"a client without a site is moyu's", readerWrite, "SITE_NOT_BOUND", nil, upstream.Internal, false},
		{"a parameter catalog refuses is moyu's", appKeyRead, "UNKNOWN_INCLUDE", nil, upstream.Internal, false},
		{"a missing If-Match is moyu's", readerWrite, "PRECONDITION_REQUIRED", nil, upstream.Internal, false},
		{"a miss is a miss", appKeyRead, "NOT_FOUND", nil, upstream.NotFound, false},
		{"a merge is a miss with somewhere to go", appKeyRead, "ENTITY_MERGED", merged, upstream.NotFound, false},
		{"a field-level refusal of an edit is the reader's", readerWrite, "VALIDATION_FAILED", nil, upstream.Rejected, false},
		{"a field-level refusal of a read is moyu's", readerRead, "VALIDATION_FAILED", nil, upstream.Internal, false},
		{"another owner's claim is the reader's", readerWrite, "CLAIM_NOT_OWNED", nil, upstream.Rejected, false},
		{"another site's entity is the reader's", readerWrite, "TENANT_MISMATCH", nil, upstream.Rejected, false},
		{"a moderation standing the reader lacks is theirs", readerRead, "PERMISSION_REQUIRED", nil, upstream.Rejected, false},
		{"a state that moved is a conflict", readerWrite, "INVALID_STATE_TRANSITION", nil, upstream.Conflict, false},
		{"a stale ETag is a conflict", readerWrite, "PRECONDITION_FAILED", nil, upstream.Conflict, false},
		{"a reused key is a conflict", readerWrite, "IDEMPOTENCY_KEY_REUSED", nil, upstream.Conflict, false},
		{"a duplicate mint is a conflict", readerWrite, "DUPLICATE_SUSPECTS",
			map[string]any{"suspects": []any{map[string]any{"id": "9", "display_name": "夏日口袋"}}}, upstream.Conflict, false},
		{"a short window is a rate limit", readerWrite, "RATE_LIMITED", nil, upstream.RateLimited, false},
		{"a daily quota is a rate limit", appKeyRead, "QUOTA_EXCEEDED", nil, upstream.RateLimited, false},
		{"catalog's own bug is its outage", appKeyRead, "INTERNAL_ERROR", nil, upstream.Unavailable, false},
		{"an unbound dependency is an outage", readerRead, "SERVICE_UNAVAILABLE", nil, upstream.Unavailable, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				catalogv2test.Problem(w, r, tc.code, "detail from catalog", tc.extra)
			}))
			t.Cleanup(srv.Close)

			err := tc.lane.call(catalogv2.New(srv.URL, "nmk_live_x"))
			e, ok := upstream.As(err)
			if !ok {
				t.Fatalf("got %T %v, want *upstream.Error", err, err)
			}
			if e.Kind != tc.want || e.Code != tc.code || e.Service != "catalog" {
				t.Fatalf("kind %d code %q service %q, want kind %d", e.Kind, e.Code, e.Service, tc.want)
			}
			if e.Status != catalogv2test.Status(tc.code) || e.RequestID != catalogv2test.RequestID {
				t.Fatalf("status %d request_id %q", e.Status, e.RequestID)
			}
			if got := catalogv2.ReauthRequired(err); got != tc.reauth {
				t.Fatalf("ReauthRequired = %v, want %v", got, tc.reauth)
			}
			if p, ok := catalogv2.ProblemOf(err); !ok || p.Code != tc.code {
				t.Fatalf("the problem body is not reachable from the error: %v", err)
			}
		})
	}
}

func TestOpNamesTheRouteWithoutItsQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		catalogv2test.Problem(w, r, "NOT_FOUND", "No work with this id.", nil)
	}))
	t.Cleanup(srv.Close)
	_, err := catalogv2.New(srv.URL, "k").GetWork(context.Background(), 1, true)
	if e, _ := upstream.As(err); e == nil || e.Op != "GET /v2/catalog/works/1" {
		t.Fatalf("op = %+v", e)
	}
}

// A body that is not catalog's problem+json is not catalog talking: a proxy's
// 404 page means moyu is pointed at the wrong origin, and the forum spent months
// telling readers their game did not exist over exactly that.
func TestABodyCatalogDidNotWrite(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   upstream.Kind
	}{
		{"a proxy's 404 page is moyu pointed at the wrong origin", 404, "<html><body>404 page not found</body></html>", upstream.Internal},
		{"a house envelope is not catalog either", 404, `{"code":40400,"message":"not found"}`, upstream.Internal},
		{"a gateway page is an outage", 502, "<html>Bad Gateway</html>", upstream.Unavailable},
		{"a proxy's rate limit is still a rate limit", 429, "slow down", upstream.RateLimited},
		{"a bare 403 from a gateway is moyu's", 403, "Forbidden", upstream.Internal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			t.Cleanup(srv.Close)
			_, err := catalogv2.New(srv.URL, "k").GetWork(context.Background(), 1, true)
			e, ok := upstream.As(err)
			if !ok || e.Kind != tc.want {
				t.Fatalf("got %v, want kind %d", err, tc.want)
			}
			if catalogv2.IsNotFound(err) {
				t.Fatal("a body catalog did not write can never mean the entry is missing")
			}
			if _, isProblem := catalogv2.ProblemOf(err); isProblem {
				t.Fatal("no problem body was sent")
			}
		})
	}
}

func TestARateLimitCarriesItsWait(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "17")
		catalogv2test.Problem(w, r, "RATE_LIMITED", "The short-window rate limit was exceeded.", nil)
	}))
	t.Cleanup(srv.Close)
	_, err := catalogv2.New(srv.URL, "k").GetWork(context.Background(), 1, true)
	if e, _ := upstream.As(err); e == nil || e.RetryAfter != 17*time.Second {
		t.Fatalf("retry after = %+v", e)
	}
}

func TestTransportAndDecodeFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"object":"work","id":`)
	}))
	t.Cleanup(srv.Close)
	_, err := catalogv2.New(srv.URL, "k").GetWork(context.Background(), 1, true)
	if upstream.KindOf(err) != upstream.Internal {
		t.Fatalf("a body moyu cannot decode is moyu's: %v", err)
	}

	srv.Close()
	_, err = catalogv2.New(srv.URL, "k").GetWork(context.Background(), 1, true)
	if upstream.KindOf(err) != upstream.Unavailable {
		t.Fatalf("an unreachable catalog is an outage: %v", err)
	}
}

func TestAcceptNamesProblemJSON(t *testing.T) {
	var accept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept = r.Header.Get("Accept")
		_, _ = io.WriteString(w, `{"object":"work","id":"1"}`)
	}))
	t.Cleanup(srv.Close)
	if _, err := catalogv2.New(srv.URL, "k").GetWork(context.Background(), 1, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(accept, "application/problem+json") {
		t.Fatalf("Accept = %q", accept)
	}
}

// infra's cond.go treats only a bare * as the wildcard.
func TestWithdrawProposalFallsBackToTheBareWildcard(t *testing.T) {
	var match string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			match = r.Header.Get("If-Match")
		}
		_, _ = io.WriteString(w, `{"id":"32","state":"withdrawn"}`)
	}))
	t.Cleanup(srv.Close)
	if _, err := catalogv2.New(srv.URL, "k").WithdrawProposal(context.Background(), "tok", 32); err != nil {
		t.Fatal(err)
	}
	if match != "*" {
		t.Fatalf("If-Match = %q, want *", match)
	}
}
