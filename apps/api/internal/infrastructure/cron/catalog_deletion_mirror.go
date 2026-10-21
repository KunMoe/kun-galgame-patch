package cron

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"kun-galgame-patch-api/pkg/catalogv2"

	"gorm.io/gorm"
)

type workDeletions interface {
	WorkDeletion(ctx context.Context, id int64) (*catalogv2.Deletion, error)
}

// markDeletedPages stamps the pages whose work the changes feed reported gone
// and catalog confirms deleted. Gone also means merged away, which the merge
// cron follows, or hidden, which hydration already drops; neither is stamped.
func markDeletedPages(ctx context.Context, db *gorm.DB, catalog workDeletions, gone []int64) (int, error) {
	if len(gone) == 0 {
		return 0, nil
	}
	var pages []int64
	if err := db.Raw(
		`SELECT id FROM patch WHERE id IN ? AND catalog_deleted_at IS NULL`, gone,
	).Scan(&pages).Error; err != nil {
		return 0, fmt.Errorf("look up gone pages: %w", err)
	}
	stamped := 0
	for _, id := range pages {
		d, err := catalog.WorkDeletion(ctx, id)
		if err != nil {
			if _, merged := catalogv2.MergedInto(err); merged || catalogv2.IsNotFound(err) {
				continue
			}
			return stamped, fmt.Errorf("ask catalog whether work %d was deleted: %w", id, err)
		}
		if d == nil {
			continue
		}
		at, perr := time.Parse(time.RFC3339, d.DeletedAt)
		if perr != nil {
			at = time.Now()
		}
		res := db.Exec(`
			UPDATE patch SET catalog_deleted_at = ?, catalog_deleted_reason = ?
			WHERE id = ? AND catalog_deleted_at IS NULL`, at, d.Reason, id)
		if res.Error != nil {
			return stamped, fmt.Errorf("stamp deleted page %d: %w", id, res.Error)
		}
		if res.RowsAffected > 0 {
			stamped++
			slog.Info("catalog 删除了作品，本站页面随之隐藏", "work_id", id, "reason", d.Reason)
		}
	}
	return stamped, nil
}

// clearRestoredPages lifts the stamp from pages whose work the feed reports
// live again: catalog restores a deleted work under the same id.
func clearRestoredPages(db *gorm.DB, live []int64) (int, error) {
	if len(live) == 0 {
		return 0, nil
	}
	res := db.Exec(`
		UPDATE patch SET catalog_deleted_at = NULL, catalog_deleted_reason = NULL
		WHERE id IN ? AND catalog_deleted_at IS NOT NULL`, live)
	if res.Error != nil {
		return 0, fmt.Errorf("clear restored pages: %w", res.Error)
	}
	return int(res.RowsAffected), nil
}
