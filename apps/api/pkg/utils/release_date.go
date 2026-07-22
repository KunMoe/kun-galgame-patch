package utils

import (
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Release-date filter parsing for GET /api/galgame, mirroring the galgame
// protocol in docs/galgame_wiki/00-handbook §17. moyu can't import galgame's
// pkg/utils helpers (separate repo), so this reimplements the same YYYY /
// YYYY-MM → date-boundary contract against the local patch.release_date
// column (PG `date`, no time component — so boundaries are plain dates, not
// the 00:00:00 / 23:59:59 timestamps galgame uses).

// ErrInvalidReleaseBound is returned for malformed released_from/to input
// (e.g. "24", "2024-3" missing zero-pad, "2024-13", "garbage"). The handler
// maps it to a 400 — §17.1 mandates loud rejection, not silent ignore.
var ErrInvalidReleaseBound = errors.New("invalid release date bound (expect YYYY or YYYY-MM)")

// ErrInvalidMonthSet is returned for malformed released_months input
// (element outside 1-12, non-numeric, or empty element like "3,,7"). 400.
var ErrInvalidMonthSet = errors.New("invalid released_months (expect comma-separated 1-12)")

// ParseMonthSet parses the released_months CSV (e.g. "3,7,12") into a sorted,
// deduped []int of months 1-12, per docs/galgame_wiki/00-handbook §17.10. It's
// an AND filter layered on top of the year range: keep only games whose
// release month ∈ the set. "" / whitespace → (nil, nil) = no month filter.
//
// Used by GET /api/galgame as `EXTRACT(MONTH FROM release_date)::int IN (...)`.
// NULL release_date rows drop automatically (EXTRACT(NULL) → NULL → not IN),
// matching the §17.4 year-range NULL semantics.
func ParseMonthSet(s string) ([]int, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	seen := make(map[int]struct{})
	months := make([]int, 0, 12)
	for _, part := range strings.Split(s, ",") {
		p := strings.TrimSpace(part)
		m, err := strconv.Atoi(p)
		if err != nil || m < 1 || m > 12 {
			return nil, ErrInvalidMonthSet
		}
		if _, dup := seen[m]; dup {
			continue
		}
		seen[m] = struct{}{}
		months = append(months, m)
	}
	sort.Ints(months)
	return months, nil
}

var (
	releaseYearRe  = regexp.MustCompile(`^\d{4}$`)
	releaseMonthRe = regexp.MustCompile(`^(\d{4})-(\d{2})$`)
)

// ParseReleaseLowerBound parses released_from into an inclusive lower date
// bound. "" → (nil, nil) meaning "no lower bound". YYYY → Jan 1 of that year;
// YYYY-MM → the 1st of that month. Malformed → ErrInvalidReleaseBound.
func ParseReleaseLowerBound(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	if releaseYearRe.MatchString(s) {
		t := time.Date(atoi(s), time.January, 1, 0, 0, 0, 0, time.UTC)
		return &t, nil
	}
	if m := releaseMonthRe.FindStringSubmatch(s); m != nil {
		mm := atoi(m[2])
		if mm < 1 || mm > 12 {
			return nil, ErrInvalidReleaseBound
		}
		t := time.Date(atoi(m[1]), time.Month(mm), 1, 0, 0, 0, 0, time.UTC)
		return &t, nil
	}
	return nil, ErrInvalidReleaseBound
}

// ParseReleaseUpperBound parses released_to into an inclusive upper date
// bound. "" → (nil, nil). YYYY → Dec 31 of that year; YYYY-MM → the last day
// of that month (28/29/30/31 handled by date normalization). Malformed →
// ErrInvalidReleaseBound.
func ParseReleaseUpperBound(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	if releaseYearRe.MatchString(s) {
		t := time.Date(atoi(s), time.December, 31, 0, 0, 0, 0, time.UTC)
		return &t, nil
	}
	if m := releaseMonthRe.FindStringSubmatch(s); m != nil {
		mm := atoi(m[2])
		if mm < 1 || mm > 12 {
			return nil, ErrInvalidReleaseBound
		}
		// Last day of month = first day of next month minus one day.
		// time.Month(mm)+1 with mm=12 normalizes to next-year January.
		firstNext := time.Date(atoi(m[1]), time.Month(mm)+1, 1, 0, 0, 0, 0, time.UTC)
		last := firstNext.AddDate(0, 0, -1)
		return &last, nil
	}
	return nil, ErrInvalidReleaseBound
}

// ParseGalgameReleaseDate turns galgame's release_date string into a date-only
// *time.Time for storage in patch.release_date. "" / nil / unparseable → nil
// (best-effort: skip rather than abort).
//
// Format: canonical galgame now returns a bare `YYYY-MM-DD` string (per
// docs/galgame_wiki/00-handbook §17.7 — a custom Date type fixed the earlier
// bug where the PG `date` column was serialized as RFC3339 "2016-11-25T00:00:00Z").
// We try the bare-date layout first (the current canonical shape), then fall
// back to RFC3339 (± fractional seconds) to stay robust against older cached
// payloads / pre-fix data — §17.7 explicitly endorses this dual-tolerance.
func ParseGalgameReleaseDate(s string) *time.Time {
	if s == "" {
		return nil
	}
	layouts := []string{
		"2006-01-02",                    // canonical: bare date (§17.7)
		time.RFC3339,                    // legacy: 2016-11-25T00:00:00Z
		"2006-01-02T15:04:05.000Z07:00", // legacy: with milliseconds
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
			return &d
		}
	}
	return nil
}

// atoi is a panic-free helper for already-regex-validated digit strings.
func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
