package utils

import (
	"errors"
	"regexp"
	"strconv"
	"time"
)

// Release-date filter parsing for GET /api/galgame, mirroring the Wiki
// protocol in docs/galgame_wiki/00-handbook §17. moyu can't import Wiki's
// pkg/utils helpers (separate repo), so this reimplements the same YYYY /
// YYYY-MM → date-boundary contract against the local patch.release_date
// column (PG `date`, no time component — so boundaries are plain dates, not
// the 00:00:00 / 23:59:59 timestamps Wiki uses).

// ErrInvalidReleaseBound is returned for malformed released_from/to input
// (e.g. "24", "2024-3" missing zero-pad, "2024-13", "garbage"). The handler
// maps it to a 400 — §17.1 mandates loud rejection, not silent ignore.
var ErrInvalidReleaseBound = errors.New("invalid release date bound (expect YYYY or YYYY-MM)")

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

// ParseWikiReleaseDate turns Wiki's release_date string into a date-only
// *time.Time for storage in patch.release_date. "" / nil / unparseable → nil
// (best-effort: skip rather than abort).
//
// Format: canonical wiki now returns a bare `YYYY-MM-DD` string (per
// docs/galgame_wiki/00-handbook §17.7 — a custom Date type fixed the earlier
// bug where the PG `date` column was serialized as RFC3339 "2016-11-25T00:00:00Z").
// We try the bare-date layout first (the current canonical shape), then fall
// back to RFC3339 (± fractional seconds) to stay robust against older cached
// payloads / pre-fix data — §17.7 explicitly endorses this dual-tolerance.
func ParseWikiReleaseDate(s string) *time.Time {
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
