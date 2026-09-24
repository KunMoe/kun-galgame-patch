package service

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"kun-galgame-patch-api/pkg/problem"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

// parse runs the real parser behind a real fiber context, because half of what
// it does is read query parameters off one.
func parse(t *testing.T, query string) (*PatchQuery, *problem.Problem) {
	t.Helper()
	app := fiber.New()
	var (
		got  *PatchQuery
		prob *problem.Problem
	)
	app.Get("/x", func(c fiber.Ctx) error {
		got, prob = ParsePatchQuery(c)
		return c.SendStatus(fiber.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/x?"+query, nil)
	if _, err := app.Test(req); err != nil {
		t.Fatalf("test request: %v", err)
	}
	return got, prob
}

func TestParsePatchQueryRefusals(t *testing.T) {
	cases := []struct {
		name, query, code, parameter string
	}{
		{"limit is not clamped", "limit=101", problem.CodeLimitTooLarge, "limit"},
		{"limit below one", "limit=0", problem.CodeInvalidParameter, "limit"},
		{"limit is not a number", "limit=many", problem.CodeInvalidParameter, "limit"},
		{"unknown sort", "sort=rating", problem.CodeUnknownSort, "sort"},
		{"unknown include", "include=stickers", problem.CodeUnknownInclude, "include"},
		{"unknown language", "language=klingon", problem.CodeUnknownEnumValue, "language"},
		{"unknown type", "type=fansub", problem.CodeUnknownEnumValue, "type"},
		{"unknown platform", "platform=switch", problem.CodeUnknownEnumValue, "platform"},
		{"nsfw is not a boolean", "nsfw=yes", problem.CodeInvalidParameter, "nsfw"},
		{"cursor is not ours", "cursor=abc", problem.CodeInvalidCursor, "cursor"},
		{"cursor payload is not an offset", "cursor=cur_" + b64("nope"), problem.CodeInvalidCursor, "cursor"},
		{"ids and refs together", "ids=1&refs=vndb:v1", problem.CodeInvalidParameter, "refs"},
		{"ref without a source", "refs=v65869", problem.CodeInvalidParameter, "refs"},
		{"ref with an unknown source", "refs=bangumi:123", problem.CodeInvalidParameter, "refs"},
		{"catalog ref that is not a number", "refs=catalog:v1", problem.CodeInvalidParameter, "refs"},
		{"id that is not a number", "ids=abc", problem.CodeInvalidParameter, "ids"},
		{"cursor with a batch", "ids=1&cursor=cur_" + b64("20"), problem.CodeInvalidParameter, "cursor"},
		{"sort with a batch", "ids=1&sort=views", problem.CodeInvalidParameter, "sort"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, prob := parse(t, tc.query)
			if prob == nil {
				t.Fatalf("%s was accepted", tc.query)
			}
			if prob.Code != tc.code {
				t.Errorf("code = %s, want %s", prob.Code, tc.code)
			}
			if len(prob.Errors) != 1 || prob.Errors[0].Parameter != tc.parameter {
				t.Errorf("errors = %+v, want one naming %s", prob.Errors, tc.parameter)
			}
		})
	}
}

func TestParsePatchQueryTooManyIDs(t *testing.T) {
	refs := make([]string, MaxBatchIDs+1)
	for i := range refs {
		refs[i] = "vndb:v" + strconv.Itoa(i+1)
	}
	_, prob := parse(t, "refs="+strings.Join(refs, ","))
	if prob == nil || prob.Code != problem.CodeTooManyIDs {
		t.Fatalf("101 refs = %+v, want TOO_MANY_IDS", prob)
	}
}

func TestParsePatchQueryDefaults(t *testing.T) {
	q, prob := parse(t, "")
	if prob != nil {
		t.Fatalf("empty query refused: %+v", prob)
	}
	if q.Filter.Limit != DefaultLimit {
		t.Errorf("limit = %d, want %d", q.Filter.Limit, DefaultLimit)
	}
	if q.Filter.Sort != "updated" {
		t.Errorf("sort = %q, want updated", q.Filter.Sort)
	}
	// The default gate is the site's default gate. A face that answered nsfw
	// rows to a caller that did not ask is the one failure mode here that
	// cannot be walked back.
	if q.Filter.ContentLimit != utils.ContentLimitSFW {
		t.Errorf("content limit = %q, want %q", q.Filter.ContentLimit, utils.ContentLimitSFW)
	}
	if q.IncludeTotal || q.Include.Has("resources") || q.Include.Has("publisher") {
		t.Errorf("nothing should be included by default: %+v", q)
	}
}

func TestParsePatchQueryNSFWOpensBothGates(t *testing.T) {
	q, prob := parse(t, "nsfw=true")
	if prob != nil {
		t.Fatalf("refused: %+v", prob)
	}
	if q.Filter.ContentLimit != "" {
		t.Errorf("content limit = %q, want it unset", q.Filter.ContentLimit)
	}
}

func TestParsePatchQueryBatchAnchors(t *testing.T) {
	q, prob := parse(t, "refs=vndb:v65869,catalog:61311,vndb:v65869")
	if prob != nil {
		t.Fatalf("refused: %+v", prob)
	}
	if !q.Filter.Batch {
		t.Fatal("refs did not turn on the batch lane")
	}
	// The three anchors are different id spaces and have to land in different
	// columns; folding them into one IN list would match another game.
	if len(q.Filter.VndbIDs) != 1 || q.Filter.VndbIDs[0] != "v65869" {
		t.Errorf("vndb ids = %v", q.Filter.VndbIDs)
	}
	if len(q.Filter.WorkIDs) != 1 || q.Filter.WorkIDs[0] != 61311 {
		t.Errorf("work ids = %v", q.Filter.WorkIDs)
	}
	if q.Filter.Limit != MaxBatchIDs {
		t.Errorf("batch limit = %d, want %d", q.Filter.Limit, MaxBatchIDs)
	}
}

// A padded anchor found its row and was then reported missing, because the
// caller's spelling was compared against the canonical one.
func TestParsePatchQueryCanonicalisesAnchors(t *testing.T) {
	q, prob := parse(t, "refs=catalog:0061311")
	if prob != nil {
		t.Fatalf("refused: %+v", prob)
	}
	if len(q.Refs) != 1 || q.Refs[0] != "catalog:61311" {
		t.Errorf("refs = %v, want the canonical spelling", q.Refs)
	}
	if q, prob = parse(t, "ids=00117"); prob != nil {
		t.Fatalf("refused: %+v", prob)
	}
	if len(q.IDs) != 1 || q.IDs[0] != "117" {
		t.Errorf("ids = %v, want the canonical spelling", q.IDs)
	}
}

func TestCursorRoundTrip(t *testing.T) {
	cur := EncodeCursor(40)
	if cur == nil || !strings.HasPrefix(*cur, "cur_") {
		t.Fatalf("cursor = %v, want the platform's cur_ prefix", cur)
	}
	q, prob := parse(t, "cursor="+*cur)
	if prob != nil {
		t.Fatalf("our own cursor was refused: %+v", prob)
	}
	if q.Filter.Offset != 40 {
		t.Errorf("offset = %d, want 40", q.Filter.Offset)
	}
	if EncodeCursor(0) != nil {
		t.Error("offset 0 must not mint a cursor; there is no page before the first")
	}
}

var requestIDShape = regexp.MustCompile(`^req_[0-9A-HJKMNP-TV-Z]{26}$`)

func TestRequestIDMatchesThePlatformShape(t *testing.T) {
	seen := map[string]bool{}
	for range 1000 {
		id := problem.NewRequestID()
		if !requestIDShape.MatchString(id) {
			t.Fatalf("request id %q does not match %s", id, requestIDShape)
		}
		if seen[id] {
			t.Fatalf("request id %q repeated", id)
		}
		seen[id] = true
	}
}

// Matching the pattern is not enough: infra documents these as ULIDs, so a
// client decoding one with a ULID library has to read back the moment the
// failure happened rather than a number four times too large.
func TestRequestIDIsADecodableULID(t *testing.T) {
	const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	before := time.Now().UTC().UnixMilli()
	id := problem.NewRequestID()
	after := time.Now().UTC().UnixMilli()

	var ms int64
	for _, c := range id[len("req_") : len("req_")+10] {
		ms = ms<<5 | int64(strings.IndexRune(crockford, c))
	}
	if ms < before || ms > after {
		t.Errorf("timestamp in %s decoded to %d, want between %d and %d", id, ms, before, after)
	}
}

func TestProblemIsWrittenAsProblemJSON(t *testing.T) {
	app := fiber.New()
	app.Get("/x", func(c fiber.Ctx) error {
		return problem.Write(c, problem.Param(problem.CodeLimitTooLarge, "limit", problem.ReasonOutOfRange, "too big"))
	})
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/x?limit=999", nil))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", res.StatusCode)
	}
	// c.JSON overwrites Content-Type, so a header set beforehand silently
	// turned every problem document back into application/json.
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, problem.ContentType) {
		t.Errorf("content-type = %q, want %s", ct, problem.ContentType)
	}
	body, _ := io.ReadAll(res.Body)
	var doc problem.Problem
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("body is not json: %v", err)
	}
	if doc.Type != "https://developer.nextmoe.dev/problems/platform/limit-too-large" || doc.Code != problem.CodeLimitTooLarge {
		t.Errorf("doc = %+v", doc)
	}
	if doc.Instance != "/x?limit=999" {
		t.Errorf("instance = %q, want the failing query string with it", doc.Instance)
	}
	if !requestIDShape.MatchString(doc.RequestID) || res.Header.Get("X-Request-ID") != doc.RequestID {
		t.Errorf("request id %q is not echoed in the header %q", doc.RequestID, res.Header.Get("X-Request-ID"))
	}
}

func b64(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}
