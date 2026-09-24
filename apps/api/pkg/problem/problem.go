// Package problem writes RFC 9457 problem details.
//
// This is the error language of the public developer-platform face, not of this
// site's own BFF: /api/v1 keeps the house {code, message, data} envelope, while
// /v2/moyu answers the way catalog's /v2 does. A third party holding one nmk_
// key across both faces should not have to decode two error dialects, and the
// gateway's own refusals (401 and 429 from ForwardAuth) are the only ones this
// service never gets to shape.
package problem

import (
	"crypto/rand"
	"encoding/binary"
	"time"

	"github.com/gofiber/fiber/v3"
)

// Codes and their titles are reused verbatim from infra's closed registry
// (apps/api/internal/platform/apiv2/problem/registry.go) so one decoder covers
// both faces. Only the subset a read-only face can produce is here.
const (
	CodeInvalidParameter   = "INVALID_PARAMETER"
	CodeUnknownEnumValue   = "UNKNOWN_ENUM_VALUE"
	CodeLimitTooLarge      = "LIMIT_TOO_LARGE"
	CodeTooManyIDs         = "TOO_MANY_IDS"
	CodeInvalidCursor      = "INVALID_CURSOR"
	CodeUnknownInclude     = "UNKNOWN_INCLUDE"
	CodeUnknownSort        = "UNKNOWN_SORT"
	CodeNotFound           = "NOT_FOUND"
	CodeMethodNotAllowed   = "METHOD_NOT_ALLOWED"
	CodeInternalError      = "INTERNAL_ERROR"
	CodeServiceUnavailable = "SERVICE_UNAVAILABLE"
)

// Field-level reasons, same registry.
const (
	ReasonInvalidFormat   = "INVALID_FORMAT"
	ReasonOutOfRange      = "OUT_OF_RANGE"
	ReasonTooManyItems    = "TOO_MANY_ITEMS"
	ReasonUnknownValue    = "UNKNOWN_VALUE"
	ReasonNotAllowedValue = "NOT_ALLOWED_VALUE"
)

// Every code above is one of infra's platform codes, and a code has exactly one
// type URI across the platform, so it is infra's URI. This face used to publish
// them under problems/moyu/, while the developer docs for it already showed
// problems/platform/.
const TypeBase = "https://developer.nextmoe.dev/problems/platform/"

// ContentType is RFC 9457's media type.
const ContentType = "application/problem+json"

type definition struct {
	title  string
	status int
	slug   string
}

var registry = map[string]definition{
	CodeInvalidParameter:   {"Invalid parameter", fiber.StatusBadRequest, "invalid-parameter"},
	CodeUnknownEnumValue:   {"Unknown enum value", fiber.StatusBadRequest, "unknown-enum-value"},
	CodeLimitTooLarge:      {"Limit too large", fiber.StatusBadRequest, "limit-too-large"},
	CodeTooManyIDs:         {"Too many ids", fiber.StatusBadRequest, "too-many-ids"},
	CodeInvalidCursor:      {"Invalid cursor", fiber.StatusBadRequest, "invalid-cursor"},
	CodeUnknownInclude:     {"Unknown include", fiber.StatusBadRequest, "unknown-include"},
	CodeUnknownSort:        {"Unknown sort", fiber.StatusBadRequest, "unknown-sort"},
	CodeNotFound:           {"Not found", fiber.StatusNotFound, "not-found"},
	CodeMethodNotAllowed:   {"Method not allowed", fiber.StatusMethodNotAllowed, "method-not-allowed"},
	CodeInternalError:      {"Internal error", fiber.StatusInternalServerError, "internal-error"},
	CodeServiceUnavailable: {"Service unavailable", fiber.StatusServiceUnavailable, "service-unavailable"},
}

// FieldError names the one parameter that failed. Exactly one of Parameter or
// Header is set; a read face has no request body to point into.
type FieldError struct {
	Parameter string `json:"parameter,omitempty"`
	Header    string `json:"header,omitempty"`
	Reason    string `json:"reason"`
	Detail    string `json:"detail"`
}

// Problem is the wire shape. Field names and their meanings match infra's, so
// the two faces decode the same way; the members that only make sense for a
// write face (object, current_id, suspects) are left out rather than emitted
// permanently empty.
type Problem struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Detail    string       `json:"detail"`
	Instance  string       `json:"instance"`
	Code      string       `json:"code"`
	RequestID string       `json:"request_id"`
	Errors    []FieldError `json:"errors"`
}

// Error lets a Problem travel as an error through the service layer, so a
// handler can return one without a second vocabulary for the same failure.
func (p *Problem) Error() string {
	if p == nil {
		return ""
	}
	if p.Detail != "" {
		return p.Detail
	}
	return p.Title
}

// New builds a problem without writing it, for the service layer.
func New(code, detail string, fields ...FieldError) *Problem {
	def, ok := registry[code]
	if !ok {
		code, def = CodeInternalError, registry[CodeInternalError]
	}
	if fields == nil {
		fields = []FieldError{}
	}
	return &Problem{
		Type:   TypeBase + def.slug,
		Title:  def.title,
		Status: def.status,
		Detail: detail,
		Code:   code,
		Errors: fields,
	}
}

// Param is the common case: one query parameter, one reason.
func Param(code, name, reason, detail string) *Problem {
	return New(code, detail, FieldError{Parameter: name, Reason: reason, Detail: detail})
}

// Write answers with a problem document, filling in the two members that are
// only knowable at the edge.
func Write(c fiber.Ctx, p *Problem) error {
	if p == nil {
		p = New(CodeInternalError, "")
	}
	p.Instance = instance(c)
	if p.RequestID == "" {
		p.RequestID = NewRequestID()
	}
	if p.Errors == nil {
		p.Errors = []FieldError{}
	}
	c.Set("X-Request-ID", p.RequestID)
	// The content type is c.JSON's second argument, not a header set
	// beforehand: c.JSON overwrites Content-Type, which quietly turned every
	// problem document back into application/json.
	return c.Status(p.Status).JSON(p, ContentType)
}

// WriteCode is Write for a failure with nothing to add beyond its code.
func WriteCode(c fiber.Ctx, code, detail string) error {
	return Write(c, New(code, detail))
}

func instance(c fiber.Ctx) string {
	path := c.Path()
	// For a read face, which filter combination failed is most of the
	// diagnostic value, so the query string stays on.
	if raw := string(c.Request().URI().QueryString()); raw != "" {
		return path + "?" + raw
	}
	return path
}

// crockford is the ULID alphabet: base32 without I, L, O and U.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// NewRequestID mints the same shape infra does -- `req_` plus a 26-character
// ULID -- so a caller reporting a failure quotes one kind of identifier no
// matter which face produced it. This one resolves in moyu's logs, not in
// infra's: the gateway never sees a response body.
//
// A real ULID, not merely 26 characters matching the pattern: 10 characters of
// 48-bit millisecond timestamp then 16 of 80-bit randomness, so a client
// decoding one with a ULID library reads back the moment the failure happened.
func NewRequestID() string {
	out := make([]byte, 4, 30)
	copy(out, "req_")

	ms := uint64(time.Now().UTC().UnixMilli())
	// 10 characters carry 50 bits; the top 2 of the first are the padding.
	for shift := 45; shift >= 0; shift -= 5 {
		out = append(out, crockford[(ms>>uint(shift))&0x1f])
	}

	var rnd [10]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		// A request id is diagnostic, never a decision input, so a starved
		// entropy pool must not turn a 404 into a 500.
		binary.BigEndian.PutUint64(rnd[:8], uint64(time.Now().UnixNano()))
	}
	// 80 bits divide evenly into 16 characters, so nothing is left over.
	var acc, bits uint16
	for _, b := range rnd {
		acc = acc<<8 | uint16(b)
		for bits += 8; bits >= 5; bits -= 5 {
			out = append(out, crockford[(acc>>(bits-5))&0x1f])
		}
	}
	return string(out)
}
