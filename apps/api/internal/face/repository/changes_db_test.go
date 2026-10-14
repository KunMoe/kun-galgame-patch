package repository

import (
	"os"
	"slices"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	feedUser  = 1499999801
	feedPatch = 1499999801
	feedOther = 1499999802
	feedRes   = 1499999801
	feedRes2  = 1499999802
)

// Runs only against the launcher-provided TEST_DATABASE_DSN, migrated through
// 048, with `go test -count=1 -p 1`.
func feedDB(t *testing.T) *gorm.DB {
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
	if err := db.Raw(`SELECT to_regclass('patch_resource_change') IS NOT NULL`).Scan(&migrated).Error; err != nil || !migrated {
		t.Fatalf("TEST_DATABASE_DSN is not migrated through 048: %v", err)
	}
	clean := func() {
		db.Exec(`DELETE FROM patch WHERE id IN (?, ?)`, feedPatch, feedOther)
		db.Exec(`DELETE FROM "user" WHERE id = ?`, feedUser)
		db.Exec(`DELETE FROM patch_resource_change WHERE resource_id IN (?, ?)`, feedRes, feedRes2)
	}
	clean()
	t.Cleanup(clean)
	mustExec(t, db, `INSERT INTO "user" (id, updated) VALUES (?, now())`, feedUser)
	mustExec(t, db, `INSERT INTO patch (id, vndb_id, user_id, updated) VALUES
		(?, 'v-feed-1', ?, now()), (?, 'v-feed-2', ?, now())`, feedPatch, feedUser, feedOther, feedUser)
	return db
}

func mustExec(t *testing.T, db *gorm.DB, q string, args ...any) {
	t.Helper()
	if err := db.Exec(q, args...).Error; err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

// drain reads the whole feed from the start and keeps the test's own rows.
func drain(t *testing.T, r *Repository) []ChangeRow {
	t.Helper()
	var out []ChangeRow
	var txid, seq int64
	for {
		rows, err := r.ListChanges(txid, seq, 100)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if row.ResourceID == feedRes || row.ResourceID == feedRes2 {
				out = append(out, row)
			}
		}
		if len(rows) < 100 {
			return out
		}
		txid, seq = rows[len(rows)-1].Txid, rows[len(rows)-1].Seq
	}
}

func kinds(rows []ChangeRow) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Kind)
	}
	return out
}

func TestChangeFeedKinds(t *testing.T) {
	db := feedDB(t)
	r := New(db)

	mustExec(t, db, `INSERT INTO patch_resource (id, storage, artifact_uuid, user_id, galgame_id, updated)
		VALUES (?, 's3', 'a1', ?, ?, now())`, feedRes, feedUser, feedPatch)
	mustExec(t, db, `UPDATE patch_resource SET download = download + 1, like_count = like_count + 1 WHERE id = ?`, feedRes)
	mustExec(t, db, `UPDATE patch_resource SET name = name, artifact_uuid = artifact_uuid WHERE id = ?`, feedRes)
	mustExec(t, db, `UPDATE patch_resource SET type = '["ai"]' WHERE id = ?`, feedRes)
	mustExec(t, db, `UPDATE patch_resource SET artifact_uuid = 'a2' WHERE id = ?`, feedRes)
	mustExec(t, db, `UPDATE patch_resource SET galgame_id = ? WHERE id = ?`, feedOther, feedRes)
	mustExec(t, db, `UPDATE patch_resource SET status = 2 WHERE id = ?`, feedRes)
	mustExec(t, db, `UPDATE patch_resource SET name = 'edited while hidden', status = 1 WHERE id = ?`, feedRes)
	mustExec(t, db, `UPDATE patch_resource SET status = 0 WHERE id = ?`, feedRes)
	mustExec(t, db, `DELETE FROM patch WHERE id = ?`, feedOther)

	got := drain(t, r)
	want := []string{"created", "updated", "file_replaced", "updated", "hidden", "restored", "deleted"}
	if !slices.Equal(kinds(got), want) {
		t.Fatalf("kinds = %v, want %v", kinds(got), want)
	}
	if got[3].PatchID != feedOther || got[6].PatchID != feedOther {
		t.Errorf("a moved resource reports page %d then %d, want %d", got[3].PatchID, got[6].PatchID, feedOther)
	}
}

// A transaction that drew the lower seq but commits later must hold the feed
// back, not be stepped past.
func TestChangeFeedWaitsForAnOlderTransaction(t *testing.T) {
	db := feedDB(t)
	r := New(db)

	slow := db.Begin()
	t.Cleanup(func() { slow.Rollback() })
	mustExec(t, slow, `INSERT INTO patch_resource (id, storage, user_id, galgame_id, updated)
		VALUES (?, 's3', ?, ?, now())`, feedRes, feedUser, feedPatch)
	mustExec(t, db, `INSERT INTO patch_resource (id, storage, user_id, galgame_id, updated)
		VALUES (?, 's3', ?, ?, now())`, feedRes2, feedUser, feedPatch)

	if got := drain(t, r); len(got) != 0 {
		t.Fatalf("served %v while an older transaction was still open; its row would have been skipped", got)
	}
	if err := slow.Commit().Error; err != nil {
		t.Fatal(err)
	}
	got := drain(t, r)
	if len(got) != 2 || got[0].ResourceID != feedRes || got[1].ResourceID != feedRes2 {
		t.Fatalf("after commit = %+v, want the older transaction's row first, then the newer", got)
	}
}
