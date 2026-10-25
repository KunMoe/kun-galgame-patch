package service

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"kun-galgame-patch-api/internal/face/repository"
	"kun-galgame-patch-api/pkg/problem"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

const (
	DefaultLimit = 20
	// MaxLimit and MaxBatchIDs are the platform's numbers, and the platform
	// answers LIMIT_TOO_LARGE rather than clamping: a silently clamped page is
	// a caller who thinks they have read everything.
	MaxLimit    = 100
	MaxBatchIDs = 100
)

// The closed vocabularies this face declares. They are the site's own lists;
// where the site silently drops an unknown token, the face refuses it, because
// a filter that quietly did not apply is worse than a 400 for a third party
// building against a contract.
var (
	PatchTypes = []string{
		"manual", "ai", "machine_polishing", "machine", "save", "crack",
		"fix", "mod", "r18", "decensor", "image", "other",
	}
	PatchLanguages = []string{"zh-Hans", "zh-Hant", "ja", "en", "other"}
	PatchPlatforms = []string{"windows", "android", "macos", "ios", "linux", "other"}

	PatchIncludes    = []string{"resources", "publisher"}
	ResourceIncludes = []string{"publisher"}

	RefSources = []string{"vndb", "catalog"}
)

type Includes map[string]bool

func (i Includes) Has(name string) bool { return i[name] }

type PatchQuery struct {
	Filter       repository.PatchFilter
	Include      Includes
	IncludeTotal bool
	// Refs and IDs are the anchors that were asked for, canonicalised, so
	// `missing` can be computed by comparing them against the rows that came
	// back.
	Refs []string
	IDs  []string
}

func ParsePatchQuery(c fiber.Ctx) (*PatchQuery, *problem.Problem) {
	q := &PatchQuery{Filter: repository.PatchFilter{Limit: DefaultLimit}}

	include, err := ParseInclude(c, PatchIncludes)
	if err != nil {
		return nil, err
	}
	q.Include = include

	if q.IncludeTotal, err = ParseBool(c, "include_total", false); err != nil {
		return nil, err
	}

	nsfw, err := ParseBool(c, "nsfw", false)
	if err != nil {
		return nil, err
	}
	if !nsfw {
		q.Filter.ContentLimit = utils.ContentLimitSFW
	}

	if q.Filter.Types, err = parseEnumCSV(c, "type", PatchTypes); err != nil {
		return nil, err
	}
	if q.Filter.Language, err = parseEnumCSV(c, "language", PatchLanguages); err != nil {
		return nil, err
	}
	if q.Filter.Platform, err = parseEnumCSV(c, "platform", PatchPlatforms); err != nil {
		return nil, err
	}
	if q.Filter.HasResources, err = parseOptionalBool(c, "has_resources"); err != nil {
		return nil, err
	}

	if err = q.parseBatch(c); err != nil {
		return nil, err
	}

	if q.Filter.Limit, err = ParseLimit(c, q.Filter.Batch); err != nil {
		return nil, err
	}
	// A batch answers a set, so sort and cursor have nothing to act on. Saying
	// so is better than accepting them and returning something else.
	if q.Filter.Batch {
		if c.Query("cursor") != "" {
			return nil, problem.Param(problem.CodeInvalidParameter, "cursor",
				problem.ReasonNotAllowedValue, "cursor cannot be combined with ids= or refs=")
		}
		if c.Query("sort") != "" {
			return nil, problem.Param(problem.CodeInvalidParameter, "sort",
				problem.ReasonNotAllowedValue, "sort cannot be combined with ids= or refs=")
		}
		return q, nil
	}

	if q.Filter.Sort, err = parseSort(c); err != nil {
		return nil, err
	}
	if q.Filter.Offset, err = ParseCursor(c); err != nil {
		return nil, err
	}
	return q, nil
}

func (q *PatchQuery) parseBatch(c fiber.Ctx) *problem.Problem {
	rawIDs, rawRefs := c.Query("ids"), c.Query("refs")
	if rawIDs != "" && rawRefs != "" {
		return problem.Param(problem.CodeInvalidParameter, "refs",
			problem.ReasonNotAllowedValue, "ids and refs cannot be combined; use one anchor per request")
	}
	switch {
	case rawIDs != "":
		tokens, err := splitBatch(rawIDs, "ids")
		if err != nil {
			return err
		}
		for _, token := range tokens {
			n, convErr := strconv.Atoi(token)
			if convErr != nil || n <= 0 {
				return problem.Param(problem.CodeInvalidParameter, "ids",
					problem.ReasonInvalidFormat, "not a patch id: "+token)
			}
			q.Filter.IDs = append(q.Filter.IDs, n)
			// The canonical spelling, not the caller's: `missing` is computed
			// by comparing these against the rows that came back, and `00117`
			// would find patch 117 and then report it missing.
			q.IDs = append(q.IDs, strconv.Itoa(n))
		}
		q.Filter.Batch = true
	case rawRefs != "":
		tokens, err := splitBatch(rawRefs, "refs")
		if err != nil {
			return err
		}
		for _, token := range tokens {
			source, external, ok := strings.Cut(token, ":")
			if !ok || external == "" || !slices.Contains(RefSources, source) {
				return problem.Param(problem.CodeInvalidParameter, "refs",
					problem.ReasonInvalidFormat,
					fmt.Sprintf("not a ref: %s (expected %s)", token, strings.Join(RefSources, ":<id> or ")+":<id>"))
			}
			if source == "catalog" {
				n, convErr := strconv.ParseInt(external, 10, 64)
				if convErr != nil || n <= 0 {
					return problem.Param(problem.CodeInvalidParameter, "refs",
						problem.ReasonInvalidFormat, "not a catalog work id: "+external)
				}
				q.Filter.WorkIDs = append(q.Filter.WorkIDs, n)
				q.Refs = append(q.Refs, "catalog:"+strconv.FormatInt(n, 10))
				continue
			}
			q.Filter.VndbIDs = append(q.Filter.VndbIDs, external)
			q.Refs = append(q.Refs, token)
		}
		q.Filter.Batch = true
	}
	return nil
}

func splitBatch(raw, name string) ([]string, *problem.Problem) {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" && !slices.Contains(out, part) {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return nil, problem.Param(problem.CodeInvalidParameter, name,
			problem.ReasonInvalidFormat, name+" was empty")
	}
	if len(out) > MaxBatchIDs {
		return nil, problem.Param(problem.CodeTooManyIDs, name, problem.ReasonTooManyItems,
			fmt.Sprintf("%s named %d items; the maximum is %d", name, len(out), MaxBatchIDs))
	}
	return out, nil
}

// ParseLimit is one rule in one place: the "not clamped" contract is written
// into the error text, and a second copy for the sub-collections would be a
// second answer to the same question.
func ParseResourceTypes(c fiber.Ctx) ([]string, *problem.Problem) {
	return parseEnumCSV(c, "type", PatchTypes)
}

func ParseLimit(c fiber.Ctx, batch bool) (int, *problem.Problem) {
	raw := c.Query("limit")
	if raw == "" {
		if batch {
			return MaxBatchIDs, nil
		}
		return DefaultLimit, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, problem.Param(problem.CodeInvalidParameter, "limit",
			problem.ReasonInvalidFormat, "limit is not an integer: "+raw)
	}
	if n < 1 {
		return 0, problem.Param(problem.CodeInvalidParameter, "limit",
			problem.ReasonOutOfRange, "limit must be at least 1")
	}
	if n > MaxLimit {
		return 0, problem.Param(problem.CodeLimitTooLarge, "limit", problem.ReasonOutOfRange,
			fmt.Sprintf("limit is %d; the maximum is %d. The value is not clamped.", n, MaxLimit))
	}
	return n, nil
}

func parseSort(c fiber.Ctx) (string, *problem.Problem) {
	raw := c.Query("sort")
	if raw == "" {
		return repository.SortKeys[0], nil
	}
	if !slices.Contains(repository.SortKeys, raw) {
		return "", problem.Param(problem.CodeUnknownSort, "sort", problem.ReasonUnknownValue,
			fmt.Sprintf("unknown sort %q; this collection declares %s", raw, strings.Join(repository.SortKeys, ", ")))
	}
	return raw, nil
}
