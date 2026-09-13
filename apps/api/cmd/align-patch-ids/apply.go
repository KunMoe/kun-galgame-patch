package main

import (
	"fmt"
	"log/slog"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Re-running the command after a partial operator session must not fail on the
// ledger rows the previous attempt already wrote.
func upsertRedirect() clause.OnConflict {
	return clause.OnConflict{
		Columns:   []clause.Column{{Name: "old_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"new_id", "scope"}),
	}
}

// A child table with nothing unique per game: both pages' rows can coexist
// under the survivor, so every row simply moves.
var foldMovable = []string{"patch_resource", "patch_comment"}

// A child table with a unique key over (galgame_id, peer). When the survivor
// already has a row for that peer the two cannot both live, so the loser's row
// is dropped rather than moved. Nothing here carries user-written text -- these
// are join rows -- which is why they are counted and logged rather than
// archived the way the forum archives its ratings.
var foldUnique = []struct{ table, peer string }{
	{"patch_link", "name"},
	{"user_patch_contribute_relation", "user_id"},
	{"user_patch_favorite_relation", "user_id"},
}

// Everything this transaction touches, parents before children. The first
// production attempt died 57 seconds into the park hop with "ERROR: deadlock
// detected (SQLSTATE 40P01)" and rolled the whole run back: a page view
// increments patch.view and reaches the same rows from the other side, and the
// renumber holds them for minutes. Taking the tables up front makes every other
// writer queue instead of racing. Stop the API for the window regardless --
// this only turns a lost run into a stalled site.
const lockEverything = `LOCK TABLE
	patch, patch_resource, patch_comment, patch_link,
	user_patch_contribute_relation, user_patch_favorite_relation, user_message
	IN ACCESS EXCLUSIVE MODE`

func applyPlan(db *gorm.DB, p *Plan) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(lockEverything).Error; err != nil {
			return fmt.Errorf("lock: %w", err)
		}
		for _, f := range p.Folds {
			for _, loser := range f.Losers {
				if err := foldPatch(tx, loser, f.Survivor); err != nil {
					return fmt.Errorf("fold %d -> %d: %w", loser, f.Survivor, err)
				}
			}
		}
		if err := renumber(tx, p.Moves); err != nil {
			return err
		}
		if err := writeLedger(tx, p.Redirects); err != nil {
			return err
		}
		for _, f := range p.Folds {
			if err := recount(tx, f.Target); err != nil {
				return fmt.Errorf("recount %d: %w", f.Target, err)
			}
		}
		return rewriteMessageLinks(tx)
	})
}

func foldPatch(tx *gorm.DB, loser, survivor int) error {
	for _, table := range foldMovable {
		if err := tx.Exec(
			fmt.Sprintf("UPDATE %s SET galgame_id = ? WHERE galgame_id = ?", table),
			survivor, loser,
		).Error; err != nil {
			return err
		}
	}
	for _, t := range foldUnique {
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

// renumber moves every page in two hops. Landing directly is impossible:
// 9,519 of the old ids are also somebody's new id, so whichever row goes first
// lands on a number another row still occupies. The children come along on
// their own -- migration 037 made the foreign keys ON UPDATE CASCADE.
func renumber(tx *gorm.DB, moves []move) error {
	if len(moves) == 0 {
		return nil
	}
	if err := tx.Exec(`CREATE TEMP TABLE align_map (
		old_id integer PRIMARY KEY, new_id integer NOT NULL UNIQUE
	) ON COMMIT DROP`).Error; err != nil {
		return err
	}
	const chunk = 1000
	for start := 0; start < len(moves); start += chunk {
		end := min(start+chunk, len(moves))
		rows := make([]map[string]any, 0, end-start)
		for _, m := range moves[start:end] {
			rows = append(rows, map[string]any{"old_id": m.OldID, "new_id": m.NewID})
		}
		if err := tx.Table("align_map").Create(rows).Error; err != nil {
			return fmt.Errorf("stage mapping: %w", err)
		}
	}

	parked := tx.Exec(`
		UPDATE patch SET id = id + ?
		WHERE id IN (SELECT old_id FROM align_map)`, parkBase)
	if parked.Error != nil {
		return fmt.Errorf("park: %w", parked.Error)
	}
	if parked.RowsAffected != int64(len(moves)) {
		return fmt.Errorf("park moved %d rows, expected %d", parked.RowsAffected, len(moves))
	}

	landed := tx.Exec(`
		UPDATE patch SET id = m.new_id
		FROM align_map m WHERE patch.id = m.old_id + ?`, parkBase)
	if landed.Error != nil {
		return fmt.Errorf("land: %w", landed.Error)
	}
	if landed.RowsAffected != int64(len(moves)) {
		return fmt.Errorf("land moved %d rows, expected %d", landed.RowsAffected, len(moves))
	}
	slog.Info("改号完成", "rows", landed.RowsAffected)
	return nil
}

func writeLedger(tx *gorm.DB, redirects []move) error {
	const chunk = 1000
	for start := 0; start < len(redirects); start += chunk {
		end := min(start+chunk, len(redirects))
		rows := make([]map[string]any, 0, end-start)
		for _, r := range redirects[start:end] {
			rows = append(rows, map[string]any{
				"old_id": r.OldID, "new_id": r.NewID, "scope": "legacy",
			})
		}
		if err := tx.Table("patch_redirect").
			Clauses(upsertRedirect()).Create(rows).Error; err != nil {
			return fmt.Errorf("write ledger: %w", err)
		}
	}
	return nil
}

func recount(tx *gorm.DB, patchID int) error {
	return tx.Exec(`
		UPDATE patch SET
			resource_count   = (SELECT count(*) FROM patch_resource WHERE galgame_id = patch.id),
			comment_count    = (SELECT count(*) FROM patch_comment WHERE galgame_id = patch.id),
			contribute_count = (SELECT count(*) FROM user_patch_contribute_relation WHERE galgame_id = patch.id)
		WHERE id = ?`, patchID).Error
}

// The canonical page is /galgame/<id> and it has no sub-routes any more, so a
// notification that pointed at /patch/<gid>/resource has to carry the tab in a
// query instead. 121,222 of production's 126,000 links are patch links, and
// they have no foreign key -- nothing would have moved them.
func rewriteMessageLinks(tx *gorm.DB) error {
	res := tx.Exec(`
		WITH parsed AS (
			SELECT id,
			       m[1]::int                AS gid,
			       COALESCE(m[2], '')       AS seg,
			       COALESCE(m[3], '')       AS anchor
			FROM user_message,
			     LATERAL regexp_match(link, '^/patch/([0-9]+)(?:/([a-z]+))?(#.*)?$') AS m
			WHERE link ~ '^/patch/[0-9]+'
		)
		UPDATE user_message t
		SET link = '/galgame/' || COALESCE(r.new_id, p.gid)::text
		         || CASE p.seg
		              WHEN 'resource' THEN '?tab=resource'
		              WHEN 'comment'  THEN '?tab=comment'
		              ELSE ''
		            END
		         || p.anchor
		FROM parsed p LEFT JOIN patch_redirect r ON r.old_id = p.gid AND r.scope = 'legacy'
		WHERE t.id = p.id`)
	if res.Error != nil {
		return fmt.Errorf("rewrite notification links: %w", res.Error)
	}
	slog.Info("站内通知链接已重写", "rows", res.RowsAffected)
	return nil
}
