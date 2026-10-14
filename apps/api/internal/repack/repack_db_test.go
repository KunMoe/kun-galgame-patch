package repack

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"kun-galgame-patch-api/pkg/artifactclient"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	testUser     = 1499999901
	testPatch    = 1499999901
	testResource = 1499999901
	srcUUID      = "00000000-0000-4000-8000-0000000000a1"
	newSrcUUID   = "00000000-0000-4000-8000-0000000000a2"
	repackUUID   = "00000000-0000-4000-8000-0000000000b1"
	repackUUID2  = "00000000-0000-4000-8000-0000000000b2"
	failingUUID  = "00000000-0000-4000-8000-0000000000f1"
)

// Runs only against the launcher-provided TEST_DATABASE_DSN, migrated through
// 047, with `go test -count=1 -p 1`.
func repackDB(t *testing.T) *gorm.DB {
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
	if err := db.Raw(`SELECT to_regclass('patch_resource_repack_purge') IS NOT NULL`).Scan(&migrated).Error; err != nil || !migrated {
		t.Fatalf("TEST_DATABASE_DSN is not migrated through 047: %v", err)
	}
	clean := func() {
		db.Exec(`DELETE FROM patch WHERE id = ?`, testPatch)
		db.Exec(`DELETE FROM "user" WHERE id = ?`, testUser)
		db.Exec(`DELETE FROM patch_resource_repack_purge WHERE resource_id = ?`, testResource)
	}
	clean()
	t.Cleanup(clean)
	for _, q := range []string{
		`INSERT INTO "user" (id, updated) VALUES (1499999901, now())`,
		`INSERT INTO patch (id, vndb_id, user_id, updated) VALUES (1499999901, 'v-repack-test', 1499999901, now())`,
		`INSERT INTO patch_resource (id, storage, artifact_uuid, user_id, galgame_id, updated)
			VALUES (1499999901, 's3', '` + srcUUID + `', 1499999901, 1499999901, now())`,
	} {
		if err := db.Exec(q).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

type anyOwner struct{}

func (anyOwner) RequireOwner(context.Context, string, int) error { return nil }

// artifacts is the artifact service as far as a repack touches it: every uuid
// is a ready public 42-byte file, and DELETE fails for failingUUID.
type artifacts struct {
	mu      sync.Mutex
	deleted []string
}

func (a *artifacts) client(t *testing.T) *artifactclient.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		uuid := strings.TrimPrefix(r.URL.Path, "/api/v1/artifacts/")
		if r.Method == http.MethodDelete {
			if uuid == failingUUID {
				w.WriteHeader(500)
				_, _ = w.Write([]byte(`{"code":50013,"message":"制品存储失败"}`))
				return
			}
			a.mu.Lock()
			a.deleted = append(a.deleted, uuid)
			a.mu.Unlock()
			_, _ = w.Write([]byte(`{"code":0,"message":"成功","data":{"uuid":"` + uuid + `","deleted":true}}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"message":"成功","data":{"uuid":"` + uuid + `","name":"r.zip","file_size":42,"public":true,"status":1}}`))
	}))
	t.Cleanup(srv.Close)
	return artifactclient.New(artifactclient.Config{BaseURL: srv.URL, ClientID: "moyu", ClientSecret: "s"})
}

func request(artifact, source string) AttachRequest {
	return AttachRequest{
		ArtifactUUID: artifact, SourceArtifactUUID: source,
		Blake3: strings.Repeat("ab", 32), Bytes: 42, FileCount: 3, SourceBlake3: strings.Repeat("cd", 32),
	}
}

func queued(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var out []string
	if err := db.Raw(`SELECT artifact_uuid FROM patch_resource_repack_purge WHERE resource_id = ? ORDER BY artifact_uuid`,
		testResource).Scan(&out).Error; err != nil {
		t.Fatal(err)
	}
	return out
}

func attached(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var out string
	db.Raw(`SELECT artifact_uuid FROM patch_resource_repack WHERE resource_id = ?`, testResource).Scan(&out)
	return out
}

func TestRepackLifecycle(t *testing.T) {
	db := repackDB(t)
	art := &artifacts{}
	svc := New(db, art.client(t), anyOwner{})
	ctx := context.Background()

	if _, err := svc.Attach(ctx, testResource, testUser, request(repackUUID, newSrcUUID)); !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("attach against a stale source: %v, want ErrSourceChanged", err)
	}
	first, err := svc.Attach(ctx, testResource, testUser, request(repackUUID, srcUUID))
	if err != nil {
		t.Fatal(err)
	}
	again, err := svc.Attach(ctx, testResource, testUser, request(repackUUID, srcUUID))
	if err != nil || !again.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("replayed attach: %v, created_at %v -> %v; want a no-op", err, first.CreatedAt, again.CreatedAt)
	}

	if err := db.Exec(`UPDATE patch_resource SET name = 'renamed', artifact_uuid = artifact_uuid WHERE id = ?`,
		testResource).Error; err != nil {
		t.Fatal(err)
	}
	if got := attached(t, db); got != repackUUID {
		t.Fatalf("an edit that kept the file dropped the repack (now %q)", got)
	}

	if _, err := svc.Attach(ctx, testResource, testUser, request(repackUUID2, srcUUID)); err != nil {
		t.Fatal(err)
	}
	if got := queued(t, db); len(got) != 1 || got[0] != repackUUID {
		t.Fatalf("superseded repack queued = %v, want [%s]", got, repackUUID)
	}
	if _, err := svc.Attach(ctx, testResource, testUser, request(repackUUID, srcUUID)); !errors.Is(err, ErrArtifactUnusable) {
		t.Fatalf("re-attaching an artifact queued for deletion: %v, want ErrArtifactUnusable", err)
	}

	if err := db.Exec(`UPDATE patch_resource SET artifact_uuid = ? WHERE id = ?`, newSrcUUID, testResource).Error; err != nil {
		t.Fatal(err)
	}
	if got := attached(t, db); got != "" {
		t.Fatalf("replacing the file left repack %q attached", got)
	}
	if got := queued(t, db); len(got) != 2 {
		t.Fatalf("queued after file replace = %v, want both repacks", got)
	}

	if n, err := svc.Purge(ctx); err != nil || n != 2 {
		t.Fatalf("purge = %d, %v", n, err)
	}
	if got := queued(t, db); len(got) != 0 || len(art.deleted) != 2 {
		t.Fatalf("after purge: queued %v, deleted %v", got, art.deleted)
	}
}

// A page delete reaches the resource by FK cascade, which no Go write path
// sees; the repack's artifact must be queued all the same.
func TestCascadeQueuesTheRepack(t *testing.T) {
	db := repackDB(t)
	svc := New(db, (&artifacts{}).client(t), anyOwner{})
	ctx := context.Background()

	if _, err := svc.Attach(ctx, testResource, testUser, request(failingUUID, srcUUID)); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`DELETE FROM patch WHERE id = ?`, testPatch).Error; err != nil {
		t.Fatal(err)
	}
	if got := queued(t, db); len(got) != 1 || got[0] != failingUUID {
		t.Fatalf("queued after page delete = %v", got)
	}

	if n, err := svc.Purge(ctx); err == nil || n != 0 {
		t.Fatalf("purge of a failing delete = %d, %v; want the failure reported", n, err)
	}
	var row struct {
		Attempts int
		Later    bool
	}
	db.Raw(`SELECT attempts, next_attempt_at > now() AS later FROM patch_resource_repack_purge WHERE artifact_uuid = ?`,
		failingUUID).Scan(&row)
	if row.Attempts != 1 || !row.Later {
		t.Fatalf("failed purge row = %+v, want one attempt and a later retry", row)
	}
	if n, err := svc.Purge(ctx); err != nil || n != 0 {
		t.Fatalf("purge before the retry is due = %d, %v; want nothing tried", n, err)
	}
}

func TestWithdraw(t *testing.T) {
	db := repackDB(t)
	svc := New(db, (&artifacts{}).client(t), anyOwner{})
	ctx := context.Background()

	if _, err := svc.Attach(ctx, testResource, testUser, request(repackUUID, srcUUID)); err != nil {
		t.Fatal(err)
	}
	if deleted, err := svc.Withdraw(ctx, testResource); err != nil || !deleted {
		t.Fatalf("withdraw = %v, %v", deleted, err)
	}
	if deleted, err := svc.Withdraw(ctx, testResource); err != nil || deleted {
		t.Fatalf("second withdraw = %v, %v; want nothing left to delete", deleted, err)
	}
	if got := queued(t, db); len(got) != 1 || got[0] != repackUUID {
		t.Fatalf("queued after withdraw = %v", got)
	}

	if err := db.Exec(`UPDATE patch_resource SET status = 2 WHERE id = ?`, testResource).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Attach(ctx, testResource, testUser, request(repackUUID2, srcUUID)); !errors.Is(err, ErrResourceNotFound) {
		t.Fatalf("attach to a hidden resource: %v, want ErrResourceNotFound", err)
	}
}
