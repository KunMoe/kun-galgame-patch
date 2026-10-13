package rating

import (
	"context"
	"os"
	"testing"
	"time"

	"kun-galgame-patch-api/pkg/catalogv2"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Runs only against the launcher-provided TEST_DATABASE_DSN, migrated through
// 046, with `go test -count=1 -p 1`.
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
	if err := db.Raw(`SELECT to_regclass('galgame_rating') IS NOT NULL`).Scan(&migrated).Error; err != nil || !migrated {
		t.Fatalf("TEST_DATABASE_DSN is not migrated through 046: %v", err)
	}
	clean := func() {
		db.Exec(`DELETE FROM galgame_rating WHERE work_id BETWEEN 990000 AND 990099`)
		db.Exec(`DELETE FROM cron_state WHERE name = ?`, mirrorCronName)
	}
	clean()
	t.Cleanup(clean)
	return db
}

func centralRating(id int64, userID int, workID int64, summary string) *catalogv2.UserRating {
	eight, all := 8, "all"
	return &catalogv2.UserRating{
		ID: id, WorkID: workID, UserID: userID, Overall: 9,
		Aspects:   catalogv2.RatingAspects{Art: &eight},
		Recommend: "yes", SpoilerLevel: "none", ShortSummary: summary,
		ClientID: "other-site-client", Site: "kungal", State: "done", Completion: &all,
		CreatedAt: time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC),
	}
}

// Every drain replays the partial page after its cursor and the write-through
// replays the author's own save: an unchanged rating must write nothing.
func TestMirrorRewritesOnlyAChangedRating(t *testing.T) {
	store := NewStore(mirrorDB(t))
	r := centralRating(990001, 7, 990001, "好")

	if changed, err := store.Mirror(r); err != nil || !changed {
		t.Fatalf("first mirror: changed %v, err %v", changed, err)
	}
	if changed, err := store.Mirror(r); err != nil || changed {
		t.Fatalf("replay of the same rating: changed %v, err %v; want nothing written", changed, err)
	}

	r.State, r.Completion = "on_hold", nil
	if changed, err := store.Mirror(r); err != nil || !changed {
		t.Fatalf("author state moved: changed %v, err %v", changed, err)
	}
	got, err := store.Get(990001)
	if err != nil {
		t.Fatal(err)
	}
	if got.AuthorState == nil || *got.AuthorState != "on_hold" || got.AuthorCompletion != nil || *got.Art != 8 || got.Story != nil {
		t.Errorf("row = %+v", got)
	}
}

// A rating replaced upstream gets a new id; its gone entry can arrive after the
// new rating, and the unique (user, work) must not refuse the new one.
func TestMirrorReplacesAnotherIDOnTheSameUserAndWork(t *testing.T) {
	store := NewStore(mirrorDB(t))
	if _, err := store.Mirror(centralRating(990002, 7, 990002, "old")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Mirror(centralRating(990003, 7, 990002, "new")); err != nil {
		t.Fatalf("replacement refused: %v", err)
	}
	if _, err := store.Get(990002); err != ErrNotFound {
		t.Errorf("replaced rating still mirrored: %v", err)
	}
	rows, total, err := store.ListByWork(990002, 0, 10)
	if err != nil || total != 1 || rows[0].ID != 990003 {
		t.Fatalf("list = %+v (%d), err %v", rows, total, err)
	}
	if dropped, err := store.Drop(990002); err != nil || dropped {
		t.Errorf("late gone of the replaced id: dropped %v, err %v", dropped, err)
	}
}

type pagedFeed struct {
	pages   map[string]catalogv2.RatingChangePage
	cursors []string
}

func (f *pagedFeed) Configured() bool { return true }

func (f *pagedFeed) RatingChanges(_ context.Context, cursor string) (catalogv2.RatingChangePage, error) {
	f.cursors = append(f.cursors, cursor)
	return f.pages[cursor], nil
}

func TestSyncKeepsTheLastFullPageCursor(t *testing.T) {
	db := mirrorDB(t)
	feed := &pagedFeed{pages: map[string]catalogv2.RatingChangePage{
		"": {Items: []catalogv2.RatingChange{
			{ID: 990010, Rating: centralRating(990010, 7, 990010, "a")},
			{ID: 990011, Rating: centralRating(990011, 8, 990010, "b")},
		}, NextCursor: "cur_full"},
		"cur_full": {Items: []catalogv2.RatingChange{
			{ID: 990011, Gone: true},
		}},
	}}
	m := NewMirror(db, feed, NewStore(db))

	applied, err := m.Sync(context.Background())
	if err != nil || applied != 3 {
		t.Fatalf("first sync: applied %d, err %v; want 2 written and 1 dropped", applied, err)
	}
	applied, err = m.Sync(context.Background())
	if err != nil || applied != 0 {
		t.Fatalf("second sync: applied %d, err %v; want the short page re-read with nothing to change", applied, err)
	}
	if want := []string{"", "cur_full", "cur_full"}; len(feed.cursors) != 3 || feed.cursors[2] != want[2] {
		t.Errorf("cursors read = %q, want %q", feed.cursors, want)
	}
	if _, total, _ := NewStore(db).ListByWork(990010, 0, 10); total != 1 {
		t.Errorf("work 990010 has %d ratings, want 1", total)
	}
}
