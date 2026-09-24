// Package catalogv2test writes catalog's failures the way nextmoe-infra does,
// so a fake cannot answer in a shape the real service never sends. The fakes
// this replaced invented codes (FORBIDDEN, CONFLICT, BAD_GATEWAY, INTERNAL)
// that are not in catalog's registry, and the client's mapping was tested
// against them.
package catalogv2test

import (
	"encoding/json"
	"net/http"
	"strings"
)

const RequestID = "req_01J8Z3M4T6Y0000000000000AB"

type def struct {
	domain string
	status int
	title  string
}

// Copied from nextmoe-infra apps/api/internal/platform/apiv2/problem/registry.go.
var registry = map[string]def{
	"MALFORMED_BODY":                  {"platform", 400, "Malformed body"},
	"INVALID_PARAMETER":               {"platform", 400, "Invalid parameter"},
	"UNKNOWN_ENUM_VALUE":              {"platform", 400, "Unknown enum value"},
	"MUTUALLY_EXCLUSIVE_PARAMETERS":   {"platform", 400, "Mutually exclusive parameters"},
	"LIMIT_TOO_LARGE":                 {"platform", 400, "Limit too large"},
	"TOO_MANY_IDS":                    {"platform", 400, "Too many ids"},
	"INVALID_CURSOR":                  {"platform", 400, "Invalid cursor"},
	"UNKNOWN_INCLUDE":                 {"platform", 400, "Unknown include"},
	"UNKNOWN_FIELD":                   {"platform", 400, "Unknown field"},
	"UNKNOWN_SORT":                    {"platform", 400, "Unknown sort"},
	"UNKNOWN_FACET":                   {"platform", 400, "Unknown facet"},
	"MISSING_CREDENTIAL":              {"platform", 401, "Missing credential"},
	"INVALID_CREDENTIAL":              {"platform", 401, "Invalid credential"},
	"SCOPE_REQUIRED":                  {"platform", 403, "Scope required"},
	"FIRST_PARTY_ONLY":                {"platform", 403, "First party only"},
	"NOT_FOUND":                       {"platform", 404, "Not found"},
	"METHOD_NOT_ALLOWED":              {"platform", 405, "Method not allowed"},
	"IDEMPOTENCY_KEY_REUSED":          {"platform", 409, "Idempotency key reused"},
	"IDEMPOTENCY_REQUEST_IN_PROGRESS": {"platform", 409, "Idempotency request in progress"},
	"GONE":                            {"platform", 410, "Gone"},
	"PRECONDITION_FAILED":             {"platform", 412, "Precondition failed"},
	"PAYLOAD_TOO_LARGE":               {"platform", 413, "Payload too large"},
	"UNSUPPORTED_MEDIA_TYPE":          {"platform", 415, "Unsupported media type"},
	"VALIDATION_FAILED":               {"platform", 422, "Validation failed"},
	"PRECONDITION_REQUIRED":           {"platform", 428, "Precondition required"},
	"RATE_LIMITED":                    {"platform", 429, "Rate limited"},
	"QUOTA_EXCEEDED":                  {"platform", 429, "Quota exceeded"},
	"INTERNAL_ERROR":                  {"platform", 500, "Internal error"},
	"SERVICE_UNAVAILABLE":             {"platform", 503, "Service unavailable"},
	"ENTITY_MERGED":                   {"catalog", 404, "Entity merged"},
	"USER_IDENTITY_REQUIRED":          {"me", 403, "User identity required"},
	"SITE_NOT_BOUND":                  {"me", 403, "Site not bound"},
	"RELEASE_CREATION_DISABLED":       {"me", 403, "Release creation disabled"},
	"ALREADY_EXISTS":                  {"me", 409, "Already exists"},
	"DUPLICATE_SUSPECTS":              {"me", 409, "Duplicate suspects"},
	"INVALID_STATE_TRANSITION":        {"me", 409, "Invalid state transition"},
	"CLAIM_NOT_OWNED":                 {"me", 403, "Claim not owned"},
	"PERMISSION_REQUIRED":             {"moderation", 403, "Permission required"},
	"TENANT_MISMATCH":                 {"moderation", 403, "Tenant mismatch"},
	"DECISION_ALREADY_MADE":           {"moderation", 409, "Decision already made"},
	"SOURCE_NOT_YOURS":                {"news", 403, "Source not yours"},
	"SOURCE_INACTIVE":                 {"news", 422, "Source inactive"},
	"STORE_QUOTA_EXCEEDED":            {"store", 403, "Store quota exceeded"},
	"STORE_LINK_UNAVAILABLE":          {"store", 502, "Store link unavailable"},
}

// Codes is every code in the registry, for a test that has to cover them all.
func Codes() []string {
	out := make([]string, 0, len(registry))
	for code := range registry {
		out = append(out, code)
	}
	return out
}

func Status(code string) int {
	return lookup(code).status
}

// Problem answers the way infra's problem.WriteFiberError does
// (apps/api/internal/platform/apiv2/problem/problem.go): the status, type and
// title come from the registry, request_id is in the body and in X-Request-ID,
// errors is present even when empty, and the media type is problem+json.
// extra carries the per-code members: errors, object and current_id,
// suspects.
func Problem(w http.ResponseWriter, r *http.Request, code, detail string, extra map[string]any) {
	d := lookup(code)
	body := map[string]any{
		"type":       "https://developer.nextmoe.dev/problems/" + d.domain + "/" + strings.ToLower(strings.ReplaceAll(code, "_", "-")),
		"title":      d.title,
		"status":     d.status,
		"detail":     detail,
		"instance":   r.URL.RequestURI(),
		"code":       code,
		"request_id": RequestID,
		"errors":     []any{},
	}
	for k, v := range extra {
		body[k] = v
	}
	w.Header().Set("X-Request-ID", RequestID)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/problem+json")
	if d.status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", `Bearer realm="nextmoe", error="invalid_token"`)
	}
	w.WriteHeader(d.status)
	_ = json.NewEncoder(w).Encode(body)
}

func lookup(code string) def {
	d, ok := registry[code]
	if !ok {
		panic("catalogv2test: " + code + " is not in catalog's problem registry")
	}
	return d
}
