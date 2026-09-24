package catalogv2

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"kun-galgame-patch-api/pkg/upstream"
)

const service = "catalog"

// Problem is catalog's problem+json body, as nextmoe-infra's
// apps/api/internal/platform/apiv2/problem.Problem writes it.
type Problem struct {
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Status    int            `json:"status"`
	Detail    string         `json:"detail"`
	Instance  string         `json:"instance"`
	Code      string         `json:"code"`
	RequestID string         `json:"request_id"`
	Errors    []ProblemField `json:"errors"`
	Object    string         `json:"object"`
	CurrentID string         `json:"current_id"`
	Suspects  []Suspect      `json:"suspects"`
}

// Exactly one of Pointer / Parameter / Header is set. The pointer prefix is not
// consistent — a validation failure emits "/<key>", an unknown or locked field
// emits "/patch/<key>" — which is why the browser side parses it rather than
// this one.
type ProblemField struct {
	Pointer   string `json:"pointer,omitempty"`
	Parameter string `json:"parameter,omitempty"`
	Header    string `json:"header,omitempty"`
	Reason    string `json:"reason"`
	Detail    string `json:"detail"`
}

type Suspect struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

func (p *Problem) Error() string {
	if p.Detail != "" {
		return p.Detail
	}
	return p.Title
}

const (
	CodeScopeRequired     = "SCOPE_REQUIRED"
	CodeInvalidCredential = "INVALID_CREDENTIAL"
	CodeValidationFailed  = "VALIDATION_FAILED"
	CodeEntityMerged      = "ENTITY_MERGED"
	CodeDuplicateSuspects = "DUPLICATE_SUSPECTS"
)

// Every code in catalog's closed registry
// (nextmoe-infra apps/api/internal/platform/apiv2/problem/registry.go), by
// whose fault it is. A 400 is moyu's: what a reader sends is validated before
// it is forwarded, so a parameter catalog refuses is a request this site built
// wrong, and passing it back as the reader's 400 hid that.
var problemKinds = map[string]upstream.Kind{
	"MALFORMED_BODY":                  upstream.Internal,
	"INVALID_PARAMETER":               upstream.Internal,
	"UNKNOWN_ENUM_VALUE":              upstream.Internal,
	"MUTUALLY_EXCLUSIVE_PARAMETERS":   upstream.Internal,
	"LIMIT_TOO_LARGE":                 upstream.Internal,
	"TOO_MANY_IDS":                    upstream.Internal,
	"INVALID_CURSOR":                  upstream.Internal,
	"UNKNOWN_INCLUDE":                 upstream.Internal,
	"UNKNOWN_FIELD":                   upstream.Internal,
	"UNKNOWN_SORT":                    upstream.Internal,
	"UNKNOWN_FACET":                   upstream.Internal,
	"MISSING_CREDENTIAL":              upstream.Internal,
	CodeInvalidCredential:             upstream.Internal,
	CodeScopeRequired:                 upstream.Internal,
	"FIRST_PARTY_ONLY":                upstream.Internal,
	"NOT_FOUND":                       upstream.NotFound,
	"METHOD_NOT_ALLOWED":              upstream.Internal,
	"IDEMPOTENCY_KEY_REUSED":          upstream.Conflict,
	"IDEMPOTENCY_REQUEST_IN_PROGRESS": upstream.Conflict,
	"GONE":                            upstream.Internal,
	"PRECONDITION_FAILED":             upstream.Conflict,
	"PAYLOAD_TOO_LARGE":               upstream.Rejected,
	"UNSUPPORTED_MEDIA_TYPE":          upstream.Internal,
	CodeValidationFailed:              upstream.Rejected,
	"PRECONDITION_REQUIRED":           upstream.Internal,
	"RATE_LIMITED":                    upstream.RateLimited,
	"QUOTA_EXCEEDED":                  upstream.RateLimited,
	"INTERNAL_ERROR":                  upstream.Unavailable,
	"SERVICE_UNAVAILABLE":             upstream.Unavailable,
	CodeEntityMerged:                  upstream.NotFound,
	"USER_IDENTITY_REQUIRED":          upstream.Internal,
	"SITE_NOT_BOUND":                  upstream.Internal,
	"RELEASE_CREATION_DISABLED":       upstream.Rejected,
	"ALREADY_EXISTS":                  upstream.Conflict,
	CodeDuplicateSuspects:             upstream.Conflict,
	"INVALID_STATE_TRANSITION":        upstream.Conflict,
	"CLAIM_NOT_OWNED":                 upstream.Rejected,
	"PERMISSION_REQUIRED":             upstream.Rejected,
	"TENANT_MISMATCH":                 upstream.Rejected,
	"DECISION_ALREADY_MADE":           upstream.Conflict,
	"SOURCE_NOT_YOURS":                upstream.Rejected,
	"SOURCE_INACTIVE":                 upstream.Rejected,
	"STORE_QUOTA_EXCEEDED":            upstream.Internal,
	"STORE_LINK_UNAVAILABLE":          upstream.Unavailable,
}

// A refusal of moyu's own application key is moyu's whatever the code; it used
// to tell readers to sign in again, which cannot grant the key a scope. The
// reader's token is refused for something a fresh sign-in fixes only when it
// predates a scope or catalog no longer accepts it.
func problemKind(method string, byReader bool, status int, code string) upstream.Kind {
	switch {
	case byReader && (code == CodeScopeRequired || code == CodeInvalidCredential):
		return upstream.Rejected
	case !byReader && (status == http.StatusUnauthorized || status == http.StatusForbidden):
		return upstream.Internal
	case code == CodeValidationFailed && method == http.MethodGet:
		return upstream.Internal
	}
	if k, ok := problemKinds[code]; ok {
		return k
	}
	return upstream.ByStatus(status)
}

// A body that is not catalog's problem+json did not come from catalog: a 404
// page from a proxy means moyu is pointed at the wrong origin, not that the
// game is missing.
func bareKind(status int) upstream.Kind {
	switch {
	case status >= 500:
		return upstream.Unavailable
	case status == http.StatusTooManyRequests:
		return upstream.RateLimited
	default:
		return upstream.Internal
	}
}

func failure(op, method string, byReader bool, resp *http.Response, raw []byte) *upstream.Error {
	e := &upstream.Error{
		Service: service, Op: op, Status: resp.StatusCode,
		RequestID:  resp.Header.Get("X-Request-ID"),
		RetryAfter: upstream.RetryAfter(resp.Header),
	}
	var p Problem
	if json.Unmarshal(raw, &p) == nil && p.Code != "" {
		e.Code = p.Code
		e.Kind = problemKind(method, byReader, resp.StatusCode, p.Code)
		if p.RequestID != "" {
			e.RequestID = p.RequestID
		}
		e.Cause = &p
		return e
	}
	e.Kind = bareKind(resp.StatusCode)
	e.Detail = snippet(raw)
	return e
}

func snippet(raw []byte) string {
	s := strings.TrimSpace(string(raw))
	if len(s) <= 200 {
		return s
	}
	return strings.ToValidUTF8(s[:200], "") + "…"
}

// Absent is catalog's "nothing here" said without a 404: an empty answer to a
// lookup by ref, or a row this site does not draw.
func Absent(op string) error {
	return &upstream.Error{Service: service, Op: op, Kind: upstream.NotFound}
}

func ProblemOf(err error) (*Problem, bool) {
	var p *Problem
	ok := errors.As(err, &p)
	return p, ok
}

func MergedInto(err error) (int64, bool) {
	p, ok := ProblemOf(err)
	if !ok || p.Code != CodeEntityMerged {
		return 0, false
	}
	return ParseID(p.CurrentID)
}

// IsNotFound leaves a merge out: a merged id has somewhere to go.
func IsNotFound(err error) bool {
	if _, merged := MergedInto(err); merged {
		return false
	}
	return upstream.KindOf(err) == upstream.NotFound
}

// ReauthRequired is the reader's session token refused for a scope it was
// minted without. Signing in again is the only fix, and the frontend knows it
// as 40399.
func ReauthRequired(err error) bool {
	e, ok := upstream.As(err)
	return ok && e.Service == service && e.Kind == upstream.Rejected &&
		(e.Code == CodeScopeRequired || e.Code == CodeInvalidCredential)
}
