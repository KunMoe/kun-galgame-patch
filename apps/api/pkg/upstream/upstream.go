// Package upstream is the one vocabulary for a failed call to a NextMoe
// service. The services answer in three dialects (problem+json, the house
// {code, message} envelope, and bare statuses from a proxy in front of them),
// and only a client knows which of its upstream's codes mean "moyu's request or
// credential is wrong" and which mean "the reader's is". So each client decides
// the Kind, and a handler maps the Kind to a response without reading statuses.
package upstream

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Kind uint8

const (
	// Internal is moyu's own request, credential or decoding going wrong. It is
	// a logged 500: the forum answered these as 404s and 4xx for months, which
	// told readers their game did not exist when moyu's API key had lost a scope.
	Internal Kind = iota
	Unavailable
	RateLimited
	NotFound
	Conflict
	Rejected
)

type Error struct {
	Service    string
	Op         string
	Kind       Kind
	Status     int
	Code       string
	Detail     string
	RequestID  string
	RetryAfter time.Duration
	Cause      error
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString(e.Service)
	if e.Op != "" {
		b.WriteString(" " + e.Op)
	}
	if e.Status != 0 {
		fmt.Fprintf(&b, ": %d", e.Status)
	}
	if e.Code != "" {
		b.WriteString(" " + e.Code)
	}
	if e.Detail != "" {
		b.WriteString(": " + e.Detail)
	}
	if e.Cause != nil {
		fmt.Fprintf(&b, ": %v", e.Cause)
	}
	return b.String()
}

func (e *Error) Unwrap() error { return e.Cause }

func As(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}

// KindOf answers Internal for any error a client did not classify: an error
// that never crossed the wire is moyu's own.
func KindOf(err error) Kind {
	if e, ok := As(err); ok {
		return e.Kind
	}
	return Internal
}

// ByStatus is the fallback for an answer in the service's own dialect whose
// code the client does not recognise. It must not be used for a body the
// service did not write: a proxy's 404 page means moyu is pointed at the wrong
// origin, which is Internal.
func ByStatus(status int) Kind {
	switch {
	case status == http.StatusNotFound:
		return NotFound
	case status == http.StatusConflict, status == http.StatusPreconditionFailed:
		return Conflict
	case status == http.StatusUnprocessableEntity:
		return Rejected
	case status == http.StatusTooManyRequests:
		return RateLimited
	case status >= 500:
		return Unavailable
	default:
		return Internal
	}
}

func Transport(service, op string, err error) *Error {
	return &Error{Service: service, Op: op, Kind: Unavailable, Cause: err}
}

func RetryAfter(h http.Header) time.Duration {
	secs, err := strconv.Atoi(strings.TrimSpace(h.Get("Retry-After")))
	if err != nil || secs <= 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}

const MaxBody = 16 << 20

// ReadBody reads at most MaxBody bytes; a longer answer fails to decode, which
// is Internal, instead of holding the whole of it in memory.
func ReadBody(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, MaxBody))
}

// IdempotencyKey derives a write's key from what the write is. Infra scopes a
// key to the credential, method and path, and replays the first answer to a
// repeat for 24h, so a reader who retries a timed-out submit gets that answer
// back instead of a second record.
func IdempotencyKey(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return "moyu-" + hex.EncodeToString(h.Sum(nil))[:40]
}
