package repository

import (
	"encoding/json"
	"os"
	"testing"

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
