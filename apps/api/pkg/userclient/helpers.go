package userclient

import (
	"context"
	"log/slog"

	"kun-galgame-patch-api/pkg/upstream"
)

func BriefMapByInt(ctx context.Context, c *Client, ids []int) map[int]*Brief {
	if c == nil || len(ids) == 0 {
		return map[int]*Brief{}
	}
	seen := make(map[uint]struct{}, len(ids))
	clean := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		u := uint(id)
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		clean = append(clean, u)
	}
	if len(clean) == 0 {
		return map[int]*Brief{}
	}
	briefs, err := c.Users(ctx, clean)
	if err != nil {
		LogFailure(ctx, "oauth users/batch failed; user briefs unfilled", err, "count", len(clean))
		return map[int]*Brief{}
	}
	out := make(map[int]*Brief, len(briefs))
	for id, b := range briefs {
		out[int(id)] = b
	}
	return out
}

// LogFailure is for a read that degrades instead of failing. An outage is a
// WARN; moyu's own credential or request is an ERROR, because the reader only
// ever sees blank names and the log is the one place it shows.
func LogFailure(ctx context.Context, msg string, err error, attrs ...any) {
	level := slog.LevelError
	if k := upstream.KindOf(err); k == upstream.Unavailable || k == upstream.RateLimited {
		level = slog.LevelWarn
	}
	slog.Log(ctx, level, msg, append(attrs, "error", err)...)
}
