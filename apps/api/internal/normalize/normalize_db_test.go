package normalize

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"kun-galgame-patch-api/internal/common/upload"
	"kun-galgame-patch-api/internal/repack"
	"kun-galgame-patch-api/pkg/artifactclient"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	testUser     = 1499999902
	testPatch    = 1499999902
	testResource = 1499999902
	srcUUID      = "00000000-0000-4000-8000-0000000000c1"
	newSrcUUID   = "00000000-0000-4000-8000-0000000000c2"
	productUUID  = "00000000-0000-4000-8000-0000000000d1"
	product2UUID = "00000000-0000-4000-8000-0000000000d2"
	repackUUID   = "00000000-0000-4000-8000-0000000000e1"
	missingUUID  = "00000000-0000-4000-8000-0000000000f1"
	productBytes = 3 * 1024 * 1024
)

// Runs only against the launcher-provided TEST_DATABASE_DSN, migrated through
// 050, with `go test -count=1 -p 1`.
func normalizeDB(t *testing.T) *gorm.DB {
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
	if err := db.Raw(`SELECT to_regclass('patch_resource_normalize') IS NOT NULL`).Scan(&migrated).Error; err != nil || !migrated {
		t.Fatalf("TEST_DATABASE_DSN is not migrated through 050: %v", err)
	}
	clean := func() {
		db.Exec(`DELETE FROM patch WHERE id = ?`, testPatch)
		db.Exec(`DELETE FROM "user" WHERE id = ?`, testUser)
		db.Exec(`DELETE FROM patch_resource_repack_purge WHERE resource_id = ?`, testResource)
		db.Exec(`DELETE FROM patch_resource_change WHERE resource_id = ?`, testResource)
	}
	clean()
	t.Cleanup(clean)
	for _, q := range []string{
		`INSERT INTO "user" (id, updated) VALUES (1499999902, now())`,
		`INSERT INTO patch (id, vndb_id, user_id, updated) VALUES (1499999902, 'v-normalize-test', 1499999902, now())`,
		`INSERT INTO patch_resource (id, storage, artifact_uuid, password, size, note, user_id, galgame_id, update_time, created, updated)
			VALUES (1499999902, 's3', '` + srcUUID + `', 'moyu.moe', '1 MB', 'readme', 1499999902, 1499999902,
				'2025-03-01T12:00:00Z', '2025-03-01T12:00:00Z', '2025-03-01T12:00:00Z')`,
		`DELETE FROM patch_resource_change WHERE resource_id = 1499999902`,
	} {
		if err := db.Exec(q).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

type owners struct{ allow bool }

func (o owners) RequireOwner(context.Context, string, int) error {
	if o.allow {
		return nil
	}
	return upload.ErrNotUploadOwner
}

// fakeArtifacts is the artifact service as the lane sees it: every uuid is a
// ready public file, products are 3 MiB zips, the repack still carries
// letmoe's name, and missingUUID does not exist.
type fakeArtifacts struct {
	mu         sync.Mutex
	names      map[string]string
	renames    []string
	failRename bool
}

func (f *fakeArtifacts) name(uuid string) string {
	if n, ok := f.names[uuid]; ok {
		return n
	}
	switch uuid {
	case srcUUID, newSrcUUID:
		return "汉化补丁 v1.2.rar"
	case repackUUID:
		return "Studio20200101Title (letmoe repack).zip"
	}
	return "汉化补丁 v1.2.zip"
}

func (f *fakeArtifacts) client(t *testing.T) *artifactclient.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		uuid, isDownload := strings.CutSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/artifacts/"), "/download")
		switch {
		case uuid == missingUUID:
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"code":50001,"message":"资源不存在"}`))
			return
		case isDownload:
			_, _ = w.Write([]byte(`{"code":0,"message":"成功","data":{"url":"https://dl.imoe.uk/moyu/` + uuid + `"}}`))
			return
		case r.Method == http.MethodPatch && f.failRename:
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"code":50013,"message":"制品存储失败"}`))
			return
		case r.Method == http.MethodPatch:
			var body struct{ Name string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			if f.names == nil {
				f.names = map[string]string{}
			}
			f.names[uuid] = body.Name
			f.renames = append(f.renames, uuid+" "+body.Name)
		}
		size := productBytes
		if uuid == srcUUID || uuid == newSrcUUID {
			size = 1048576
		}
		meta, _ := json.Marshal(map[string]any{"uuid": uuid, "name": f.name(uuid), "file_size": size, "public": true, "status": 1})
		_, _ = w.Write([]byte(`{"code":0,"message":"成功","data":` + string(meta) + `}`))
	}))
	t.Cleanup(srv.Close)
	return artifactclient.New(artifactclient.Config{BaseURL: srv.URL, ClientID: "moyu", ClientSecret: "s"})
}

func artifacts(t *testing.T) *artifactclient.Client { return (&fakeArtifacts{}).client(t) }

func complete(product, source string, hasArchives bool) CompleteRequest {
	return CompleteRequest{
		ArtifactUUID: product, SourceArtifactUUID: source, Filename: "汉化补丁 v1.2.zip", Bytes: productBytes,
		Blake3: strings.Repeat("ab", 32), FileCount: 4, HasArchives: hasArchives,
	}
}

func claimOurs(t *testing.T, svc *Service) []Job {
	t.Helper()
	jobs, err := svc.Claim(context.Background(), MaxClaim, nil)
	if err != nil {
		t.Fatal(err)
	}
	return slices.DeleteFunc(jobs, func(j Job) bool { return j.ResourceID != testResource })
}

type resourceRow struct {
	ArtifactUUID string
	Password     string
	Size         string
	Blake3       string
	Note         string
	UpdateTime   time.Time
	Updated      time.Time
}

func resource(t *testing.T, db *gorm.DB) resourceRow {
	t.Helper()
	var out resourceRow
	if err := db.Raw(`SELECT artifact_uuid, password, size, blake3, note, update_time, updated FROM patch_resource WHERE id = ?`,
		testResource).Scan(&out).Error; err != nil {
		t.Fatal(err)
	}
	return out
}

func strings_(t *testing.T, db *gorm.DB, q string) []string {
	t.Helper()
	var out []string
	if err := db.Raw(q, testResource).Scan(&out).Error; err != nil {
		t.Fatal(err)
	}
	return out
}

func purgeQueue(t *testing.T, db *gorm.DB) []string {
	return strings_(t, db, `SELECT artifact_uuid FROM patch_resource_repack_purge WHERE resource_id = ? ORDER BY artifact_uuid`)
}

func feed(t *testing.T, db *gorm.DB) []string {
	return strings_(t, db, `SELECT kind FROM patch_resource_change WHERE resource_id = ? ORDER BY seq`)
}

func TestSwapChangesOnlyTheFile(t *testing.T) {
	db := normalizeDB(t)
	art := &fakeArtifacts{}
	svc := New(db, art.client(t), owners{allow: true})
	ctx := context.Background()

	jobs := claimOurs(t, svc)
	if len(jobs) != 1 {
		t.Fatalf("claimed %d jobs, want 1", len(jobs))
	}
	job := jobs[0]
	if job.ArtifactUUID != srcUUID || job.GalgameID != testPatch || job.Filename != "汉化补丁 v1.2.rar" ||
		job.Password != "moyu.moe" || job.Bytes != 1048576 || job.Repack != nil || !job.LeaseExpiresAt.After(time.Now()) {
		t.Fatalf("job = %+v", job)
	}
	if again := claimOurs(t, svc); len(again) != 0 {
		t.Fatalf("a leased file was offered again: %+v", again)
	}

	before := resource(t, db)
	out, err := svc.Complete(ctx, testResource, testUser, complete(productUUID, srcUUID, false))
	if err != nil {
		t.Fatal(err)
	}
	if out.State != "done" || out.ArtifactUUID != productUUID || out.SourcePassword != "moyu.moe" || out.SourceSize != "1 MB" {
		t.Fatalf("job after complete = %+v", out)
	}
	after := resource(t, db)
	if after.ArtifactUUID != productUUID || after.Password != "" || after.Size != "3.000 MB" ||
		after.Blake3 != strings.Repeat("ab", 32) || after.Note != "readme" {
		t.Fatalf("resource after the swap = %+v", after)
	}
	if !after.UpdateTime.Equal(before.UpdateTime) || !after.Updated.Equal(before.Updated) {
		t.Fatalf("the swap moved the resource's times: %v/%v -> %v/%v",
			before.UpdateTime, before.Updated, after.UpdateTime, after.Updated)
	}
	if got := feed(t, db); !slices.Equal(got, []string{"normalized"}) {
		t.Fatalf("feed = %v, want [normalized]", got)
	}
	if got := purgeQueue(t, db); !slices.Equal(got, []string{srcUUID}) {
		t.Fatalf("purge queue = %v, want the original", got)
	}
	var held bool
	db.Raw(`SELECT next_attempt_at > now() + interval '29 days' FROM patch_resource_repack_purge WHERE artifact_uuid = ?`,
		srcUUID).Scan(&held)
	if !held {
		t.Fatal("the original is due for deletion within 29 days of the swap")
	}

	replay, err := svc.Complete(ctx, testResource, testUser, complete(productUUID, srcUUID, false))
	if err != nil || replay.ArtifactUUID != productUUID {
		t.Fatalf("replayed complete = %+v, %v", replay, err)
	}
	if _, err := svc.Complete(ctx, testResource, testUser, complete(product2UUID, srcUUID, false)); !errors.Is(err, ErrAlreadyDone) {
		t.Fatalf("a second product for the same source: %v, want ErrAlreadyDone", err)
	}
	if again := claimOurs(t, svc); len(again) != 0 {
		t.Fatalf("a forge product was offered: %+v", again)
	}
	if len(art.renames) != 0 {
		t.Fatalf("renamed a product the forge already named: %v", art.renames)
	}
	bad := complete(product2UUID, srcUUID, false)
	bad.Filename = "../汉化补丁.zip"
	if _, err := svc.Complete(ctx, testResource, testUser, bad); !errors.Is(err, ErrArtifactUnusable) {
		t.Fatalf("a filename with a path: %v, want ErrArtifactUnusable", err)
	}
}

func TestInnerArchivesKeepThePassword(t *testing.T) {
	db := normalizeDB(t)
	svc := New(db, artifacts(t), owners{allow: true})

	if _, err := svc.Complete(context.Background(), testResource, testUser, complete(productUUID, srcUUID, true)); err != nil {
		t.Fatal(err)
	}
	if got := resource(t, db); got.ArtifactUUID != productUUID || got.Password != "moyu.moe" {
		t.Fatalf("resource = %+v, want the product with the password kept", got)
	}
}

// Promoting the repack makes it the file: 047 drops the repack row, and 050
// keeps that drop from purging the file the swap just installed.
func TestPromotingTheRepack(t *testing.T) {
	db := normalizeDB(t)
	art := &fakeArtifacts{}
	svc := New(db, art.client(t), owners{allow: false})
	ctx := context.Background()

	attachRepack(t, db)
	jobs := claimOurs(t, svc)
	if len(jobs) != 1 || jobs[0].Repack == nil || jobs[0].Repack.ArtifactUUID != repackUUID {
		t.Fatalf("claimed %+v, want the resource with its repack", jobs)
	}
	var counted bool
	db.Raw(`SELECT claimed_at IS NOT NULL FROM patch_resource_normalize WHERE source_artifact_uuid = ?`, srcUUID).Scan(&counted)
	if counted {
		t.Fatal("a promotion claim counts toward the daily cap")
	}

	if _, err := svc.Complete(ctx, testResource, testUser, complete(repackUUID, srcUUID, false)); err != nil {
		t.Fatal(err)
	}
	if got := resource(t, db).ArtifactUUID; got != repackUUID {
		t.Fatalf("file = %s, want the repack", got)
	}
	if got := strings_(t, db, `SELECT artifact_uuid FROM patch_resource_repack WHERE resource_id = ?`); len(got) != 0 {
		t.Fatalf("repack row survived its promotion: %v", got)
	}
	if got := purgeQueue(t, db); !slices.Equal(got, []string{srcUUID}) {
		t.Fatalf("purge queue = %v, want only the original", got)
	}
	if got := feed(t, db); !slices.Equal(got, []string{"normalized"}) {
		t.Fatalf("feed = %v, want [normalized]", got)
	}
	if want := []string{repackUUID + " 汉化补丁 v1.2.zip"}; !slices.Equal(art.renames, want) {
		t.Fatalf("renames = %v, want %v", art.renames, want)
	}
}

// The rename comes after the swap, so a failed one is finished by the forge
// replaying complete.
func TestFailedRenameIsReplayed(t *testing.T) {
	db := normalizeDB(t)
	art := &fakeArtifacts{failRename: true}
	svc := New(db, art.client(t), owners{allow: true})
	ctx := context.Background()
	art.names = map[string]string{productUUID: "汉化补丁 v1.2 (forge).zip"}

	if _, err := svc.Complete(ctx, testResource, testUser, complete(productUUID, srcUUID, false)); err == nil {
		t.Fatal("complete reported success over a failed rename")
	}
	if got := resource(t, db).ArtifactUUID; got != productUUID {
		t.Fatalf("file = %s, want the swap kept", got)
	}
	art.failRename = false
	if _, err := svc.Complete(ctx, testResource, testUser, complete(productUUID, srcUUID, false)); err != nil {
		t.Fatal(err)
	}
	if got := art.name(productUUID); got != "汉化补丁 v1.2.zip" {
		t.Fatalf("name after the replay = %q", got)
	}
}

func TestReplacedSourceDiscardsTheProduct(t *testing.T) {
	db := normalizeDB(t)
	svc := New(db, artifacts(t), owners{allow: true})
	ctx := context.Background()

	if jobs := claimOurs(t, svc); len(jobs) != 1 {
		t.Fatalf("claimed %d jobs, want 1", len(jobs))
	}
	if err := db.Exec(`UPDATE patch_resource SET artifact_uuid = ? WHERE id = ?`, newSrcUUID, testResource).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Complete(ctx, testResource, testUser, complete(productUUID, srcUUID, false)); !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("complete against a replaced source: %v, want ErrSourceChanged", err)
	}
	if got := resource(t, db).ArtifactUUID; got != newSrcUUID {
		t.Fatalf("file = %s, want the author's replacement", got)
	}
	if got := purgeQueue(t, db); !slices.Equal(got, []string{productUUID}) {
		t.Fatalf("purge queue = %v, want the stale product", got)
	}
	jobs := claimOurs(t, svc)
	if len(jobs) != 1 || jobs[0].ArtifactUUID != newSrcUUID {
		t.Fatalf("after the replacement claimed %+v, want the new file", jobs)
	}
}

func TestFailIsTerminalForTheFile(t *testing.T) {
	db := normalizeDB(t)
	svc := New(db, artifacts(t), owners{allow: true})
	ctx := context.Background()

	if err := svc.Fail(ctx, testResource, FailRequest{SourceArtifactUUID: srcUUID, Code: "wrong_password"}); !errors.Is(err, ErrNoJob) {
		t.Fatalf("fail before a claim: %v, want ErrNoJob", err)
	}
	if jobs := claimOurs(t, svc); len(jobs) != 1 {
		t.Fatalf("claimed %d jobs, want 1", len(jobs))
	}
	if err := svc.Fail(ctx, testResource, FailRequest{SourceArtifactUUID: srcUUID, Code: "wrong_password", Message: "crc"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE patch_resource_normalize SET lease_until = now() - interval '1 second'
		WHERE resource_id = ?`, testResource).Error; err != nil {
		t.Fatal(err)
	}
	if jobs := claimOurs(t, svc); len(jobs) != 0 {
		t.Fatalf("a failed file was offered again: %+v", jobs)
	}
	if got := resource(t, db); got.ArtifactUUID != srcUUID || got.Password != "moyu.moe" {
		t.Fatalf("a failure touched the resource: %+v", got)
	}

	if err := db.Exec(`UPDATE patch_resource SET artifact_uuid = ? WHERE id = ?`, newSrcUUID, testResource).Error; err != nil {
		t.Fatal(err)
	}
	if jobs := claimOurs(t, svc); len(jobs) != 1 || jobs[0].ArtifactUUID != newSrcUUID {
		t.Fatalf("after the author replaced a failed file claimed %+v", jobs)
	}
}

func TestLeasesLapseAndRunOut(t *testing.T) {
	db := normalizeDB(t)
	svc := New(db, artifacts(t), owners{allow: true})
	lapse := func() {
		t.Helper()
		if err := db.Exec(`UPDATE patch_resource_normalize SET lease_until = now() - interval '1 second'
			WHERE resource_id = ?`, testResource).Error; err != nil {
			t.Fatal(err)
		}
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if jobs := claimOurs(t, svc); len(jobs) != 1 {
			t.Fatalf("attempt %d claimed %d jobs, want 1", attempt, len(jobs))
		}
		lapse()
	}
	if jobs := claimOurs(t, svc); len(jobs) != 0 {
		t.Fatalf("offered after %d lapsed leases: %+v", maxAttempts, jobs)
	}
	var code string
	db.Raw(`SELECT code FROM patch_resource_normalize WHERE source_artifact_uuid = ?`, srcUUID).Scan(&code)
	if code != "abandoned" {
		t.Fatalf("code = %q, want abandoned", code)
	}
}

func TestMissingSourceIsFailed(t *testing.T) {
	db := normalizeDB(t)
	svc := New(db, artifacts(t), owners{allow: true})
	if err := db.Exec(`UPDATE patch_resource SET artifact_uuid = ? WHERE id = ?`, missingUUID, testResource).Error; err != nil {
		t.Fatal(err)
	}
	if jobs := claimOurs(t, svc); len(jobs) != 0 {
		t.Fatalf("claimed a file the artifact service lacks: %+v", jobs)
	}
	var row struct{ State, Code string }
	db.Raw(`SELECT state, code FROM patch_resource_normalize WHERE source_artifact_uuid = ?`, missingUUID).Scan(&row)
	if row.State != "failed" || row.Code != "source_missing" {
		t.Fatalf("job = %+v, want failed source_missing", row)
	}
}

func attachRepack(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`INSERT INTO patch_resource_repack
			(resource_id, artifact_uuid, source_artifact_uuid, blake3, bytes, file_count, source_blake3)
		VALUES (?, ?, ?, ?, ?, 4, ?)`,
		testResource, repackUUID, srcUUID, strings.Repeat("ab", 32), productBytes, strings.Repeat("cd", 32)).Error; err != nil {
		t.Fatal(err)
	}
}

func TestTargetedClaim(t *testing.T) {
	db := normalizeDB(t)
	svc := New(db, artifacts(t), owners{allow: true})
	ctx := context.Background()

	if jobs, err := svc.Claim(ctx, MaxClaim, []int{testResource + 1}); err != nil || len(jobs) != 0 {
		t.Fatalf("claim narrowed to another resource = %+v, %v", jobs, err)
	}
	jobs, err := svc.Claim(ctx, MaxClaim, []int{testResource})
	if err != nil || len(jobs) != 1 || jobs[0].ResourceID != testResource {
		t.Fatalf("claim narrowed to ours = %+v, %v", jobs, err)
	}
}

func TestDailyCap(t *testing.T) {
	db := normalizeDB(t)
	svc := New(db, artifacts(t), owners{allow: true})
	if err := db.Exec(`INSERT INTO patch_resource_normalize (source_artifact_uuid, resource_id, state, claimed_at)
		SELECT 'cap-' || g, ?, 'failed', now() FROM generate_series(1, ?) g`, testResource, dailyClaimCap).Error; err != nil {
		t.Fatal(err)
	}
	if jobs := claimOurs(t, svc); len(jobs) != 0 {
		t.Fatalf("claimed past the daily cap: %+v", jobs)
	}
	attachRepack(t, db)
	if jobs := claimOurs(t, svc); len(jobs) != 1 || jobs[0].Repack == nil {
		t.Fatalf("a promotion was held back by the cap: %+v", jobs)
	}
	if err := db.Exec(`DELETE FROM patch_resource_repack WHERE resource_id = ?`, testResource).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE patch_resource_normalize SET claimed_at = now() - interval '2 days', lease_until = now()
		WHERE resource_id = ?`, testResource).Error; err != nil {
		t.Fatal(err)
	}
	if jobs := claimOurs(t, svc); len(jobs) != 1 || jobs[0].Repack != nil {
		t.Fatalf("claimed %+v once yesterday's claims stopped counting, want the upload", jobs)
	}
}

// The revert in docs/specs/2026-09-15-bot-submit-lane.md: the original becomes
// the file again while its delete is still queued, and must survive the drain.
func TestRevertKeepsTheOriginal(t *testing.T) {
	db := normalizeDB(t)
	art := &fakeArtifacts{}
	svc := New(db, art.client(t), owners{allow: true})
	ctx := context.Background()

	if _, err := svc.Complete(ctx, testResource, testUser, complete(productUUID, srcUUID, false)); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`UPDATE patch_resource r SET artifact_uuid = j.source_artifact_uuid, password = j.source_password,
			size = j.source_size, blake3 = '' FROM patch_resource_normalize j
			WHERE j.resource_id = r.id AND r.id = 1499999902`,
		`INSERT INTO patch_resource_repack_purge (artifact_uuid, resource_id) VALUES ('` + productUUID + `', 1499999902)`,
		`UPDATE patch_resource_normalize SET state = 'failed', code = 'reverted' WHERE resource_id = 1499999902`,
		`UPDATE patch_resource_repack_purge SET next_attempt_at = now() WHERE resource_id = 1499999902`,
	} {
		if err := db.Exec(q).Error; err != nil {
			t.Fatal(err)
		}
	}
	if got := resource(t, db); got.ArtifactUUID != srcUUID || got.Password != "moyu.moe" || got.Size != "1 MB" {
		t.Fatalf("resource after the revert = %+v", got)
	}

	deleted := &deletes{}
	if _, err := repack.New(db, deleted.client(t), nil).Purge(ctx); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(deleted.uuids, []string{productUUID}) {
		t.Fatalf("deleted %v, want only the product", deleted.uuids)
	}
	if got := purgeQueue(t, db); len(got) != 0 {
		t.Fatalf("purge queue after the drain = %v", got)
	}
	if jobs := claimOurs(t, svc); len(jobs) != 0 {
		t.Fatalf("a reverted file was offered again: %+v", jobs)
	}
}

type deletes struct{ uuids []string }

func (d *deletes) client(t *testing.T) *artifactclient.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		uuid := strings.TrimPrefix(r.URL.Path, "/api/v1/artifacts/")
		d.uuids = append(d.uuids, uuid)
		_, _ = w.Write([]byte(`{"code":0,"message":"成功","data":{"uuid":"` + uuid + `","deleted":true}}`))
	}))
	t.Cleanup(srv.Close)
	return artifactclient.New(artifactclient.Config{BaseURL: srv.URL, ClientID: "moyu", ClientSecret: "s"})
}

func TestDeletedResourceDiscardsTheProduct(t *testing.T) {
	db := normalizeDB(t)
	svc := New(db, artifacts(t), owners{allow: true})
	if err := db.Exec(`DELETE FROM patch_resource WHERE id = ?`, testResource).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`DELETE FROM patch_resource_repack_purge WHERE resource_id = ?`, testResource).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Complete(context.Background(), testResource, testUser, complete(productUUID, srcUUID, false)); !errors.Is(err, ErrResourceNotFound) {
		t.Fatalf("complete on a deleted resource: %v, want ErrResourceNotFound", err)
	}
	if got := purgeQueue(t, db); !slices.Equal(got, []string{productUUID}) {
		t.Fatalf("purge queue = %v, want the orphaned product", got)
	}
}

// forge runs several claimers at once. On 2026-10-15 4,902 claims went to
// 2,385 resources: a claimer that read the queue before another committed
// re-leased the same files.
func TestConcurrentClaimsNeverShareAFile(t *testing.T) {
	db := normalizeDB(t)
	svc := New(db, artifacts(t), owners{allow: true})
	const extra = 8
	for i := 1; i <= extra; i++ {
		if err := db.Exec(`INSERT INTO patch_resource (id, storage, artifact_uuid, user_id, galgame_id, updated)
			VALUES (?, 's3', ?, ?, ?, now())`,
			testResource+i, fmt.Sprintf("00000000-0000-4000-8000-0000000001%02d", i), testUser, testPatch).Error; err != nil {
			t.Fatal(err)
		}
	}

	for round := 0; round < 3; round++ {
		if err := db.Exec(`DELETE FROM patch_resource_normalize WHERE resource_id BETWEEN ? AND ?`,
			testResource, testResource+extra).Error; err != nil {
			t.Fatal(err)
		}
		var (
			mu   sync.Mutex
			got  []int
			wg   sync.WaitGroup
			errs []error
		)
		for range 3 * (extra + 1) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				jobs, err := svc.Claim(context.Background(), 1, nil)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					errs = append(errs, err)
				}
				for _, j := range jobs {
					got = append(got, j.ResourceID)
				}
			}()
		}
		wg.Wait()
		if len(errs) > 0 {
			t.Fatal(errs[0])
		}
		slices.Sort(got)
		if len(slices.Compact(slices.Clone(got))) != len(got) {
			t.Fatalf("round %d: one file went to several claimers: %v", round, got)
		}
		if len(got) != extra+1 {
			t.Fatalf("round %d: claimed %v, want each of the %d files once", round, got, extra+1)
		}
	}
}

// A deleted work's files keep their original: swapping one would spend the
// daily cap and tell letmoe a hidden resource changed.
func TestADeletedWorkIsNotNormalized(t *testing.T) {
	db := normalizeDB(t)
	svc := New(db, artifacts(t), owners{allow: true})
	if err := db.Exec(`UPDATE patch SET catalog_deleted_at = now() WHERE id = ?`, testPatch).Error; err != nil {
		t.Fatal(err)
	}
	if jobs, err := svc.Claim(context.Background(), MaxClaim, []int{testResource}); err != nil || len(jobs) != 0 {
		t.Fatalf("claimed %+v (%v) on a deleted work", jobs, err)
	}
}
