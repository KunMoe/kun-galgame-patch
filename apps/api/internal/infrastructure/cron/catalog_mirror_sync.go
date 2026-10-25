package cron

import (
	"context"
	"fmt"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/pkg/catalogv2"

	"gorm.io/gorm"
)

const (
	// Its own cursor namespace: this feed pages by opaque cur_ strings over
	// catalog_work, the claim feed by an integer event id. Sharing a row would
	// hand one of them the other's watermark. The _v2 row starts a fresh drain:
	// verdicts written under the old name came from content_rating whenever a
	// work had no kungal claim, and only a walk from an empty cursor revisits
	// every one of them.
	mirrorSyncCronName = "catalog_display_mirror_v2"
	// Staggered off the claim sync's */10 so the two do not queue behind each
	// other on the same catalog origin.
	mirrorSyncSchedule = "3-59/10 * * * *"
	mirrorSyncPageSize = catalogv2.ChangesMaxLimit
	// The first drain walks catalog's whole galgame population — ~63k works,
	// not the ~10k moyu carries — because an empty cursor is how the feed
	// bootstraps. Bounded per tick at 50 pages (100 requests), so the backlog
	// clears over a couple of hours instead of in one long call. Steady state
	// is a single page that comes back short.
	mirrorSyncMaxPages = 50
)

// RunCatalogMirrorSync mirrors catalog's display verdict onto patch.content_limit
// so the galgame lists can filter the axis in SQL instead of losing rows after
// the LIMIT. Catalog's own gate at hydrate time stays the authority; this column
// only decides which ids reach it, which is what makes a stale value cost a
// short row rather than an NSFW leak. The same drain stamps a page whose work
// catalog deleted, and lifts the stamp when the work is restored (051).
func RunCatalogMirrorSync(
	ctx context.Context,
	db *gorm.DB,
	catalog *catalogv2.Client,
	galgame *galgameClient.Client,
) (updated int, caughtUp bool, err error) {
	if db == nil || catalog == nil || galgame == nil || !catalog.Configured() {
		return 0, false, fmt.Errorf("catalog mirror sync: missing db, catalog client or galgame client")
	}

	cursor, err := readMirrorCursor(db)
	if err != nil {
		return 0, false, err
	}

	for page := 0; page < mirrorSyncMaxPages; page++ {
		feed, ferr := catalog.Changes(ctx, cursor, mirrorSyncPageSize)
		if ferr != nil {
			return updated, false, fmt.Errorf("fetch changes feed: %w", ferr)
		}
		live := make([]int64, 0, len(feed.Items))
		var gone []int64
		for i := range feed.Items {
			if feed.Items[i].Gone {
				gone = append(gone, feed.Items[i].ID)
				continue
			}
			live = append(live, feed.Items[i].ID)
		}
		n, aerr := applyDisplayVerdicts(ctx, db, galgame, live)
		updated += n
		if aerr != nil {
			return updated, false, aerr
		}
		n, aerr = clearRestoredPages(db, live)
		updated += n
		if aerr != nil {
			return updated, false, aerr
		}
		n, aerr = markDeletedPages(ctx, db, catalog, gone)
		updated += n
		if aerr != nil {
			return updated, false, aerr
		}
		// A short page has no next cursor, so the watermark stays at the last
		// full page and the tail is re-read next tick. Re-applying it is free:
		// every UPDATE matches nothing when the state has not moved.
		if feed.NextCursor == "" {
			caughtUp = true
			break
		}
		cursor = feed.NextCursor
		if werr := writeMirrorCursor(db, cursor); werr != nil {
			return updated, false, werr
		}
	}
	return updated, caughtUp, nil
}

func applyDisplayVerdicts(
	ctx context.Context,
	db *gorm.DB,
	galgame *galgameClient.Client,
	ids []int64,
) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	rows, err := galgame.DisplayVerdictsByCatalogIDs(ctx, ids)
	if err != nil {
		return 0, fmt.Errorf("hydrate display verdicts: %w", err)
	}
	byLimit := map[string][]int{}
	for _, r := range rows {
		switch r.ContentLimit {
		case "sfw", "nsfw":
			byLimit[r.ContentLimit] = append(byLimit[r.ContentLimit], r.GID)
		}
	}
	updated := 0
	for limit, gids := range byLimit {
		// Raw SQL, not db.Model(&Patch{}).UpdateColumn: Patch.ContentLimit is
		// read-only (`->`), so GORM dropped the only assignment and skipped the
		// statement with RowsAffected 0 and no error. Every verdict from
		// 2026-09-08 to 2026-10-25 was discarded that way while ticks logged
		// updated=0.
		res := db.Exec(
			`UPDATE patch SET content_limit = ? WHERE id IN ? AND content_limit IS DISTINCT FROM ?`,
			limit, gids, limit,
		)
		if res.Error != nil {
			return updated, fmt.Errorf("write content_limit=%s: %w", limit, res.Error)
		}
		updated += int(res.RowsAffected)
	}
	return updated, nil
}

func readMirrorCursor(db *gorm.DB) (string, error) {
	var rows []string
	if err := db.Raw(
		`SELECT COALESCE(last_cursor, '') FROM cron_state WHERE name = ?`, mirrorSyncCronName,
	).Scan(&rows).Error; err != nil {
		return "", fmt.Errorf("read mirror cursor: %w", err)
	}
	if len(rows) == 0 {
		return "", nil
	}
	return rows[0], nil
}

func writeMirrorCursor(db *gorm.DB, cursor string) error {
	return db.Exec(`
		INSERT INTO cron_state(name, last_id, last_cursor, updated_at)
		VALUES (?, 0, ?, NOW())
		ON CONFLICT(name) DO UPDATE
		SET last_cursor = EXCLUDED.last_cursor, updated_at = EXCLUDED.updated_at
	`, mirrorSyncCronName, cursor).Error
}
