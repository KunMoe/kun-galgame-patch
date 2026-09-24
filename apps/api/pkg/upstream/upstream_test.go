package upstream

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestByStatus(t *testing.T) {
	for status, want := range map[int]Kind{
		400: Internal, 401: Internal, 403: Internal,
		404: NotFound, 409: Conflict, 412: Conflict,
		422: Rejected, 429: RateLimited,
		500: Unavailable, 502: Unavailable, 503: Unavailable,
	} {
		if got := ByStatus(status); got != want {
			t.Errorf("ByStatus(%d) = %d, want %d", status, got, want)
		}
	}
}

func TestKindOfSeesThroughWrapping(t *testing.T) {
	e := &Error{Service: "catalog", Kind: RateLimited}
	if got := KindOf(fmt.Errorf("list works: %w", e)); got != RateLimited {
		t.Fatalf("KindOf(wrapped) = %d, want RateLimited", got)
	}
	if got := KindOf(errors.New("local")); got != Internal {
		t.Fatalf("KindOf(local) = %d, want Internal", got)
	}
}

func TestRetryAfter(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "30")
	if got := RetryAfter(h); got != 30*time.Second {
		t.Fatalf("RetryAfter = %v, want 30s", got)
	}
	h.Set("Retry-After", "Wed, 21 Oct 2026 07:28:00 GMT")
	if got := RetryAfter(h); got != 0 {
		t.Fatalf("RetryAfter(date) = %v, want 0", got)
	}
}

func TestReadBodyStopsAtMaxBody(t *testing.T) {
	b, err := ReadBody(strings.NewReader(strings.Repeat("x", MaxBody+10)))
	if err != nil || len(b) != MaxBody {
		t.Fatalf("ReadBody = %d bytes, %v; want %d", len(b), err, MaxBody)
	}
}

func TestIdempotencyKey(t *testing.T) {
	a := IdempotencyKey("mint", "7", `{"x":1}`)
	if a != IdempotencyKey("mint", "7", `{"x":1}`) {
		t.Fatal("same write, different key")
	}
	if a == IdempotencyKey("mint", "8", `{"x":1}`) || a == IdempotencyKey("mint7", `{"x":1}`) {
		t.Fatal("different writes share a key")
	}
	if len(a) > 255 {
		t.Fatalf("key is %d bytes; infra accepts at most 255", len(a))
	}
}
