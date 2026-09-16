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
//
// patch_comment is NOT here any more. The loser's comment wall is a thread in
// the community primitive, anchored on the loser's page id, and no SQL in this
// database can move it. Fold logs the stranded wall instead; infra's
// cmd/retire-merged-comments sweeps it, and knows this site by name — a moyu
// site_game anchor IS a catalog work id (铁律 3), so the "a product id that
// merely collides" exclusion the forum needs is off for moyu.
var movableChildren = []string{"patch_resource"}

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
	var strandedComments int64
	tx.Table("patch").Select("comment_count").Where("id = ?", loser).Scan(&strandedComments)
	if strandedComments > 0 {
		slog.Warn("合并遗留了一面评论墙：community 的串仍锚在被合并页上，本库无法搬移",
			"loser", loser, "survivor", survivor, "comments", strandedComments,
			"site", "moyu", "anchor_kind", 1, "anchor_id", loser)
	}

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

	// vndb_id and bangumi_id are UNIQUE, so the survivor can only adopt an
	// anchor after the loser has stopped holding it. Catalog moves the retired
	// work's external refs onto the survivor in the same transaction as the
	// merge; dropping them here leaves the /v2/moyu face answering "no page"
	// for a vndb id catalog says this site covers.
	var gone struct {
		VndbID    string
		BangumiID *int
	}
	if err := tx.Raw(
		`DELETE FROM patch WHERE id = ? RETURNING vndb_id, bangumi_id`, loser,
	).Scan(&gone).Error; err != nil {
		return err
	}
	return tx.Exec(`
		UPDATE patch SET
			vndb_id    = CASE WHEN vndb_id !~ '^v[0-9]+$' AND ? ~ '^v[0-9]+$'
			                  THEN ? ELSE vndb_id END,
			bangumi_id = COALESCE(bangumi_id, ?)
		WHERE id = ?`, gone.VndbID, gone.VndbID, gone.BangumiID, survivor).Error
}

// Recount rebuilds what a fold invalidates on the survivor: the three counters,
// and the facet arrays the browse filters read. patch.type / language /
// platform are aggregated from the page's own resources and /galgame?language=
// reads those arrays, never the resources -- so moving resources across without
// recomputing them leaves the survivor unfilterable by everything it just
// gained. Call it once every loser has been folded in.
func Recount(tx *gorm.DB, patchID int) error {
	// comment_count is absent on purpose: it counts posts in the community
	// primitive, which this query cannot see. Recomputing it from the frozen
	// patch_comment would reset the survivor to its pre-cutover snapshot.
	return tx.Exec(`
		UPDATE patch SET
			resource_count   = (SELECT count(*) FROM patch_resource WHERE galgame_id = patch.id),
			contribute_count = (SELECT count(*) FROM user_patch_contribute_relation WHERE galgame_id = patch.id),
			type             = `+resourceFacet("type")+`,
			language         = `+resourceFacet("language")+`,
			platform         = `+resourceFacet("platform")+`
		WHERE id = ?`, patchID).Error
}

// resourceFacet is the derivation PatchRepository.RecalculatePatchAggregates
// does in Go, as SQL so it can run inside the fold's transaction.
func resourceFacet(column string) string {
	return fmt.Sprintf(`COALESCE((
		SELECT jsonb_agg(DISTINCT e.v ORDER BY e.v)
		FROM patch_resource r
		CROSS JOIN LATERAL jsonb_array_elements_text(r.%s) AS e(v)
		WHERE r.galgame_id = patch.id), '[]'::jsonb)`, column)
}
