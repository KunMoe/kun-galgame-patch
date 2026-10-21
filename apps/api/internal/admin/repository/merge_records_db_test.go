package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	patchMerge "kun-galgame-patch-api/internal/patch/merge"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Runs only against the launcher-provided TEST_DATABASE_DSN, migrated through
// 052, with `go test -count=1 -p 1`.
func TestMergeRecordsListTheUndosThatLeftSomethingBehind(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	clean := func() { db.Exec(`DELETE FROM patch_merge_record WHERE old_id BETWEEN 1499999961 AND 1499999963`) }
	clean()
	t.Cleanup(clean)
	for _, q := range []string{
		`INSERT INTO patch_merge_record (old_id, new_id, action) VALUES (1499999961, 1, 'fold')`,
		`INSERT INTO patch_merge_record (old_id, new_id, action, undone_at, undo_report)
			VALUES (1499999962, 1, 'fold', now(), '{"record_id":0,"action":"fold","moved_back":{"patch_resource":2}}')`,
		`INSERT INTO patch_merge_record (old_id, new_id, action, undone_at, undo_report)
			VALUES (1499999963, 1, 'renumber', now(), '{"record_id":0,"action":"renumber","conflicts":{"patch_link":[5]}}')`,
	} {
		if err := db.Exec(q).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := New(db)

	all, total, err := repo.ListMergeRecords(0, 100, false)
	if err != nil || total < 3 {
		t.Fatalf("all = %d records, err %v", total, err)
	}
	issues, n, err := repo.ListMergeRecords(0, 100, true)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range issues {
		if r.OldID == 1499999962 || r.OldID == 1499999961 {
			t.Errorf("record %d listed as an issue", r.OldID)
		}
		if r.OldID == 1499999963 {
			found = true
			var rep struct {
				Conflicts map[string][]int64 `json:"conflicts"`
			}
			if err := json.Unmarshal(r.UndoReport, &rep); err != nil || len(rep.Conflicts["patch_link"]) != 1 {
				t.Errorf("report = %s (%v)", r.UndoReport, err)
			}
		}
	}
	if !found || n < 1 {
		t.Errorf("issues = %+v, want the undo with a conflict", issues)
	}
	for _, r := range all {
		if r.OldID == 1499999961 && r.UndoReport != nil {
			t.Errorf("an applied merge carries an undo report: %s", r.UndoReport)
		}
	}
}

// A deleted work's page is missing from catalog's answer like a true orphan,
// and deleting it from the orphan list would make a restore impossible.
func TestOrphanListLeavesDeletedWorksAlone(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	clean := func() {
		db.Exec(`DELETE FROM patch WHERE id IN (1499999971, 1499999972)`)
		db.Exec(`DELETE FROM "user" WHERE id = 1499999971`)
	}
	clean()
	t.Cleanup(clean)
	for _, q := range []string{
		`INSERT INTO "user" (id, updated) VALUES (1499999971, now())`,
		`INSERT INTO patch (id, vndb_id, user_id, updated) VALUES (1499999971, 'wiki-1499999971', 1499999971, now())`,
		`INSERT INTO patch (id, vndb_id, user_id, updated, catalog_deleted_at) VALUES (1499999972, 'wiki-1499999972', 1499999971, now(), now())`,
	} {
		if err := db.Exec(q).Error; err != nil {
			t.Fatal(err)
		}
	}
	ids, err := New(db).GetOrphanCandidateIDs()
	if err != nil {
		t.Fatal(err)
	}
	has := map[int]bool{}
	for _, id := range ids {
		has[id] = true
	}
	if !has[1499999971] || has[1499999972] {
		t.Errorf("orphan candidates include live placeholder: %v, deleted work: %v", has[1499999971], has[1499999972])
	}
}

func mergeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	clean := func() {
		db.Exec(`DELETE FROM patch_merge_record WHERE old_id BETWEEN 1499999973 AND 1499999976`)
		db.Exec(`DELETE FROM patch WHERE id BETWEEN 1499999973 AND 1499999976`)
		db.Exec(`DELETE FROM "user" WHERE id IN (1499999973, 1499999974)`)
	}
	clean()
	t.Cleanup(clean)
	for _, q := range []string{
		`INSERT INTO "user" (id, updated) VALUES (1499999973, now()), (1499999974, now())`,
		`INSERT INTO patch (id, vndb_id, user_id, view, updated) VALUES (1499999973, 'v-mr-1', 1499999973, 10, now()), (1499999974, 'v-mr-2', 1499999974, 100, now())`,
		`INSERT INTO patch_resource (storage, user_id, galgame_id, updated) VALUES ('user', 1499999973, 1499999973, now())`,
	} {
		if err := db.Exec(q).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		id, err := patchMerge.Begin(tx, patchMerge.ActionFold, 1499999973, 1499999974)
		if err != nil {
			return err
		}
		if err := patchMerge.Fold(tx, 1499999973, 1499999974); err != nil {
			return err
		}
		return patchMerge.Finish(tx, id, 1499999974)
	}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestOnlyAFailedUndoCanBeRetried(t *testing.T) {
	db := mergeTestDB(t)
	repo := New(db)
	var recordID int64
	db.Raw(`SELECT id FROM patch_merge_record WHERE old_id = 1499999973`).Scan(&recordID)

	if _, err := repo.RetryMergeUndo(recordID, nil); err != ErrNoFailedUndo {
		t.Fatalf("retry of a merge catalog never undid: %v, want it refused", err)
	}
	if err := patchMerge.MarkFailed(db, recordID, fmt.Errorf("boom")); err != nil {
		t.Fatal(err)
	}
	report, err := repo.RetryMergeUndo(recordID, nil)
	if err != nil || report == nil || report.MovedBack["patch_resource"] != 1 {
		t.Fatalf("retry = (%+v, %v)", report, err)
	}
	var open bool
	db.Raw(`SELECT undone_at IS NULL FROM patch_merge_record WHERE id = ?`, recordID).Scan(&open)
	if open {
		t.Error("a retried undo left the record open")
	}
}

func TestPurgeTakesTheUsersRowsOutOfMergeRecords(t *testing.T) {
	db := mergeTestDB(t)
	if err := db.Transaction(func(tx *gorm.DB) error {
		return purgeMergeSnapshots(tx, 1499999973, false, 1499999974)
	}); err != nil {
		t.Fatal(err)
	}
	var resources, pagesOfPurged, handed int64
	db.Raw(`SELECT count(*) FROM patch_merge_record_row r JOIN patch_merge_record m ON m.id = r.record_id
		WHERE m.old_id = 1499999973 AND r.table_name = 'patch_resource'`).Scan(&resources)
	db.Raw(`SELECT count(*) FROM patch_merge_record_row WHERE table_name = 'patch' AND value->>'user_id' = '1499999973'`).Scan(&pagesOfPurged)
	db.Raw(`SELECT count(*) FROM patch_merge_record_row r JOIN patch_merge_record m ON m.id = r.record_id
		WHERE m.old_id = 1499999973 AND r.table_name = 'patch' AND r.row_id = 1499999973 AND r.value->>'user_id' = '1499999974'`).Scan(&handed)
	if resources != 0 || pagesOfPurged != 0 || handed != 1 {
		t.Errorf("resource copies %d, pages naming the purged user %d, handed to the heir %d", resources, pagesOfPurged, handed)
	}
}

// Review 2, item 2: a double click ran two retries at once, and the second
// took the retired page's counters off the survivor a second time.
func TestConcurrentRetriesUndoOnce(t *testing.T) {
	db := mergeTestDB(t)
	repo := New(db)
	var recordID int64
	db.Raw(`SELECT id FROM patch_merge_record WHERE old_id = 1499999973`).Scan(&recordID)
	if err := patchMerge.MarkFailed(db, recordID, fmt.Errorf("boom")); err != nil {
		t.Fatal(err)
	}

	first := db.Begin()
	rec, err := patchMerge.FailedRecord(first, recordID)
	if err != nil || rec == nil {
		t.Fatalf("record: %+v, %v", rec, err)
	}
	if _, err := patchMerge.Undo(first, rec, ""); err != nil {
		first.Rollback()
		t.Fatal(err)
	}
	second := make(chan error, 1)
	go func() {
		_, err := repo.RetryMergeUndo(recordID, nil)
		second <- err
	}()
	time.Sleep(300 * time.Millisecond)
	if err := first.Commit().Error; err != nil {
		t.Fatal(err)
	}
	if err := <-second; err != ErrNoFailedUndo {
		t.Errorf("second retry = %v, want it refused once the first had undone the merge", err)
	}
	var view int
	db.Raw(`SELECT view FROM patch WHERE id = 1499999974`).Scan(&view)
	if view != 100 {
		t.Errorf("survivor view = %d, want 100: the retired page's 10 taken off once", view)
	}
}
