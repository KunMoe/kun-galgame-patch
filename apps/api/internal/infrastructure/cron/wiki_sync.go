package cron

// Wiki messages → local notifications + moemoepoint sync.
//
// Pulls /galgame/messages/feed every ~10 minutes (decision per
// docs/galgame_wiki/00-handbook-for-downstream.md §7) and applies the four
// admin-triggered events that target a specific user:
//
//   approved   → +3 moemoepoint, write local "approved" notification
//   declined   → write local "declined" notification (no moemoepoint reversal)
//   banned     → write local "banned" notification (no moemoepoint reversal)
//   unbanned   → write local "unbanned" notification
//
// Idempotency: each Wiki message_id is inserted into wiki_message_processed
// inside the same tx as its side effects. A re-run sees the row already
// present and skips, so a crash mid-batch can safely be retried.
//
// Per-MESSAGE transaction model (changed 2026-05-30, audit F025):
//   - One tx wraps a SINGLE message's idempotency insert + side effects +
//     cron_state cursor advance. They still commit atomically (exactly-once
//     preserved), but the synchronous OAuth award HTTP inside applyWikiMessage
//     is now bounded to ONE call per open tx instead of up to `wikiBatchLimit`
//     (1000). A slow/erroring OAuth can no longer pin one DB connection or hold
//     row locks across a whole feed page, and a poison message stops the run
//     without re-rolling-back (and re-awarding) the messages committed before
//     it. On error we abort the loop, the cursor stays at the last committed
//     message, and the next tick retries from there.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	authModel "kun-galgame-patch-api/internal/auth/model"
	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	userModel "kun-galgame-patch-api/internal/user/model"
	"kun-galgame-patch-api/pkg/moemoepoint"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	wikiSyncCronName = "wiki_msg_sync"
	wikiSyncSchedule = "*/10 * * * *" // every 10 minutes
	wikiBatchLimit   = 1000           // wiki caps at 5000; 1000 keeps a single tx small
)

// RunWikiMessageSync is exported so it can be called manually (e.g. for tests
// or one-off backfills). The cron path is the StartWikiSync wrapper below.
//
// Returns the number of messages applied + the new cursor value.
func RunWikiMessageSync(ctx context.Context, db *gorm.DB, wiki *galgameClient.Client, mp *moemoepoint.Client) (int, int64, error) {
	if wiki == nil || db == nil {
		return 0, 0, fmt.Errorf("wiki sync: missing wiki client or db")
	}

	var sinceID int64
	// Bootstrap the row on first run; ON CONFLICT keeps the existing cursor.
	if err := db.Exec(`
		INSERT INTO cron_state(name, last_id) VALUES (?, 0)
		ON CONFLICT(name) DO NOTHING
	`, wikiSyncCronName).Error; err != nil {
		return 0, 0, fmt.Errorf("seed cron_state: %w", err)
	}
	if err := db.Raw(
		`SELECT last_id FROM cron_state WHERE name = ?`, wikiSyncCronName,
	).Scan(&sinceID).Error; err != nil {
		return 0, 0, fmt.Errorf("read cron cursor: %w", err)
	}

	applied := 0
	cursor := sinceID
	for {
		feed, err := wiki.GetWikiMessageFeed(ctx, cursor, wikiBatchLimit)
		if err != nil {
			return applied, cursor, fmt.Errorf("fetch feed: %w", err)
		}
		if len(feed.Items) == 0 {
			break
		}

		for i := range feed.Items {
			m := &feed.Items[i]
			next := cursor
			if m.ID > next {
				next = m.ID
			}
			// One tx per message: its idempotency insert, side effects, and the
			// cursor advance commit together (exactly-once), but the OAuth award
			// HTTP is bounded to a single in-tx call. See the file header (F025).
			txErr := db.Transaction(func(tx *gorm.DB) error {
				if err := applyWikiMessage(ctx, tx, mp, m); err != nil {
					return err
				}
				return tx.Exec(`
					INSERT INTO cron_state(name, last_id, updated_at)
					VALUES (?, ?, NOW())
					ON CONFLICT(name) DO UPDATE
					SET last_id = EXCLUDED.last_id, updated_at = EXCLUDED.updated_at
				`, wikiSyncCronName, next).Error
			})
			if txErr != nil {
				return applied, cursor, txErr
			}
			cursor = next
			applied++
		}

		if !feed.HasMore {
			break
		}
	}
	return applied, cursor, nil
}

// applyWikiMessage handles a single Wiki message inside an open tx. It is the
// idempotency boundary: a non-zero RowsAffected on the INSERT means this is
// the first time we're seeing this message — only then do we run the side
// effects. Repeats short-circuit.
func applyWikiMessage(ctx context.Context, tx *gorm.DB, mp *moemoepoint.Client, m *galgameClient.WikiMessage) error {
	// Idempotency gate. ON CONFLICT DO NOTHING; if RowsAffected==0 we already
	// applied this message in a prior run (or a prior tx in this batch).
	res := tx.Exec(`
		INSERT INTO wiki_message_processed(message_id) VALUES (?)
		ON CONFLICT(message_id) DO NOTHING
	`, m.ID)
	if res.Error != nil {
		return fmt.Errorf("idempotency insert: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil
	}

	// "Ghost messages": the galgame was hard-deleted between event emission
	// and read. Per docs §7 we clean up any stale local state and skip.
	// (Our local-stats equivalent is the `patch` row itself, lazy-loaded.)
	if m.Galgame == nil {
		return nil
	}

	// Actionable events must carry the target user. A nil here is a malformed/
	// edge feed row; log it (kungal does the same) instead of silently
	// consuming the idempotency marker with no effect. (F068)
	if m.TargetUserID == nil {
		switch m.Type {
		case "approved", "declined", "banned", "unbanned":
			slog.Warn("wiki actionable message has nil target_user_id; consumed with no effect",
				"message_id", m.ID, "type", m.Type)
		}
		return nil
	}

	// Ensure the target has a LOCAL user anchor before any user-FK'd write
	// below (the user_message notification insert, and the approved-path
	// moemoepoint cache UPDATE). A wiki message can target someone who exists
	// in OAuth (they submitted to the wiki) but has NEVER logged into moyu, so
	// no local `user` row exists yet → user_message_recipient_id_fkey (23503)
	// rolls the whole per-message tx back, the feed cursor never advances, and
	// the sync wedges permanently — starving every later message of its
	// notification + moemoepoint award (observed in prod: ~90 failures/72h,
	// stuck at cursor 98). Provision a stub {ID} row (all other columns default;
	// enriched on the user's next moyu login) — the same FK-anchor pattern patch
	// ownership uses (service.ensureLocalPatch). OnConflict DoNothing so an
	// existing row is left untouched.
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&authModel.User{ID: *m.TargetUserID}).Error; err != nil {
		return fmt.Errorf("ensure recipient user anchor (uid=%d): %w", *m.TargetUserID, err)
	}

	switch m.Type {
	case "approved":
		if m.TargetUserID != nil {
			// +3 via OAuth (unified source of truth). This is the canonical
			// replay-safe path (doc §4): the per-message idempotency_key makes a
			// retry a no-op, and a failure here returns an error → the batch tx
			// rolls back → the cron retries this message next run (cursor only
			// advances on commit). On success we mirror the authoritative balance
			// into the local cache within the same tx. If OAuth isn't configured
			// (mp == nil) we skip the award rather than write a local-only +3
			// that would desync from the unified balance.
			if mp != nil {
				res, err := mp.Adjust(ctx, *m.TargetUserID, moemoepoint.AdjustRequest{
					Delta:          3,
					Reason:         "content_approved",
					Ref:            fmt.Sprintf("galgame:%d", m.Galgame.ID),
					IdempotencyKey: fmt.Sprintf("moyu:wiki_approved:%d", m.ID),
				})
				if err != nil {
					return fmt.Errorf("award moemoepoint: %w", err)
				}
				if err := tx.Exec(
					`UPDATE "user" SET moemoepoint = ? WHERE id = ?`,
					res.Balance, *m.TargetUserID,
				).Error; err != nil {
					return fmt.Errorf("sync moemoepoint cache: %w", err)
				}
			}
			if err := writeWikiNotification(tx, m, displayGalgameName(m.Galgame),
				"您提交的《%s》已通过审核，奖励 +3 萌萌点"); err != nil {
				return err
			}
		}
	case "declined":
		if m.TargetUserID != nil {
			reason := payloadString(m.Payload, "reason")
			name := displayGalgameName(m.Galgame)
			text := fmt.Sprintf("您提交的《%s》未通过审核", name)
			if reason != "" {
				text += "：" + reason
			}
			if err := writeWikiNotificationRaw(tx, m, text); err != nil {
				return err
			}
		}
	case "banned":
		if m.TargetUserID != nil {
			reason := payloadString(m.Payload, "reason")
			name := displayGalgameName(m.Galgame)
			text := fmt.Sprintf("您的作品《%s》已被封禁", name)
			if reason != "" {
				text += "：" + reason
			}
			if err := writeWikiNotificationRaw(tx, m, text); err != nil {
				return err
			}
		}
	case "unbanned":
		if m.TargetUserID != nil {
			if err := writeWikiNotification(tx, m, displayGalgameName(m.Galgame),
				"您的作品《%s》已解除封禁"); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeWikiNotification inserts a local user_message row pointing at the
// patch page of the galgame so the user can jump in one click.
func writeWikiNotification(tx *gorm.DB, m *galgameClient.WikiMessage, name, format string) error {
	return writeWikiNotificationRaw(tx, m, fmt.Sprintf(format, name))
}

func writeWikiNotificationRaw(tx *gorm.DB, m *galgameClient.WikiMessage, text string) error {
	// Use GORM Create so the model's autoCreateTime / autoUpdateTime tags
	// populate `created` / `updated`. Raw tx.Exec bypasses those hooks and
	// the DB rejects the insert (both columns are NOT NULL without DEFAULT).
	link := fmt.Sprintf("/patch/%d/introduction", m.GalgameID)
	return tx.Create(&userModel.UserMessage{
		Type:        "system",
		Content:     text,
		Status:      0,
		Link:        link,
		SenderID:    nil,
		RecipientID: m.TargetUserID,
	}).Error
}

// payloadString pulls a scalar string field out of the Wiki message payload
// JSON. Returns "" when missing or wrong-typed (no error — payload fields are
// best-effort enrichment, not contract).
func payloadString(raw json.RawMessage, key string) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// displayGalgameName picks the first non-empty translation for the local
// notification body. Falls back to the integer id so the message never reads
// "您提交的《》已通过审核".
func displayGalgameName(g *galgameClient.WikiMessageGalgame) string {
	if g == nil {
		return ""
	}
	for _, s := range []string{g.NameZhCn, g.NameZhTw, g.NameJaJp, g.NameEnUs} {
		if s != "" {
			return s
		}
	}
	return fmt.Sprintf("#%d", g.ID)
}
