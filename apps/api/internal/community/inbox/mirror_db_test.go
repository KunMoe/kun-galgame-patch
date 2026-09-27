package inbox

import (
	"os"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Runs only against the launcher-provided TEST_DATABASE_DSN, migrated through
// 044, with `go test -count=1 -p 1`.
func mirrorDB(t *testing.T) *gorm.DB {
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
	if err := db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_name = 'user_message' AND column_name = 'community_seq')`).Scan(&migrated).Error; err != nil || !migrated {
		t.Fatalf("TEST_DATABASE_DSN is not migrated through 044: %v", err)
	}
	if err := db.Exec(`INSERT INTO "user" (id, updated) VALUES (990003, now()) ON CONFLICT DO NOTHING`).Error; err != nil {
		t.Fatal(err)
	}
	clean := func() { db.Exec(`DELETE FROM user_message WHERE recipient_id = 990003`) }
	clean()
	t.Cleanup(clean)
	return db
}

func mirrorRow(t *testing.T, db *gorm.DB, id int64) (found bool, status int, content string) {
	t.Helper()
	var rows []struct {
		Status  int
		Content string
	}
	if err := db.Raw(`SELECT status, content FROM user_message WHERE community_notification_id = ?`, id).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		return false, 0, ""
	}
	return true, rows[0].Status, rows[0].Content
}

func fold(id, seq int64, content string, updatedAt time.Time) *messageRow {
	return &messageRow{
		Type: "followActivity", Content: content, RecipientID: 990003,
		Created: updatedAt, Updated: time.Now(), NotificationID: id, Seq: seq,
	}
}

// Infra review (4): community's updated_at is the fold transaction's START
// time. A retraction can start first, wait for the dispatcher lock and commit
// after a fold update that started later, so it carries the older updated_at
// and the higher seq. Guarded by updated_at it was dropped as stale and the
// local row lived forever.
func TestARetractionThatStartedEarlierStillDeletesTheFold(t *testing.T) {
	db := mirrorDB(t)
	t0 := time.Date(2026, 9, 26, 4, 0, 0, 0, time.UTC)

	if err := upsertMessage(db, fold(700001, 10, "发布了 2 个 Galgame 补丁", t0.Add(time.Second))); err != nil {
		t.Fatal(err)
	}
	if err := deleteMessage(db, &messageRow{NotificationID: 700001, Seq: 11, Retract: true}); err != nil {
		t.Fatal(err)
	}
	if found, _, _ := mirrorRow(t, db, 700001); found {
		t.Fatal("the retraction with the higher seq did not delete the row")
	}
}

func TestAnOlderRetractionLeavesANewerFold(t *testing.T) {
	db := mirrorDB(t)
	t0 := time.Date(2026, 9, 26, 4, 0, 0, 0, time.UTC)

	if err := upsertMessage(db, fold(700002, 20, "发布了 3 个 Galgame 补丁", t0)); err != nil {
		t.Fatal(err)
	}
	if err := deleteMessage(db, &messageRow{NotificationID: 700002, Seq: 19, Retract: true}); err != nil {
		t.Fatal(err)
	}
	if found, _, _ := mirrorRow(t, db, 700002); !found {
		t.Fatal("a replayed older retraction deleted a newer fold")
	}
}

func TestFoldUpdatesApplyInSeqOrder(t *testing.T) {
	db := mirrorDB(t)
	t0 := time.Date(2026, 9, 26, 4, 0, 0, 0, time.UTC)

	if err := upsertMessage(db, fold(700003, 30, "new", t0)); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE user_message SET status = 1 WHERE community_notification_id = 700003`).Error; err != nil {
		t.Fatal(err)
	}

	// Older seq, newer updated_at: skipped.
	if err := upsertMessage(db, fold(700003, 29, "old", t0.Add(time.Minute))); err != nil {
		t.Fatal(err)
	}
	if _, status, content := mirrorRow(t, db, 700003); content != "new" || status != 1 {
		t.Fatalf("older seq applied: status=%d content=%q", status, content)
	}

	// Same seq replayed: keeps the local read.
	if err := upsertMessage(db, fold(700003, 30, "new", t0)); err != nil {
		t.Fatal(err)
	}
	if _, status, _ := mirrorRow(t, db, 700003); status != 1 {
		t.Fatalf("a replay un-read the row: status=%d", status)
	}

	// Newer seq: the fold grew and is unread again.
	if err := upsertMessage(db, fold(700003, 31, "newer", t0)); err != nil {
		t.Fatal(err)
	}
	if _, status, content := mirrorRow(t, db, 700003); content != "newer" || status != 0 {
		t.Fatalf("newer seq: status=%d content=%q", status, content)
	}
}
