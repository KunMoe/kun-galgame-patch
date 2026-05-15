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
// Per-batch transaction model:
//   - One tx wraps message-processing + cron_state cursor advance for one
//     batch (up to `batchLimit` messages from a single feed page). On error
//     we abort, the cursor doesn't move, the next tick retries.

import (
	"context"
	"encoding/json"
	"fmt"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"

	"gorm.io/gorm"
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
func RunWikiMessageSync(ctx context.Context, db *gorm.DB, wiki *galgameClient.Client) (int, int64, error) {
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

		err = db.Transaction(func(tx *gorm.DB) error {
			for i := range feed.Items {
				m := &feed.Items[i]
				if err := applyWikiMessage(tx, m); err != nil {
					return err
				}
				if m.ID > cursor {
					cursor = m.ID
				}
				applied++
			}
			// Cursor + side effects move atomically.
			return tx.Exec(`
				INSERT INTO cron_state(name, last_id, updated_at)
				VALUES (?, ?, NOW())
				ON CONFLICT(name) DO UPDATE
				SET last_id = EXCLUDED.last_id, updated_at = EXCLUDED.updated_at
			`, wikiSyncCronName, cursor).Error
		})
		if err != nil {
			return applied, cursor, err
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
func applyWikiMessage(tx *gorm.DB, m *galgameClient.WikiMessage) error {
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

	switch m.Type {
	case "approved":
		if m.TargetUserID != nil {
			if err := tx.Exec(
				`UPDATE "user" SET moemoepoint = moemoepoint + 3 WHERE id = ?`,
				*m.TargetUserID,
			).Error; err != nil {
				return fmt.Errorf("award moemoepoint: %w", err)
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
	link := fmt.Sprintf("/patch/%d/introduction", m.GalgameID)
	return tx.Exec(`
		INSERT INTO user_message(type, content, status, link, sender_id, recipient_id)
		VALUES ('system', ?, 0, ?, NULL, ?)
	`, text, link, m.TargetUserID).Error
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

