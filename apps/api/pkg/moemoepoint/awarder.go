package moemoepoint

import (
	"context"
	"errors"
	"log/slog"

	"kun-galgame-patch-api/pkg/upstream"

	"gorm.io/gorm"
)

var errNotConfigured = errors.New("moemoepoint client is not configured")

type Awarder struct {
	client *Client
	db     *gorm.DB
}

func NewAwarder(client *Client, db *gorm.DB) *Awarder {
	return &Awarder{client: client, db: db}
}

func (a *Awarder) Award(ctx context.Context, userID, delta int, reason, ref, idemKey string) {
	if a == nil || a.client == nil || delta == 0 {
		return
	}
	res, err := a.client.Adjust(ctx, userID, AdjustRequest{
		Delta:          delta,
		Reason:         reason,
		Ref:            ref,
		ActorUserID:    0,
		IdempotencyKey: idemKey,
	})
	if err != nil {
		slog.Log(ctx, failureLevel(err), "moemoepoint award failed (best-effort, skipped)",
			"user_id", userID, "delta", delta, "reason", reason, "ref", ref, "key", idemKey, "error", err)
		return
	}
	a.cache(ctx, userID, res.Balance)
}

// A broken awarder has to be loud: a missing awarder flag or a rotated secret
// fails every award on the site, and at WARN it read like the odd transient.
func failureLevel(err error) slog.Level {
	switch upstream.KindOf(err) {
	case upstream.Unavailable, upstream.RateLimited, upstream.NotFound:
		return slog.LevelWarn
	}
	return slog.LevelError
}

func (a *Awarder) Log(ctx context.Context, userID, limit int, beforeID int64, reason string) ([]LogEntry, bool, error) {
	if a == nil || a.client == nil {
		return []LogEntry{}, false, nil
	}
	return a.client.Log(ctx, userID, limit, beforeID, reason)
}

// Balance reads the live balance and brings the cached column up to it, which
// is the only way awards made by other sites ever reach that column.
func (a *Awarder) Balance(ctx context.Context, userID int) (int, error) {
	if a == nil || a.client == nil {
		return 0, errNotConfigured
	}
	balance, err := a.client.Balance(ctx, userID)
	if err != nil {
		return 0, err
	}
	a.cache(ctx, userID, balance)
	return balance, nil
}

// OAuth owns the balance; this local column only mirrors what it last answered.
func (a *Awarder) cache(ctx context.Context, userID, balance int) {
	if err := a.db.WithContext(ctx).
		Exec(`UPDATE "user" SET moemoepoint = ? WHERE id = ? AND moemoepoint <> ?`, balance, userID, balance).Error; err != nil {
		slog.Warn("moemoepoint cache sync failed",
			"user_id", userID, "balance", balance, "error", err)
	}
}
