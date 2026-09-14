package main

import (
	"fmt"
	"log/slog"

	patchMerge "kun-galgame-patch-api/internal/patch/merge"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Re-running the command after a partial operator session must not fail on the
// ledger rows the previous attempt already wrote.
func upsertRedirect() clause.OnConflict {
	return clause.OnConflict{
		Columns:   []clause.Column{{Name: "scope"}, {Name: "old_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"new_id", "scope"}),
	}
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
				if err := patchMerge.Fold(tx, loser, f.Survivor); err != nil {
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
			if err := patchMerge.Recount(tx, f.Target); err != nil {
				return fmt.Errorf("recount %d: %w", f.Target, err)
			}
		}
		return rewriteMessageLinks(tx)
	})
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
