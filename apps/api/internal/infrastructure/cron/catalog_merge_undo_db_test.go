package cron

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"testing"

	patchMerge "kun-galgame-patch-api/internal/patch/merge"
	"kun-galgame-patch-api/pkg/catalogv2"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	undoUserA = 1499999951
	undoUserB = 1499999952
	undoOld   = 1499999953
	undoNew   = 1499999954
	undoChain = 1499999955
	undoLast  = 1499999959
)

// Runs only against the launcher-provided TEST_DATABASE_DSN, migrated through
// 052, with `go test -count=1 -p 1`.
func undoDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	var migrated bool
	if err := db.Raw(`SELECT to_regclass('patch_merge_record') IS NOT NULL`).Scan(&migrated).Error; err != nil || !migrated {
		t.Fatalf("TEST_DATABASE_DSN is not migrated through 052: %v", err)
	}
	clean := func() {
		db.Exec(`DELETE FROM patch_merge_record WHERE old_id BETWEEN ? AND ?`, undoOld, undoLast)
		db.Exec(`DELETE FROM patch_redirect WHERE old_id BETWEEN ? AND ? OR new_id BETWEEN ? AND ?`,
			undoOld, undoLast, undoOld, undoLast)
		db.Exec(`DELETE FROM patch WHERE id BETWEEN ? AND ?`, undoOld, undoLast)
		db.Exec(`DELETE FROM "user" WHERE id IN (?, ?)`, undoUserA, undoUserB)
	}
	clean()
	t.Cleanup(clean)
	mustExec(t, db, `INSERT INTO "user" (id, updated) VALUES (?, now()), (?, now())`, undoUserA, undoUserB)
	return db
}

func seedPage(t *testing.T, db *gorm.DB, id int, vndb string, view, status int) {
	t.Helper()
	mustExec(t, db, `INSERT INTO patch (id, vndb_id, user_id, view, status, updated) VALUES (?, ?, ?, ?, ?, now())`,
		id, vndb, undoUserA, view, status)
}

func seedResource(t *testing.T, db *gorm.DB, page, user int, name string) int {
	t.Helper()
	var id int
	if err := db.Raw(`INSERT INTO patch_resource (storage, user_id, galgame_id, name, updated)
		VALUES ('user', ?, ?, ?, now()) RETURNING id`, user, page, name).Scan(&id).Error; err != nil {
		t.Fatal(err)
	}
	return id
}

func pageOf(t *testing.T, db *gorm.DB, table string, id int) int {
	t.Helper()
	var page int
	db.Raw(fmt.Sprintf(`SELECT galgame_id FROM %s WHERE id = ?`, table), id).Scan(&page)
	return page
}

func merge(t *testing.T, db *gorm.DB, old, survivor int) {
	t.Helper()
	var r MergeSyncReport
	if err := applyRedirects(db, []catalogv2.Redirect{{OldID: int64(old), CurrentID: int64(survivor)}}, true, &r, nil); err != nil {
		t.Fatal(err)
	}
	if r.Applied() != 1 {
		t.Fatalf("merge %d -> %d applied %+v", old, survivor, r)
	}
}

func revoke(t *testing.T, db *gorm.DB, old int, vndbOf func(int) string) patchMerge.UndoReport {
	t.Helper()
	var r MergeSyncReport
	if err := applyRedirects(db, []catalogv2.Redirect{{OldID: int64(old), CurrentID: int64(old), Revoked: true}}, true, &r, vndbOf); err != nil {
		t.Fatal(err)
	}
	var raw string
	db.Raw(`SELECT undo_report::text FROM patch_merge_record WHERE old_id = ? AND undone_at IS NOT NULL ORDER BY id DESC LIMIT 1`, old).Scan(&raw)
	var rep patchMerge.UndoReport
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &rep); err != nil {
			t.Fatal(err)
		}
	}
	return rep
}

func TestUndoneRenumberGivesThePageBack(t *testing.T) {
	db := undoDB(t)
	seedPage(t, db, undoOld, "v990001", 7, 0)
	res := seedResource(t, db, undoOld, undoUserA, "汉化补丁")
	mustExec(t, db, `INSERT INTO user_patch_contribute_relation (user_id, galgame_id, updated) VALUES (?, ?, now())`, undoUserA, undoOld)
	mustExec(t, db, `INSERT INTO patch_comment (user_id, galgame_id, updated) VALUES (?, ?, now())`, undoUserA, undoOld)
	mustExec(t, db, `INSERT INTO patch_redirect (old_id, new_id, scope) VALUES (?, ?, 'legacy')`, undoChain, undoOld)

	merge(t, db, undoOld, undoNew)
	if pageOf(t, db, "patch_resource", res) != undoNew {
		t.Fatal("the renumber did not carry the resource")
	}
	late := seedResource(t, db, undoNew, undoUserB, "合并后发布")

	rep := revoke(t, db, undoOld, func(int) string { return "v990002" })

	if pageOf(t, db, "patch_resource", res) != undoOld {
		t.Error("the original resource did not go back to its own work")
	}
	if pageOf(t, db, "patch_resource", late) != undoNew {
		t.Error("a resource published on the survivor after the merge was taken away from it")
	}
	var survivor struct {
		VndbID string
		UserID int
		IsStub bool
	}
	db.Raw(`SELECT vndb_id, user_id, is_stub FROM patch WHERE id = ?`, undoNew).Scan(&survivor)
	if survivor.VndbID != "v990002" || survivor.UserID != undoUserB || survivor.IsStub {
		t.Errorf("survivor page = %+v, want its own vndb id, published by the late author", survivor)
	}
	var restored struct {
		VndbID        string
		View          int
		ResourceCount int
	}
	db.Raw(`SELECT vndb_id, view, resource_count FROM patch WHERE id = ?`, undoOld).Scan(&restored)
	if restored.VndbID != "v990001" || restored.View != 7 || restored.ResourceCount != 1 {
		t.Errorf("restored page = %+v", restored)
	}
	var legacy int
	db.Raw(`SELECT new_id FROM patch_redirect WHERE scope = 'legacy' AND old_id = ?`, undoChain).Scan(&legacy)
	if legacy != undoOld {
		t.Errorf("legacy /patch/%d points at %d, want the restored page again", undoChain, legacy)
	}
	var contributors int
	db.Raw(`SELECT count(*) FROM user_patch_contribute_relation WHERE galgame_id = ? AND user_id = ?`, undoNew, undoUserB).Scan(&contributors)
	if contributors != 1 {
		t.Error("the late author is not a contributor of the survivor page")
	}
	if rep.Incomplete() || rep.KeptOnSurvivor["patch_resource"] != 1 {
		t.Errorf("report = %+v", rep)
	}
}

func TestUndoneFoldSeparatesThePagesAgain(t *testing.T) {
	db := undoDB(t)
	seedPage(t, db, undoNew, "wiki-1499999954", 5, 0)
	seedPage(t, db, undoOld, "v990003", 10, 1)
	kept := seedResource(t, db, undoOld, undoUserA, "旧作补丁")
	gone := seedResource(t, db, undoOld, undoUserA, "之后被作者删除")
	own := seedResource(t, db, undoNew, undoUserB, "存续方自己的")
	for _, page := range []int{undoOld, undoNew} {
		mustExec(t, db, `INSERT INTO user_patch_contribute_relation (user_id, galgame_id, updated) VALUES (?, ?, now())`, undoUserA, page)
		mustExec(t, db, `INSERT INTO patch_link (galgame_id, name, url, updated) VALUES (?, 'official', 'https://x', now())`, page)
	}
	var dupContribution int
	db.Raw(`SELECT id FROM user_patch_contribute_relation WHERE galgame_id = ? AND user_id = ?`, undoOld, undoUserA).Scan(&dupContribution)
	var comment int
	db.Raw(`INSERT INTO patch_comment (user_id, galgame_id, updated) VALUES (?, ?, now()) RETURNING id`, undoUserA, undoOld).Scan(&comment)

	merge(t, db, undoOld, undoNew)
	var folded struct {
		VndbID string
		View   int
		Status int
	}
	db.Raw(`SELECT vndb_id, view, status FROM patch WHERE id = ?`, undoNew).Scan(&folded)
	if folded.VndbID != "v990003" || folded.View != 15 || folded.Status != 1 {
		t.Fatalf("folded survivor = %+v", folded)
	}
	if pageOf(t, db, "patch_comment", comment) != undoNew {
		t.Fatal("the fold let the frozen comment go with the deleted page")
	}
	mustExec(t, db, `UPDATE patch SET view = view + 3 WHERE id = ?`, undoNew)
	mustExec(t, db, `DELETE FROM patch_resource WHERE id = ?`, gone)
	late := seedResource(t, db, undoNew, undoUserB, "合并后发布")

	rep := revoke(t, db, undoOld, nil)

	for res, want := range map[int]int{kept: undoOld, own: undoNew, late: undoNew} {
		if got := pageOf(t, db, "patch_resource", res); got != want {
			t.Errorf("resource %d on %d, want %d", res, got, want)
		}
	}
	if pageOf(t, db, "patch_comment", comment) != undoOld {
		t.Error("the frozen comment did not go back")
	}
	if pageOf(t, db, "user_patch_contribute_relation", dupContribution) != undoOld {
		t.Error("the contribution the fold dropped as a duplicate was not re-inserted")
	}
	var links int
	db.Raw(`SELECT count(*) FROM patch_link WHERE galgame_id IN (?, ?)`, undoOld, undoNew).Scan(&links)
	if links != 2 {
		t.Errorf("links = %d, want each page's own back", links)
	}
	var pages []struct {
		ID     int
		VndbID string
		View   int
		Status int
	}
	db.Raw(`SELECT id, vndb_id, view, status FROM patch WHERE id IN (?, ?) ORDER BY id`, undoOld, undoNew).Scan(&pages)
	if len(pages) != 2 ||
		pages[0].VndbID != "v990003" || pages[0].View != 10 || pages[0].Status != 1 ||
		pages[1].VndbID != "wiki-1499999954" || pages[1].View != 8 || pages[1].Status != 0 {
		t.Errorf("pages = %+v", pages)
	}
	if !slices.Contains(rep.DeletedSince["patch_resource"], int64(gone)) {
		t.Errorf("report = %+v, want the resource its author deleted listed and left deleted", rep)
	}
	if rep.Incomplete() {
		t.Errorf("report = %+v", rep)
	}
}

func TestMergeBeforeRecordsOnlyLosesItsRedirect(t *testing.T) {
	db := undoDB(t)
	seedPage(t, db, undoNew, "v990004", 0, 0)
	mustExec(t, db, `INSERT INTO patch_redirect (old_id, new_id, scope) VALUES (?, ?, 'merge')`, undoOld, undoNew)

	rep := revoke(t, db, undoOld, nil)
	var left int64
	db.Raw(`SELECT count(*) FROM patch_redirect WHERE scope = 'merge' AND old_id = ?`, undoOld).Scan(&left)
	if left != 0 || rep.RecordID != 0 {
		t.Fatalf("redirects left = %d, report = %+v", left, rep)
	}
}

// The survivor was merged again before the first merge was undone: rows go
// back by primary key from wherever they sit now.
func TestUndoFindsRowsAfterTheSurvivorMovedOn(t *testing.T) {
	db := undoDB(t)
	seedPage(t, db, undoOld, "v990005", 4, 0)
	seedPage(t, db, undoNew, "v990006", 2, 0)
	mine := seedResource(t, db, undoOld, undoUserA, "旧作补丁")
	theirs := seedResource(t, db, undoNew, undoUserB, "存续方补丁")

	merge(t, db, undoOld, undoNew)
	merge(t, db, undoNew, undoLast)
	if pageOf(t, db, "patch_resource", mine) != undoLast {
		t.Fatal("setup: the second merge did not carry the resource")
	}

	rep := revoke(t, db, undoOld, nil)

	if pageOf(t, db, "patch_resource", mine) != undoOld {
		t.Error("the resource did not come back from the page it ended up on")
	}
	if pageOf(t, db, "patch_resource", theirs) != undoLast {
		t.Error("the survivor's own resource moved")
	}
	var vndb string
	db.Raw(`SELECT vndb_id FROM patch WHERE id = ?`, undoOld).Scan(&vndb)
	if vndb != "v990005" {
		t.Errorf("restored page vndb = %q", vndb)
	}
	if !rep.Incomplete() || len(rep.Notes) != 1 {
		t.Errorf("report = %+v, want a note that the survivor's counters stay", rep)
	}
}

// Review A: B→C then C→D, catalog undoes B→C first and C→D second. The second
// undo's record also lists B's resource (it sat on C when C was merged), but
// B already took it back: it must stay on B, and D must be recounted.
func TestChainedUndosLeaveEachRowWithItsOwnWork(t *testing.T) {
	db := undoDB(t)
	b, c, d := undoOld, undoNew, undoLast
	seedPage(t, db, b, "v990011", 0, 0)
	seedPage(t, db, c, "v990012", 0, 0)
	seedPage(t, db, d, "v990013", 0, 0)
	rb := seedResource(t, db, b, undoUserA, "B 的补丁")
	rc := seedResource(t, db, c, undoUserA, "C 的补丁")
	rd := seedResource(t, db, d, undoUserB, "D 的补丁")

	merge(t, db, b, c)
	merge(t, db, c, d)
	revoke(t, db, b, nil)
	rep := revoke(t, db, c, nil)

	for res, want := range map[int]int{rb: b, rc: c, rd: d} {
		if got := pageOf(t, db, "patch_resource", res); got != want {
			t.Errorf("resource %d on %d, want %d", res, got, want)
		}
	}
	var counts []int
	db.Raw(`SELECT resource_count FROM patch WHERE id IN (?, ?, ?) ORDER BY id`, b, c, d).Scan(&counts)
	if !slices.Equal(counts, []int{1, 1, 1}) {
		t.Errorf("resource_count of B, C, D = %v, want each page counting its own", counts)
	}
	if !slices.Contains(rep.MovedElsewhere["patch_resource"], int64(rb)) {
		t.Errorf("report = %+v, want B's resource listed as no longer on the survivor", rep)
	}
}

// Review C: catalog hid B (the claim event unpublished it), then merged it.
// Undoing the merge gives B its resources back, which is not an unban.
func TestUndoDoesNotUnbanAHiddenPage(t *testing.T) {
	db := undoDB(t)
	seedPage(t, db, undoNew, "v990014", 0, 0)
	seedPage(t, db, undoOld, "v990015", 0, 0)
	seedResource(t, db, undoOld, undoUserA, "被隐藏页的补丁")
	mustExec(t, db, `UPDATE patch SET published = false WHERE id = ?`, undoOld)
	mustExec(t, db, `INSERT INTO claim_event_processed (event_id, work_id, to_state, actor_uid) VALUES (?, ?, 'hidden', 0)`,
		int64(undoOld), undoOld)
	t.Cleanup(func() { db.Exec(`DELETE FROM claim_event_processed WHERE work_id = ?`, undoOld) })

	merge(t, db, undoOld, undoNew)
	revoke(t, db, undoOld, nil)

	var published bool
	db.Raw(`SELECT published FROM patch WHERE id = ?`, undoOld).Scan(&published)
	if published {
		t.Error("the undo published a page catalog had hidden")
	}
}

// Review B: the page's publisher and a dropped contributor were purged after
// the merge. Re-inserting them used to violate the user foreign key, roll the
// drain back and block every later merge on the same row forever.
func TestUndoSurvivesAPurgedUser(t *testing.T) {
	db := undoDB(t)
	const purged = 1499999956
	mustExec(t, db, `INSERT INTO "user" (id, updated) VALUES (?, now())`, purged)
	t.Cleanup(func() { db.Exec(`DELETE FROM "user" WHERE id = ?`, purged) })
	seedPage(t, db, undoNew, "v990016", 0, 0)
	mustExec(t, db, `INSERT INTO patch (id, vndb_id, user_id, updated) VALUES (?, 'v990017', ?, now())`, undoOld, purged)
	res := seedResource(t, db, undoOld, undoUserA, "补丁")
	for _, page := range []int{undoOld, undoNew} {
		mustExec(t, db, `INSERT INTO user_patch_contribute_relation (user_id, galgame_id, updated) VALUES (?, ?, now())`, purged, page)
	}

	merge(t, db, undoOld, undoNew)
	mustExec(t, db, `DELETE FROM user_patch_contribute_relation WHERE user_id = ?`, purged)
	mustExec(t, db, `DELETE FROM "user" WHERE id = ?`, purged)
	seedPage(t, db, undoChain, "v990018", 0, 0)
	mustExec(t, db, `INSERT INTO patch_redirect (old_id, new_id, scope) VALUES (?, ?, 'merge')`, undoLast, undoChain)

	var r MergeSyncReport
	items := []catalogv2.Redirect{
		{OldID: undoOld, CurrentID: undoOld, Revoked: true},
		{OldID: undoLast, CurrentID: undoLast, Revoked: true},
	}
	if err := applyRedirects(db, items, true, &r, nil); err != nil {
		t.Fatalf("the drain stopped on a purged user: %v", err)
	}
	if pageOf(t, db, "patch_resource", res) != undoOld {
		t.Error("the resource did not go back")
	}
	var owner int
	db.Raw(`SELECT user_id FROM patch WHERE id = ?`, undoOld).Scan(&owner)
	if owner != undoUserA {
		t.Errorf("restored page owner = %d, want the publisher of its resources", owner)
	}
	var left int64
	db.Raw(`SELECT count(*) FROM patch_redirect WHERE scope = 'merge' AND old_id = ?`, undoLast).Scan(&left)
	if left != 0 {
		t.Error("the revoke after the purged one was never reached")
	}
}

// Review B: an undo that fails for any other reason is recorded on the merge
// and the drain carries on; the admin page offers it for a retry.
func TestAFailedUndoIsRecordedAndTheDrainMovesOn(t *testing.T) {
	db := undoDB(t)
	seedPage(t, db, undoNew, "v990019", 0, 0)
	seedPage(t, db, undoOld, "v990020", 0, 0)
	merge(t, db, undoOld, undoNew)
	mustExec(t, db, `UPDATE patch_merge_record_row SET value = jsonb_set(value, '{view}', '"many"')
		WHERE table_name = 'patch' AND row_id = ? AND change = 'before'`, undoOld)
	seedPage(t, db, undoChain, "v990021", 0, 0)
	mustExec(t, db, `INSERT INTO patch_redirect (old_id, new_id, scope) VALUES (?, ?, 'merge')`, undoLast, undoChain)

	var r MergeSyncReport
	items := []catalogv2.Redirect{
		{OldID: undoOld, CurrentID: undoOld, Revoked: true},
		{OldID: undoLast, CurrentID: undoLast, Revoked: true},
	}
	if err := applyRedirects(db, items, true, &r, nil); err != nil {
		t.Fatalf("one failed undo stopped the drain: %v", err)
	}
	var rec struct {
		Open  bool
		Error string
	}
	db.Raw(`SELECT undone_at IS NULL AS open, COALESCE(undo_report->>'error', '') AS error
		FROM patch_merge_record WHERE old_id = ? ORDER BY id DESC LIMIT 1`, undoOld).Scan(&rec)
	if !rec.Open || rec.Error == "" {
		t.Errorf("record = %+v, want it still open with the error kept for a retry", rec)
	}
	var left int64
	db.Raw(`SELECT count(*) FROM patch_redirect WHERE scope = 'merge' AND old_id IN (?, ?)`, undoOld, undoLast).Scan(&left)
	if left != 0 {
		t.Errorf("%d revoked redirects left; the failure must not keep /galgame/<old> pointing at the survivor", left)
	}
}

// Review 2, item 1: C's undo failed, which still dropped the C→D redirect. B's
// undo must follow the failed record to D, where B's resource sits, instead
// of reading it as moved elsewhere.
func TestUndoFollowsAFailedUndoAlongTheChain(t *testing.T) {
	db := undoDB(t)
	b, c, d := undoOld, undoNew, undoLast
	seedPage(t, db, b, "v990031", 0, 0)
	seedPage(t, db, c, "v990032", 0, 0)
	seedPage(t, db, d, "v990033", 0, 0)
	rb := seedResource(t, db, b, undoUserA, "B 的补丁")

	merge(t, db, b, c)
	merge(t, db, c, d)
	mustExec(t, db, `UPDATE patch_merge_record_row SET value = jsonb_set(value, '{view}', '"many"')
		WHERE table_name = 'patch' AND row_id = ? AND change = 'before'
		  AND record_id = (SELECT id FROM patch_merge_record WHERE old_id = ? ORDER BY id DESC LIMIT 1)`, c, c)
	revoke(t, db, c, nil)
	rep := revoke(t, db, b, nil)

	if got := pageOf(t, db, "patch_resource", rb); got != b {
		t.Errorf("B's resource on %d, want back on B", got)
	}
	if len(rep.MovedElsewhere) != 0 {
		t.Errorf("report = %+v, want nothing read as moved elsewhere", rep)
	}
}

// Review 2, item 3: a page was created on the retired id before the undo ran
// (catalog revived the work and someone published to it). The page row the
// merge renumbered stays on the survivor's number, and a person has to know.
func TestUndoSaysWhenTheRetiredIdAlreadyHasAPage(t *testing.T) {
	db := undoDB(t)
	seedPage(t, db, undoOld, "v990034", 0, 0)
	seedResource(t, db, undoOld, undoUserA, "补丁")
	merge(t, db, undoOld, undoNew)
	seedPage(t, db, undoOld, "wiki-1499999953", 0, 0)

	rep := revoke(t, db, undoOld, nil)
	if len(rep.Notes) == 0 || !rep.Incomplete() {
		t.Errorf("report = %+v, want a note that the retired id already had a page", rep)
	}
}

// Review 2, item 4: the heir is the publisher of the earliest resource, by
// created, not by row order in the snapshot.
func TestHeirIsTheEarliestPublisher(t *testing.T) {
	db := undoDB(t)
	const purged = 1499999957
	mustExec(t, db, `INSERT INTO "user" (id, updated) VALUES (?, now())`, purged)
	t.Cleanup(func() { db.Exec(`DELETE FROM "user" WHERE id = ?`, purged) })
	seedPage(t, db, undoNew, "v990035", 0, 0)
	mustExec(t, db, `INSERT INTO patch (id, vndb_id, user_id, updated) VALUES (?, 'v990036', ?, now())`, undoOld, purged)
	mustExec(t, db, `INSERT INTO patch_resource (storage, user_id, galgame_id, created, updated)
		VALUES ('user', ?, ?, '2026-05-01', now())`, undoUserB, undoOld)
	mustExec(t, db, `INSERT INTO patch_resource (storage, user_id, galgame_id, created, updated)
		VALUES ('user', ?, ?, '2026-01-01', now())`, undoUserA, undoOld)

	merge(t, db, undoOld, undoNew)
	mustExec(t, db, `UPDATE patch SET user_id = ? WHERE user_id = ?`, undoUserA, purged)
	mustExec(t, db, `DELETE FROM "user" WHERE id = ?`, purged)
	revoke(t, db, undoOld, nil)

	var owner int
	db.Raw(`SELECT user_id FROM patch WHERE id = ?`, undoOld).Scan(&owner)
	if owner != undoUserA {
		t.Errorf("restored page owner = %d, want %d who published first", owner, undoUserA)
	}
}

// Contract §8: when A is merged into B, catalog re-points every redirect into
// A at B and refreshes it, and points them back at A when that merge is undone.
// Re-points reach this site as redirects of their own and leave records of
// their own; undoing X's merge has to undo every open record of X, not only
// the newest, which by then is a re-point that moved nothing.
func TestUndoOfAnIdWhoseRedirectWasRepointed(t *testing.T) {
	db := undoDB(t)
	x, a, b := undoOld, undoNew, undoLast
	seedPage(t, db, x, "v990041", 0, 0)
	seedPage(t, db, a, "v990042", 0, 0)
	seedPage(t, db, b, "v990043", 0, 0)
	rx := seedResource(t, db, x, undoUserA, "X 的补丁")

	merge(t, db, x, a)
	var r MergeSyncReport
	if err := applyRedirects(db, []catalogv2.Redirect{
		{OldID: int64(x), CurrentID: int64(b)},
		{OldID: int64(a), CurrentID: int64(b)},
	}, true, &r, nil); err != nil {
		t.Fatal(err)
	}
	if err := applyRedirects(db, []catalogv2.Redirect{
		{OldID: int64(a), CurrentID: int64(a), Revoked: true},
		{OldID: int64(x), CurrentID: int64(a)},
	}, true, &r, nil); err != nil {
		t.Fatal(err)
	}
	revoke(t, db, x, nil)

	if got := pageOf(t, db, "patch_resource", rx); got != x {
		t.Errorf("X's resource on %d, want back on X", got)
	}
	var open int64
	db.Raw(`SELECT count(*) FROM patch_merge_record WHERE old_id = ? AND undone_at IS NULL`, x).Scan(&open)
	if open != 0 {
		t.Errorf("%d records of X still open after its merge was undone", open)
	}
}
