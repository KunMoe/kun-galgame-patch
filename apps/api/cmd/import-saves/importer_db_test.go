package main

import (
	"context"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	testUser     = 1499999950
	newWork      = 1499999951
	livedWork    = 1499999952
	stubWork     = 1499999953
	deletedWork  = 1499999954
	missingWork  = 1499999955
	otherUser    = 1499999956
	otherCreated = "2026-10-20T08:00:00Z"
)

type fakeCatalog struct{ known []int }

func (f fakeCatalog) GalgameBatch(_ context.Context, ids []int, _ string) ([]galgameClient.GalgameBrief, error) {
	var out []galgameClient.GalgameBrief
	for _, id := range ids {
		if slices.Contains(f.known, id) {
			out = append(out, galgameClient.GalgameBrief{ID: id})
		}
	}
	return out, nil
}

func (fakeCatalog) GetGalgame(context.Context, int, string) (*galgameClient.GalgameDetailEnvelope, error) {
	return nil, nil
}

type fakeFiles struct{ n int }

func (f *fakeFiles) UploadFile(_ context.Context, _, _ string, size int64, _ int) (string, int64, error) {
	f.n++
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", 990000+f.n), size, nil
}

func (*fakeFiles) Delete(context.Context, string) error { return nil }

func savesDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	works := []int{newWork, livedWork, stubWork, deletedWork, missingWork}
	clean := func() {
		db.Exec(`DELETE FROM activity_push_queue WHERE key LIKE 'patch_resource:%' AND split_part(key, ':', 2)::int IN
			(SELECT id FROM patch_resource WHERE galgame_id IN ?)`, works)
		db.Exec(`DELETE FROM patch WHERE id IN ?`, works)
		db.Exec(`DELETE FROM "user" WHERE id IN ?`, []int{testUser, otherUser})
	}
	clean()
	t.Cleanup(clean)
	for _, q := range []string{
		`INSERT INTO "user" (id, updated) VALUES (1499999956, now())`,
		`INSERT INTO patch (id, vndb_id, user_id, published, resource_count, type, created, updated, resource_update_time)
			VALUES (1499999952, 'v-saves-lived', 1499999956, true, 1, '["manual"]', '` + otherCreated + `', '` + otherCreated + `', '` + otherCreated + `')`,
		`INSERT INTO patch_resource (storage, artifact_uuid, size, user_id, galgame_id, type, update_time, created, updated)
			VALUES ('s3', '00000000-0000-4000-8000-000000980001', '1 MB', 1499999956, 1499999952, '["manual"]', now(), now(), now())`,
		`INSERT INTO patch (id, vndb_id, user_id, is_stub, created, updated) VALUES (1499999953, 'v-saves-stub', 1499999956, true, now(), now())`,
		`INSERT INTO patch (id, vndb_id, user_id, catalog_deleted_at, created, updated) VALUES (1499999954, 'v-saves-deleted', 1499999956, now(), now(), now())`,
	} {
		if err := db.Exec(q).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func record(asset int64, work int, sha string) saveRecord {
	return saveRecord{AssetID: asset, WorkID: work, Filename: "s.zip", Bytes: 2048, Label: "フルコンプセーブ",
		SourceURL: "https://seiya-saiga.com/game/x.html", GameTitle: "テスト", OriginalSHA256: sha}
}

type pageRow struct {
	UserID             int
	Published          bool
	IsStub             bool
	ResourceCount      int
	ContributeCount    int
	Type               string
	Created            time.Time
	Updated            time.Time
	ResourceUpdateTime time.Time
}

func page(t *testing.T, db *gorm.DB, id int) pageRow {
	t.Helper()
	var p pageRow
	if err := db.Raw(`SELECT user_id, published, is_stub, resource_count, contribute_count, type::text AS type,
		created, updated, resource_update_time FROM patch WHERE id = ?`, id).Scan(&p).Error; err != nil {
		t.Fatal(err)
	}
	return p
}

func TestImportDatesEverythingItCreatesAndNotifiesNobody(t *testing.T) {
	db := savesDB(t)
	files := &fakeFiles{}
	imp := &importer{db: db, catalog: fakeCatalog{known: []int{newWork, livedWork, stubWork, deletedWork}}, files: files, userID: testUser}
	ctx := context.Background()

	run := func(work int, recs ...saveRecord) []result {
		t.Helper()
		out := imp.importWork(ctx, work, recs)
		for _, r := range out {
			if r.status == statusFailed {
				t.Fatalf("work %d asset %d: %s", work, r.assetID, r.msg)
			}
		}
		return out
	}
	run(newWork, record(1, newWork, "aaa"), record(2, newWork, "bbb"))
	run(livedWork, record(3, livedWork, "ccc"))
	run(stubWork, record(4, stubWork, "ddd"))
	if got := run(deletedWork, record(5, deletedWork, "eee")); got[0].status != statusWorkDeleted {
		t.Errorf("a catalog-deleted work takes no save, got %s", got[0].status)
	}
	if got := run(missingWork, record(6, missingWork, "fff")); got[0].status != statusWorkMissing {
		t.Errorf("a work the catalog does not serve gets no page, got %s", got[0].status)
	}

	created := page(t, db, newWork)
	if !created.Created.Equal(importedAt) || !created.Updated.Equal(importedAt) || !created.ResourceUpdateTime.Equal(importedAt) {
		t.Errorf("a new page is dated %s, got %+v", importedAt, created)
	}
	if created.UserID != testUser || !created.Published || created.ResourceCount != 2 || created.ContributeCount != 1 || created.Type != `["save"]` {
		t.Errorf("new page: %+v", created)
	}

	lived := page(t, db, livedWork)
	want, _ := time.Parse(time.RFC3339, otherCreated)
	if !lived.Created.Equal(want) || !lived.Updated.Equal(want) || !lived.ResourceUpdateTime.Equal(want) {
		t.Errorf("a page that already had resources keeps its dates, got %+v", lived)
	}
	if lived.UserID != otherUser || lived.ResourceCount != 2 || lived.ContributeCount != 1 || lived.Type != `["manual", "save"]` {
		t.Errorf("lived page: %+v", lived)
	}

	stub := page(t, db, stubWork)
	if stub.IsStub || !stub.Published || stub.UserID != testUser || !stub.Created.Equal(importedAt) || !stub.Updated.Equal(importedAt) {
		t.Errorf("an adopted stub is published and dated like a new page, got %+v", stub)
	}

	var res []struct {
		Created    time.Time
		Updated    time.Time
		UpdateTime time.Time
		Backfill   *bool
	}
	if err := db.Raw(`SELECT r.created, r.updated, r.update_time, q.backfill FROM patch_resource r
		LEFT JOIN activity_push_queue q ON q.key = 'patch_resource:' || r.id
		WHERE r.user_id = ? ORDER BY r.id`, testUser).Scan(&res).Error; err != nil {
		t.Fatal(err)
	}
	if len(res) != 4 {
		t.Fatalf("want 4 imported saves, got %d", len(res))
	}
	for i, r := range res {
		if !r.Created.Equal(importedAt) || !r.Updated.Equal(importedAt) || !r.UpdateTime.Equal(importedAt) {
			t.Errorf("save %d is not dated %s: %+v", i, importedAt, r)
		}
		if r.Backfill == nil || !*r.Backfill {
			t.Errorf("save %d is queued as a live push, which may notify: %+v", i, r)
		}
	}

	if got := run(newWork, record(1, newWork, "aaa")); got[0].status != statusSkipped {
		t.Errorf("a rerun skips a save already imported, got %s", got[0].status)
	}
	if files.n != 4 {
		t.Errorf("want 4 uploads, got %d", files.n)
	}
}
