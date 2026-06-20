package moemoepoint

import (
	"context"
	"log/slog"

	"gorm.io/gorm"
)

// Awarder applies a moemoepoint change via OAuth (the source of truth), then
// mirrors the returned authoritative balance into moyu's local
// user.moemoepoint read-cache (which ranking / profile / /auth/me still read).
type Awarder struct {
	client *Client
	db     *gorm.DB
}

func NewAwarder(client *Client, db *gorm.DB) *Awarder {
	return &Awarder{client: client, db: db}
}

// Award adjusts the user's unified balance and syncs the local cache.
//
// Best-effort + non-blocking: it must be called OUTSIDE any DB transaction and
// AFTER the triggering action has committed. A failure (including OAuth not yet
// reachable) only logs — it never blocks the caller's core flow, and never
// falls back to a local increment (a local `+=` would double-count after the
// one-time merge migration). Soft karma: a rarely-lost point is acceptable;
// the idempotency key makes a later retry safe.
func (a *Awarder) Award(ctx context.Context, userID, delta int, reason, ref, idemKey string) {
	if a == nil || a.client == nil || delta == 0 {
		return
	}
	res, err := a.client.Adjust(ctx, userID, AdjustRequest{
		Delta:          delta,
		Reason:         reason,
		Ref:            ref,
		ActorUserID:    0, // system
		IdempotencyKey: idemKey,
	})
	if err != nil {
		slog.Warn("moemoepoint award failed (best-effort, skipped)",
			"user_id", userID, "delta", delta, "reason", reason, "ref", ref, "error", err)
		return
	}
	// Mirror the authoritative balance into the local read-cache. Raw SQL keeps
	// this package free of an internal/auth/model import; "user" is quoted
	// (reserved word).
	if err := a.db.WithContext(ctx).
		Exec(`UPDATE "user" SET moemoepoint = ? WHERE id = ?`, res.Balance, userID).Error; err != nil {
		slog.Warn("moemoepoint cache sync failed",
			"user_id", userID, "balance", res.Balance, "error", err)
	}
}

// Log reads a page of the user's moemoepoint ledger from OAuth (the source of
// truth — moyu keeps no local ledger). Read-only passthrough to the s2s
// endpoint; used by the self-service "萌萌点记录" view. A nil Awarder/client
// yields an empty page rather than an error so the UI degrades gracefully.
func (a *Awarder) Log(ctx context.Context, userID, limit int, beforeID int64, reason string) ([]LogEntry, bool, error) {
	if a == nil || a.client == nil {
		return []LogEntry{}, false, nil
	}
	return a.client.Log(ctx, userID, limit, beforeID, reason)
}

// Balance reads the user's current authoritative balance from OAuth (C3 single
// source — NOT the local user.moemoepoint cache). Read-only passthrough; a nil
// Awarder/client yields 0 so a caller using it as one of several OR criteria
// (e.g. creator eligibility) degrades gracefully instead of erroring.
func (a *Awarder) Balance(ctx context.Context, userID int) (int, error) {
	if a == nil || a.client == nil {
		return 0, nil
	}
	return a.client.Balance(ctx, userID)
}
