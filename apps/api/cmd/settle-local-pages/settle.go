package main

import (
	"fmt"

	patchMerge "kun-galgame-patch-api/internal/patch/merge"
	patchModel "kun-galgame-patch-api/internal/patch/model"

	"gorm.io/gorm"
)

type settlement struct {
	Local  int
	Target int
	// Vndb replaces the page's vndb_id on a renumber. Empty keeps it: a stale
	// wiki-N is rewritten by refreshWikiAnchors afterwards anyway.
	Vndb string
}

// Every target is the retired wiki's galgame.catalog_work_id, a catalog merge
// of the parked number, or an exact external ref, each checked by title. The N
// in a wiki-N vndb_id is the old wiki gid, not a work id: reading it as one sent
// three of these to a different game in the census (work 61345 is 箱舟キバウ,
// the page is Grand Order 麻雀).
var settlements = []settlement{
	{1500004228, 4193, ""},
	{1500005145, 5060, ""},
	{1500005265, 5171, "wiki-5171"}, // v61011 does not exist on VNDB
	{1500006271, 6116, ""},
	{1500007668, 2755, ""}, // 秘恋 FD: its link is catalog 2755's www.hook-net.jp/slfd
	{1500061345, 61145, ""},
	{1500061906, 206713, ""},
	{1500062165, 206724, ""},
	{1500207480, 60062, "v64606"},
	{1500210811, 3627, ""},
	{1500211739, 6884, ""},
	{1500215478, 1059, ""},
	{1500215574, 4165, ""},
	{1500216264, 641, ""},
	{1500217879, 10688, ""},
	{1500218105, 691, ""},
	{1500218586, 1032, ""},
	{1500219312, 2964, ""},
	{1500220146, 19974, ""},
	{1500220474, 15866, ""},
	{1500221779, 1157, ""},
	{1500221784, 2538, ""},
	{1500221827, 1344, ""},
	{1500222202, 959, ""},
	{1500223536, 212547, ""},
	{1500225330, 13144, ""},
	{1500225704, 5504, ""},
}

// Checked against the VNDB API: catalog holds no vndb anchor for either work,
// and both numbers name another game catalog already carries.
var wrongVndb = []struct {
	ID  int
	Was string
}{
	{3567, "v48068"}, // snow daze, catalog 228054
	{5115, "v19776"}, // Turnabout Pairs, catalog 227269
}

type outcome struct {
	settlement
	Action string
}

type report struct {
	Outcomes     []outcome
	VndbFixed    int64
	WikiRefresh  int64
	LeftInBand   int64
	StaleWiki    int64
	DanglingLink int64
}

func run(tx *gorm.DB, rep *report) error {
	for _, s := range settlements {
		action, err := settle(tx, s)
		if err != nil {
			return fmt.Errorf("settle %d -> %d: %w", s.Local, s.Target, err)
		}
		rep.Outcomes = append(rep.Outcomes, outcome{s, action})
	}
	for _, w := range wrongVndb {
		res := tx.Exec(`UPDATE patch SET vndb_id = 'wiki-' || id WHERE id = ? AND vndb_id = ?`, w.ID, w.Was)
		if res.Error != nil {
			return fmt.Errorf("fix vndb_id of %d: %w", w.ID, res.Error)
		}
		rep.VndbFixed += res.RowsAffected
	}
	return refreshWikiAnchors(tx, rep)
}

func settle(tx *gorm.DB, s settlement) (string, error) {
	var pages []int
	if err := tx.Raw(`SELECT id FROM patch WHERE id IN (?, ?)`, s.Local, s.Target).
		Scan(&pages).Error; err != nil {
		return "", err
	}
	localIsPage, targetIsPage := false, false
	for _, id := range pages {
		localIsPage = localIsPage || id == s.Local
		targetIsPage = targetIsPage || id == s.Target
	}
	if !localIsPage {
		return "already settled", nil
	}

	if err := tx.Exec(`UPDATE patch_comment_community_map SET galgame_id = ? WHERE galgame_id = ?`,
		s.Target, s.Local).Error; err != nil {
		return "", err
	}

	action := "renumber"
	if targetIsPage {
		action = "fold"
		// The fold's DELETE cascades patch_comment away, and that table is the
		// rollback copy of the wall infra re-anchors onto the survivor.
		if err := tx.Exec(`UPDATE patch_comment SET galgame_id = ? WHERE galgame_id = ?`,
			s.Target, s.Local).Error; err != nil {
			return "", err
		}
		if err := tx.Exec(`
			UPDATE patch t SET comment_count = t.comment_count + s.comment_count
			FROM patch s WHERE t.id = ? AND s.id = ?`, s.Target, s.Local).Error; err != nil {
			return "", err
		}
		if err := patchMerge.Fold(tx, s.Local, s.Target); err != nil {
			return "", err
		}
		if err := patchMerge.Recount(tx, s.Target); err != nil {
			return "", err
		}
	} else {
		if err := tx.Exec(`UPDATE patch SET id = ? WHERE id = ?`, s.Target, s.Local).Error; err != nil {
			return "", err
		}
		if s.Vndb != "" {
			if err := tx.Exec(`UPDATE patch SET vndb_id = ? WHERE id = ?`, s.Vndb, s.Target).Error; err != nil {
				return "", err
			}
		}
	}

	if err := tx.Exec(`UPDATE patch_redirect SET new_id = ? WHERE new_id = ?`,
		s.Target, s.Local).Error; err != nil {
		return "", err
	}
	return action, tx.Exec(`
		INSERT INTO patch_redirect(old_id, new_id, scope) VALUES (?, ?, 'merge')
		ON CONFLICT(scope, old_id) DO UPDATE SET new_id = EXCLUDED.new_id
	`, s.Local, s.Target).Error
}

// A wiki-N left on page M is not cosmetic. Publishing onto work N, when catalog
// has no vndb anchor for it, writes vndb_id 'wiki-N' and trips the UNIQUE
// index: 16 live works were one publish away from that on 2026-09-28.
func refreshWikiAnchors(tx *gorm.DB, rep *report) error {
	res := tx.Exec(`
		UPDATE patch SET vndb_id = 'wiki-' || id
		WHERE vndb_id ~ '^wiki-[0-9]+$' AND vndb_id <> 'wiki-' || id`)
	rep.WikiRefresh = res.RowsAffected
	return res.Error
}

func (rep *report) verify(tx *gorm.DB) error {
	checks := []struct {
		into *int64
		sql  string
		args []any
	}{
		{&rep.LeftInBand, `SELECT count(*) FROM patch WHERE id >= ?`, []any{patchModel.LocalOnlyIDBase}},
		{&rep.StaleWiki, `SELECT count(*) FROM patch WHERE vndb_id ~ '^wiki-[0-9]+$' AND vndb_id <> 'wiki-' || id`, nil},
		{&rep.DanglingLink, `SELECT count(*) FROM patch_redirect r
			WHERE NOT EXISTS (SELECT 1 FROM patch p WHERE p.id = r.new_id)`, nil},
	}
	for _, c := range checks {
		if err := tx.Raw(c.sql, c.args...).Scan(c.into).Error; err != nil {
			return err
		}
	}
	return nil
}

func (rep *report) print() {
	counts := map[string]int{}
	fmt.Println("\n本地段整改")
	for _, o := range rep.Outcomes {
		counts[o.Action]++
		fmt.Printf("  %-10d -> %-7d %s\n", o.Local, o.Target, o.Action)
	}
	fmt.Printf("\n  改号 %d / 并入 %d / 已处理过 %d\n",
		counts["renumber"], counts["fold"], counts["already settled"])
	fmt.Printf("  修正错误 vndb_id     %4d\n", rep.VndbFixed)
	fmt.Printf("  刷新过期 wiki-N      %4d\n", rep.WikiRefresh)
	fmt.Printf("\n核对（应全为 0）\n")
	fmt.Printf("  本地段剩余页         %4d\n", rep.LeftInBand)
	fmt.Printf("  过期 wiki-N          %4d\n", rep.StaleWiki)
	fmt.Printf("  指向不存在页的跳转   %4d\n", rep.DanglingLink)
}
