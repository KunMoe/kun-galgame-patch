package rating

import (
	"context"
	"fmt"
	"log/slog"

	"kun-galgame-patch-api/pkg/catalogv2"

	"gorm.io/gorm"
)

const (
	// Deleting this row replays every rating NextMoe holds, which the upsert
	// absorbs; it is how the mirror is rebuilt.
	mirrorCronName = "catalog_rating_mirror"
	mirrorMaxPages = 100
)

type Feed interface {
	Configured() bool
	RatingChanges(ctx context.Context, cursor string) (catalogv2.RatingChangePage, error)
}

type Mirror struct {
	db    *gorm.DB
	feed  Feed
	store *Store
}

func NewMirror(db *gorm.DB, feed Feed, store *Store) *Mirror {
	return &Mirror{db: db, feed: feed, store: store}
}

func (m *Mirror) Configured() bool {
	return m != nil && m.feed != nil && m.feed.Configured()
}

func (m *Mirror) Sync(ctx context.Context) (int, error) {
	cursor, err := m.readCursor()
	if err != nil {
		return 0, err
	}
	applied := 0
	for range mirrorMaxPages {
		page, err := m.feed.RatingChanges(ctx, cursor)
		if err != nil {
			return applied, fmt.Errorf("fetch ratings changes: %w", err)
		}
		n, err := m.apply(page.Items)
		applied += n
		if err != nil {
			return applied, err
		}
		// Only a full page carries a cursor, so a short one is read again next
		// tick and the upsert finds nothing to change.
		if page.NextCursor == "" {
			return applied, nil
		}
		cursor = page.NextCursor
		if err := m.writeCursor(cursor); err != nil {
			return applied, err
		}
	}
	return applied, nil
}

func (m *Mirror) apply(items []catalogv2.RatingChange) (int, error) {
	applied := 0
	for _, it := range items {
		var changed bool
		var err error
		if it.Gone || it.Rating == nil {
			changed, err = m.store.Drop(it.ID)
		} else if it.Rating.WorkID <= 0 || it.Rating.UserID <= 0 {
			slog.Warn("ratings mirror: skipped a rating without its work or author", "rating_id", it.ID)
			continue
		} else {
			changed, err = m.store.Mirror(it.Rating)
		}
		if err != nil {
			return applied, fmt.Errorf("mirror rating %d: %w", it.ID, err)
		}
		if changed {
			applied++
		}
	}
	return applied, nil
}

func (m *Mirror) readCursor() (string, error) {
	var rows []string
	if err := m.db.Raw(`SELECT COALESCE(last_cursor, '') FROM cron_state WHERE name = ?`, mirrorCronName).
		Scan(&rows).Error; err != nil {
		return "", fmt.Errorf("read ratings mirror cursor: %w", err)
	}
	if len(rows) == 0 {
		return "", nil
	}
	return rows[0], nil
}

func (m *Mirror) writeCursor(cursor string) error {
	return m.db.Exec(`
		INSERT INTO cron_state(name, last_id, last_cursor, updated_at)
		VALUES (?, 0, ?, NOW())
		ON CONFLICT(name) DO UPDATE
		SET last_cursor = EXCLUDED.last_cursor, updated_at = EXCLUDED.updated_at
	`, mirrorCronName, cursor).Error
}
