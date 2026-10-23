package normalize

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/internal/patch/repository"
	patchService "kun-galgame-patch-api/internal/patch/service"

	"gorm.io/gorm"
)

const (
	oldSrcUUID   = "00000000-0000-4000-8000-0000000000b1"
	olderSrcUUID = "00000000-0000-4000-8000-0000000000b2"
	comment      = "使用说明\n解压密码：终点"
)

func ptr(s string) *string { return &s }

func archiveComment(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var out string
	if err := db.Raw(`SELECT archive_comment FROM patch_resource WHERE id = ?`, testResource).Scan(&out).Error; err != nil {
		t.Fatal(err)
	}
	return out
}

type jobRow struct {
	SourcePassword string
	SourceComment  *string
}

func jobOf(t *testing.T, db *gorm.DB, source string) jobRow {
	t.Helper()
	var out jobRow
	if err := db.Raw(`SELECT source_password, source_comment FROM patch_resource_normalize WHERE source_artifact_uuid = ?`,
		source).Scan(&out).Error; err != nil {
		t.Fatal(err)
	}
	return out
}

func TestClaimOffersEarlierPasswordsAndTheNote(t *testing.T) {
	db := normalizeDB(t)
	svc := New(db, artifacts(t), owners{allow: true})
	for _, q := range []string{
		`INSERT INTO patch_resource_normalize (source_artifact_uuid, resource_id, state, source_password, created_at)
			VALUES ('` + olderSrcUUID + `', 1499999902, 'failed', 'older', now() - interval '2 days')`,
		`INSERT INTO patch_resource_normalize (source_artifact_uuid, resource_id, state, source_password, created_at)
			VALUES ('` + oldSrcUUID + `', 1499999902, 'failed', 'old', now() - interval '1 day')`,
		`INSERT INTO patch_resource_normalize (source_artifact_uuid, resource_id, state, source_password, created_at)
			VALUES ('00000000-0000-4000-8000-0000000000b3', 1499999902, 'failed', 'moyu.moe', now() - interval '3 days')`,
		`INSERT INTO patch_resource_normalize (source_artifact_uuid, resource_id, state, source_password, created_at)
			VALUES ('00000000-0000-4000-8000-0000000000b4', 1499999902, 'failed', 'old', now() - interval '4 days')`,
	} {
		if err := db.Exec(q).Error; err != nil {
			t.Fatal(err)
		}
	}

	jobs := claimOurs(t, svc)
	if len(jobs) != 1 {
		t.Fatalf("claimed %d jobs, want 1", len(jobs))
	}
	if got := jobs[0].PasswordCandidates; !slices.Equal(got, []string{"old", "older"}) {
		t.Fatalf("password_candidates = %q, want [old older]", got)
	}
	if jobs[0].Note != "readme" {
		t.Fatalf("note = %q", jobs[0].Note)
	}
}

func TestClaimWithoutCandidatesSendsAnEmptyList(t *testing.T) {
	db := normalizeDB(t)
	jobs := claimOurs(t, New(db, artifacts(t), owners{allow: true}))
	if len(jobs) != 1 || jobs[0].PasswordCandidates == nil || len(jobs[0].PasswordCandidates) != 0 {
		t.Fatalf("jobs = %+v", jobs)
	}
}

func TestCompleteRecordsTheOpeningPasswordAndComment(t *testing.T) {
	for _, hasArchives := range []bool{false, true} {
		db := normalizeDB(t)
		svc := New(db, artifacts(t), owners{allow: true})
		db.Exec(`UPDATE patch_resource SET password = '' WHERE id = ?`, testResource)
		claimOurs(t, svc)

		req := complete(productUUID, srcUUID, hasArchives)
		req.Password = "coroz"
		req.ArchiveComment = ptr(comment)
		if _, err := svc.Complete(context.Background(), testResource, testUser, req); err != nil {
			t.Fatal(err)
		}
		want := ""
		if hasArchives {
			want = "coroz"
		}
		if got := resource(t, db); got.Password != want {
			t.Fatalf("has_archives=%v: password = %q, want %q", hasArchives, got.Password, want)
		}
		if got := archiveComment(t, db); got != comment {
			t.Fatalf("archive_comment = %q", got)
		}
		if got := jobOf(t, db, srcUUID); got.SourcePassword != "coroz" || got.SourceComment == nil || *got.SourceComment != comment {
			t.Fatalf("job = %+v", got)
		}
	}
}

func TestCompleteWithoutAPasswordKeepsTheResources(t *testing.T) {
	db := normalizeDB(t)
	svc := New(db, artifacts(t), owners{allow: true})
	claimOurs(t, svc)
	if _, err := svc.Complete(context.Background(), testResource, testUser, complete(productUUID, srcUUID, true)); err != nil {
		t.Fatal(err)
	}
	if got := resource(t, db); got.Password != "moyu.moe" {
		t.Fatalf("password = %q", got.Password)
	}
	if got := jobOf(t, db, srcUUID); got.SourcePassword != "moyu.moe" || got.SourceComment != nil {
		t.Fatalf("a complete without archive_comment: job = %+v, want source_comment NULL", got)
	}
}

func TestCommentBackfill(t *testing.T) {
	db := normalizeDB(t)
	svc := New(db, artifacts(t), owners{allow: true})
	ctx := context.Background()
	claimOurs(t, svc)
	if _, err := svc.Complete(ctx, testResource, testUser, complete(productUUID, srcUUID, false)); err != nil {
		t.Fatal(err)
	}
	// An original the artifact service no longer holds, still queued for purge.
	for _, q := range []string{
		`INSERT INTO patch_resource_normalize (source_artifact_uuid, resource_id, state)
			VALUES ('` + missingUUID + `', 1499999902, 'done')`,
		`INSERT INTO patch_resource_repack_purge (artifact_uuid, resource_id) VALUES ('` + missingUUID + `', 1499999902)`,
		// Read already.
		`INSERT INTO patch_resource_normalize (source_artifact_uuid, resource_id, state, source_comment)
			VALUES ('` + oldSrcUUID + `', 1499999902, 'done', '')`,
		`INSERT INTO patch_resource_repack_purge (artifact_uuid, resource_id) VALUES ('` + oldSrcUUID + `', 1499999902)`,
		// Purged.
		`INSERT INTO patch_resource_normalize (source_artifact_uuid, resource_id, state)
			VALUES ('` + olderSrcUUID + `', 1499999902, 'done')`,
	} {
		if err := db.Exec(q).Error; err != nil {
			t.Fatal(err)
		}
	}

	items, next, err := svc.CommentSources(ctx, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].SourceArtifactUUID != srcUUID || items[0].Filename != "汉化补丁 v1.2.rar" ||
		items[0].Bytes != 1048576 || !strings.HasSuffix(items[0].DownloadURL, srcUUID) || next != srcUUID {
		t.Fatalf("first page = %+v, next %q", items, next)
	}
	items, next, err = svc.CommentSources(ctx, 1, next)
	if err != nil || len(items) != 0 || next != missingUUID {
		t.Fatalf("a missing source: items %+v, next %q, %v; want it skipped with the cursor past it", items, next, err)
	}
	items, next, err = svc.CommentSources(ctx, 1, next)
	if err != nil || len(items) != 0 || next != "" {
		t.Fatalf("exhausted list: items %+v, next %q, %v", items, next, err)
	}

	if err := svc.RecordComment(ctx, testResource, CommentRequest{SourceArtifactUUID: srcUUID, ArchiveComment: comment}); err != nil {
		t.Fatal(err)
	}
	if got := archiveComment(t, db); got != comment {
		t.Fatalf("archive_comment after backfilling the current product's source = %q", got)
	}
	if err := svc.RecordComment(ctx, testResource, CommentRequest{SourceArtifactUUID: srcUUID, ArchiveComment: "again"}); err != nil {
		t.Fatalf("a rewrite: %v", err)
	}
	if items, _, _ := svc.CommentSources(ctx, MaxCommentList, ""); len(items) != 0 {
		t.Fatalf("a read source is listed again: %+v", items)
	}

	// The author replaced the product: the source's comment is not the file's.
	db.Exec(`UPDATE patch_resource SET artifact_uuid = ?, archive_comment = '' WHERE id = ?`, newSrcUUID, testResource)
	if err := svc.RecordComment(ctx, testResource, CommentRequest{SourceArtifactUUID: srcUUID, ArchiveComment: comment}); err != nil {
		t.Fatal(err)
	}
	if got := archiveComment(t, db); got != "" {
		t.Fatalf("a replaced file took its predecessor's comment: %q", got)
	}

	if err := svc.RecordComment(ctx, testResource, CommentRequest{SourceArtifactUUID: newSrcUUID}); !errors.Is(err, ErrNoJob) {
		t.Fatalf("a source with no job: %v, want ErrNoJob", err)
	}
}

func TestCommentOnAFailedFileShowsOnTheResource(t *testing.T) {
	db := normalizeDB(t)
	svc := New(db, artifacts(t), owners{allow: true})
	ctx := context.Background()
	claimOurs(t, svc)
	if err := svc.Fail(ctx, testResource, FailRequest{SourceArtifactUUID: srcUUID, Code: "wrong_password"}); err != nil {
		t.Fatal(err)
	}
	items, _, err := svc.CommentSources(ctx, MaxCommentList, "")
	if err != nil || !slices.ContainsFunc(items, func(c CommentSource) bool { return c.SourceArtifactUUID == srcUUID }) {
		t.Fatalf("a failed resource's current file is not listed: %+v, %v", items, err)
	}
	if err := svc.RecordComment(ctx, testResource, CommentRequest{SourceArtifactUUID: srcUUID, ArchiveComment: comment}); err != nil {
		t.Fatal(err)
	}
	if got := archiveComment(t, db); got != comment {
		t.Fatalf("archive_comment = %q", got)
	}
}

func TestAPasswordFixRequeuesAWrongPasswordFailure(t *testing.T) {
	db := normalizeDB(t)
	svc := New(db, artifacts(t), owners{allow: true})
	patches := patchService.New(repository.New(db), nil, db, nil, nil, nil, nil, nil, nil)
	ctx := context.Background()
	edit := func(password string) {
		t.Helper()
		var cur model.PatchResource
		if err := db.First(&cur, testResource).Error; err != nil {
			t.Fatal(err)
		}
		cur.Password = password
		if _, err := patches.UpdateResource(ctx, testResource, testUser, &cur, "", 0); err != nil {
			t.Fatal(err)
		}
	}

	claimOurs(t, svc)
	if err := svc.Fail(ctx, testResource, FailRequest{SourceArtifactUUID: srcUUID, Code: "too_large"}); err != nil {
		t.Fatal(err)
	}
	edit("fixed")
	if jobs := claimOurs(t, svc); len(jobs) != 0 {
		t.Fatalf("a failure other than wrong_password was requeued: %+v", jobs)
	}

	db.Exec(`UPDATE patch_resource_normalize SET code = 'wrong_password' WHERE source_artifact_uuid = ?`, srcUUID)
	edit("fixed")
	if jobs := claimOurs(t, svc); len(jobs) != 0 {
		t.Fatalf("an edit that kept the password requeued the file: %+v", jobs)
	}
	edit("fixed again")
	jobs := claimOurs(t, svc)
	if len(jobs) != 1 || jobs[0].Password != "fixed again" {
		t.Fatalf("after the author fixed the password claimed %+v", jobs)
	}
}

func TestAnEditKeepingTheFileKeepsTheComment(t *testing.T) {
	db := normalizeDB(t)
	patches := patchService.New(repository.New(db), nil, db, nil, nil, nil, nil, nil, nil)
	db.Exec(`UPDATE patch_resource SET archive_comment = ? WHERE id = ?`, comment, testResource)
	var cur model.PatchResource
	if err := db.First(&cur, testResource).Error; err != nil {
		t.Fatal(err)
	}
	cur.Name = "renamed"
	if _, err := patches.UpdateResource(context.Background(), testResource, testUser, &cur, "", 0); err != nil {
		t.Fatal(err)
	}
	if got := archiveComment(t, db); got != comment {
		t.Fatalf("an edit that kept the file dropped the comment: %q", got)
	}
}

func TestCheckComment(t *testing.T) {
	for in, ok := range map[string]bool{
		"":                                     true,
		comment:                                true,
		strings.Repeat("a", maxCommentBytes):   true,
		strings.Repeat("a", maxCommentBytes+1): false,
		"a\x00b":                               false,
		"\xff\xfe":                             false,
	} {
		if err := CheckComment(in); (err == nil) != ok {
			t.Errorf("CheckComment(%q...) = %v, want ok=%v", in[:min(len(in), 8)], err, ok)
		}
	}
}

func TestAWrongPasswordTellsTheAuthorOnce(t *testing.T) {
	db := normalizeDB(t)
	svc := New(db, artifacts(t), owners{allow: true})
	ctx := context.Background()
	messages := func() []string {
		t.Helper()
		var out []string
		if err := db.Raw(`SELECT content FROM user_message WHERE recipient_id = ? AND link = '/resource/1499999902'`,
			testUser).Scan(&out).Error; err != nil {
			t.Fatal(err)
		}
		return out
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM user_message WHERE recipient_id = ?`, testUser) })
	db.Exec(`DELETE FROM user_message WHERE recipient_id = ?`, testUser)
	db.Exec(`UPDATE patch_resource SET password = '', name = '汉化补丁' WHERE id = ?`, testResource)

	claimOurs(t, svc)
	if err := svc.Fail(ctx, testResource, FailRequest{SourceArtifactUUID: srcUUID, Code: "wrong_password"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Fail(ctx, testResource, FailRequest{SourceArtifactUUID: srcUUID, Code: "wrong_password"}); err != nil {
		t.Fatal(err)
	}
	got := messages()
	if len(got) != 1 || !strings.Contains(got[0], "「汉化补丁」是加密压缩包，但没有填写解压密码") {
		t.Fatalf("messages = %q, want one", got)
	}

	resources := []model.PatchResource{{Storage: "s3", ArtifactUUID: srcUUID}, {Storage: "s3", ArtifactUUID: newSrcUUID}}
	if err := repository.New(db).MarkAwaitingPassword(resources); err != nil {
		t.Fatal(err)
	}
	if !resources[0].AwaitingPassword || resources[1].AwaitingPassword {
		t.Fatalf("awaiting_password = %v, %v", resources[0].AwaitingPassword, resources[1].AwaitingPassword)
	}
}

func TestOtherFailuresDoNotNotify(t *testing.T) {
	db := normalizeDB(t)
	svc := New(db, artifacts(t), owners{allow: true})
	t.Cleanup(func() { db.Exec(`DELETE FROM user_message WHERE recipient_id = ?`, testUser) })
	claimOurs(t, svc)
	if err := svc.Fail(context.Background(), testResource, FailRequest{SourceArtifactUUID: srcUUID, Code: "too_large"}); err != nil {
		t.Fatal(err)
	}
	var n int
	db.Raw(`SELECT count(*) FROM user_message WHERE recipient_id = ?`, testUser).Scan(&n)
	if n != 0 {
		t.Fatalf("%d messages for a non-password failure", n)
	}
}
