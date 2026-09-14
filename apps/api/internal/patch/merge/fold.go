// Package merge folds one patch page into another. Two callers need exactly
// these semantics: cmd/align-patch-ids, which folded the pages that turned out
// to name one work during the 037 renumber, and the catalog merge cron, which
// folds a page whose work catalog has since merged away.
package merge

import (
	"fmt"
	"log/slog"

	"gorm.io/gorm"
)

// A child table with nothing unique per game: both pages' rows can coexist
// under the survivor, so every row simply moves.
var movableChildren = []string{"patch_resource", "patch_comment"}

// A child table with a unique key over (galgame_id, peer). When the survivor
// already has a row for that peer the two cannot both live, so the loser's row
// is dropped rather than moved. Nothing here carries user-written text -- these
// are join rows -- which is why they are counted and logged rather than
// archived the way the forum archives its ratings.
var uniqueChildren = []struct{ table, peer string }{
	{"patch_link", "name"},
	{"user_patch_contribute_relation", "user_id"},
	{"user_patch_favorite_relation", "user_id"},
}

// Fold moves everything the loser page carries onto the survivor and deletes
// the loser. It leaves patch_redirect alone: which scope records the move is
// the caller's decision, and the two callers answer different URLs.
func Fold(tx *gorm.DB, loser, survivor int) error {
	for _, table := range movableChildren {
		if err := tx.Exec(
			fmt.Sprintf("UPDATE %s SET galgame_id = ? WHERE galgame_id = ?", table),
			survivor, loser,
		).Error; err != nil {
			return err
		}
	}
	for _, t := range uniqueChildren {
		if err := tx.Exec(fmt.Sprintf(`
			UPDATE %[1]s SET galgame_id = ? WHERE galgame_id = ?
			  AND NOT EXISTS (
				SELECT 1 FROM %[1]s x WHERE x.galgame_id = ? AND x.%[2]s = %[1]s.%[2]s)`,
			t.table, t.peer), survivor, loser, survivor).Error; err != nil {
			return err
		}
		dropped := tx.Exec(fmt.Sprintf("DELETE FROM %s WHERE galgame_id = ?", t.table), loser)
		if dropped.Error != nil {
			return dropped.Error
		}
		if dropped.RowsAffected > 0 {
			slog.Info("合并丢弃重复的关系行",
				"table", t.table, "rows", dropped.RowsAffected, "loser", loser, "survivor", survivor)
		}
	}

	// favorite_count is added, not recounted. Favourites live in catalog
	// folders since the cutover and user_patch_favorite_relation is frozen
	// rollback material, so counting it would replace a live counter with a
	// snapshot. status takes the stricter of the two: a hidden or disabled page
	// must not shed that by being merged into a clean duplicate.
	if err := tx.Exec(`
		UPDATE patch t SET
			view                 = t.view + s.view,
			download             = t.download + s.download,
			favorite_count       = t.favorite_count + s.favorite_count,
			published            = t.published OR s.published,
			status               = GREATEST(t.status, s.status),
			is_stub              = t.is_stub AND s.is_stub,
			creator_id           = COALESCE(t.creator_id, s.creator_id),
			release_date         = COALESCE(t.release_date, s.release_date),
			created              = LEAST(t.created, s.created),
			resource_update_time = GREATEST(t.resource_update_time, s.resource_update_time)
		FROM patch s WHERE t.id = ? AND s.id = ?`, survivor, loser).Error; err != nil {
		return err
	}

	return tx.Exec("DELETE FROM patch WHERE id = ?", loser).Error
}

// Recount rebuilds the counters a fold invalidates. Call it on the survivor
// once every loser has been folded in.
func Recount(tx *gorm.DB, patchID int) error {
	return tx.Exec(`
		UPDATE patch SET
			resource_count   = (SELECT count(*) FROM patch_resource WHERE galgame_id = patch.id),
			comment_count    = (SELECT count(*) FROM patch_comment WHERE galgame_id = patch.id),
			contribute_count = (SELECT count(*) FROM user_patch_contribute_relation WHERE galgame_id = patch.id)
		WHERE id = ?`, patchID).Error
}
