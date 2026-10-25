package cron

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/pkg/catalogv2"
	"kun-galgame-patch-api/pkg/upstream"
	"kun-galgame-patch-api/pkg/utils"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	lifecycleUser   = 1499999911
	deletedPage     = 1499999911
	livePage        = 1499999912
	mergedPage      = 1499999913
	revokedOld      = 1499999914
	revokedSurvivor = 1499999915
	notAPage        = 1499999916
	neverRecorded   = 1499999917
)

// Runs only against the launcher-provided TEST_DATABASE_DSN, migrated through
// 051, with `go test -count=1 -p 1`.
func lifecycleDB(t *testing.T) *gorm.DB {
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
		WHERE table_name = 'patch' AND column_name = 'catalog_deleted_at')`).Scan(&migrated).Error; err != nil || !migrated {
		t.Fatalf("TEST_DATABASE_DSN is not migrated through 051: %v", err)
	}
	clean := func() {
		db.Exec(`DELETE FROM patch_redirect WHERE old_id BETWEEN ? AND ?`, deletedPage, neverRecorded)
		db.Exec(`DELETE FROM patch WHERE id BETWEEN ? AND ?`, deletedPage, neverRecorded)
		db.Exec(`DELETE FROM "user" WHERE id = ?`, lifecycleUser)
	}
	clean()
	t.Cleanup(clean)
	mustExec(t, db, `INSERT INTO "user" (id, updated) VALUES (?, now())`, lifecycleUser)
	for _, id := range []int{deletedPage, livePage, mergedPage, revokedSurvivor} {
		mustExec(t, db, `INSERT INTO patch (id, vndb_id, user_id, updated) VALUES (?, ?, ?, now())`,
			id, fmt.Sprintf("v-lifecycle-%d", id), lifecycleUser)
	}
	return db
}

func mustExec(t *testing.T, db *gorm.DB, q string, args ...any) {
	t.Helper()
	if err := db.Exec(q, args...).Error; err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

type deletionAnswers map[int64]error

func (a deletionAnswers) WorkDeletion(_ context.Context, id int64) (*catalogv2.Deletion, error) {
	if err, ok := a[id]; ok {
		return nil, err
	}
	if id == deletedPage {
		return &catalogv2.Deletion{Reason: "not_a_work", DeletedAt: "2026-10-23T08:00:00Z"}, nil
	}
	return nil, nil
}

func deletionStamp(t *testing.T, db *gorm.DB, id int) (*time.Time, *string) {
	t.Helper()
	var row struct {
		CatalogDeletedAt     *time.Time
		CatalogDeletedReason *string
	}
	if err := db.Raw(`SELECT catalog_deleted_at, catalog_deleted_reason FROM patch WHERE id = ?`, id).
		Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row.CatalogDeletedAt, row.CatalogDeletedReason
}

func TestDeletedWorkIsHiddenUntilRestored(t *testing.T) {
	db := lifecycleDB(t)
	ctx := context.Background()
	merged := &upstream.Error{Service: "catalog", Kind: upstream.NotFound, Status: 404, Code: catalogv2.CodeEntityMerged,
		Cause: &catalogv2.Problem{Code: catalogv2.CodeEntityMerged, CurrentID: "7"}}

	gone := []int64{deletedPage, mergedPage, notAPage}
	n, err := markDeletedPages(ctx, db, deletionAnswers{mergedPage: merged}, gone)
	if err != nil || n != 1 {
		t.Fatalf("markDeletedPages = (%d, %v), want only the deleted work stamped", n, err)
	}
	at, reason := deletionStamp(t, db, deletedPage)
	if at == nil || !at.Equal(time.Date(2026, 10, 23, 8, 0, 0, 0, time.UTC)) || reason == nil || *reason != "not_a_work" {
		t.Fatalf("stamp = (%v, %v), want catalog's deleted_at and reason", at, reason)
	}
	if at, _ := deletionStamp(t, db, mergedPage); at != nil {
		t.Error("a merged work was stamped deleted; the merge cron moves that page instead")
	}

	t.Run("the replayed tail asks catalog nothing more", func(t *testing.T) {
		refuse := deletionAnswers{deletedPage: upstream.Transport("catalog", "GET", context.Canceled)}
		if n, err := markDeletedPages(ctx, db, refuse, gone[:1]); err != nil || n != 0 {
			t.Fatalf("replay = (%d, %v), want the stamped page skipped without a probe", n, err)
		}
	})

	t.Run("lists leave the page out under every gate", func(t *testing.T) {
		for _, cl := range []string{utils.ContentLimitSFW, utils.ContentLimitNSFW, utils.ContentLimitAll, ""} {
			var ids []int
			q := utils.ScopeListablePatches(db.Table("patch").Select("id"), cl).
				Where("id BETWEEN ? AND ?", deletedPage, livePage)
			if err := q.Scan(&ids).Error; err != nil {
				t.Fatal(err)
			}
			if len(ids) != 1 || ids[0] != livePage {
				t.Errorf("content_limit %q listed %v, want only %d", cl, ids, livePage)
			}
		}
	})

	t.Run("a probe that fails stops the tick", func(t *testing.T) {
		down := deletionAnswers{livePage: &upstream.Error{Service: "catalog", Kind: upstream.Unavailable, Status: 503}}
		if _, err := markDeletedPages(ctx, db, down, []int64{livePage}); err == nil {
			t.Fatal("an unavailable catalog was read as a live work; the cursor would move past the deletion")
		}
	})

	n, err = clearRestoredPages(db, []int64{deletedPage, livePage})
	if err != nil || n != 1 {
		t.Fatalf("clearRestoredPages = (%d, %v), want the restored page cleared", n, err)
	}
	if at, reason := deletionStamp(t, db, deletedPage); at != nil || reason != nil {
		t.Fatalf("stamp after restore = (%v, %v), want none", at, reason)
	}
}

func TestRevokedMergeDropsTheRedirect(t *testing.T) {
	db := lifecycleDB(t)
	mustExec(t, db, `INSERT INTO patch_redirect (old_id, new_id, scope) VALUES (?, ?, 'merge'), (?, ?, 'legacy')`,
		revokedOld, revokedSurvivor, revokedOld, revokedSurvivor)

	items := []catalogv2.Redirect{
		{OldID: revokedOld, CurrentID: revokedOld, Revoked: true},
		{OldID: neverRecorded, CurrentID: neverRecorded, Revoked: true},
	}

	var dry MergeSyncReport
	if err := applyRedirects(db, items, false, &dry, nil); err != nil {
		t.Fatal(err)
	}
	if dry.Revoked != 1 || dry.Skipped != 1 {
		t.Fatalf("dry run = %+v, want one revoke and one row this site never recorded", dry)
	}
	if ledger, _ := mergeLedger(db, []int{revokedOld}); ledger[revokedOld] != revokedSurvivor {
		t.Fatal("a dry run deleted the redirect")
	}

	var report MergeSyncReport
	if err := applyRedirects(db, items, true, &report, nil); err != nil {
		t.Fatal(err)
	}
	if report.Revoked != 1 || report.Applied() != 1 {
		t.Fatalf("report = %+v", report)
	}
	if ledger, _ := mergeLedger(db, []int{revokedOld}); len(ledger) != 0 {
		t.Fatalf("merge ledger = %v, want /galgame/%d to stop redirecting", ledger, revokedOld)
	}
	var legacy int64
	db.Raw(`SELECT count(*) FROM patch_redirect WHERE scope = 'legacy' AND old_id = ?`, revokedOld).Scan(&legacy)
	if legacy != 1 {
		t.Error("the revoke took the legacy row too; /patch/<n> is a different namespace")
	}

	var replay MergeSyncReport
	if err := applyRedirects(db, items[:1], true, &replay, nil); err != nil || replay.Applied() != 0 {
		t.Fatalf("replayed tail = (%+v, %v), want nothing left to do", replay, err)
	}
}

// 053: the stamp reaches letmoe's change feed and both community pushes in the
// same transaction, not at the next daily reconcile.
func TestDeletionStampReachesTheFeeds(t *testing.T) {
	db := lifecycleDB(t)
	var public, disabled int
	db.Raw(`INSERT INTO patch_resource (storage, user_id, galgame_id, updated) VALUES ('user', ?, ?, now()) RETURNING id`,
		lifecycleUser, deletedPage).Scan(&public)
	db.Raw(`INSERT INTO patch_resource (storage, user_id, galgame_id, status, updated) VALUES ('user', ?, ?, 1, now()) RETURNING id`,
		lifecycleUser, deletedPage).Scan(&disabled)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM patch_resource_change WHERE resource_id IN (?, ?)`, public, disabled)
		db.Exec(`DELETE FROM activity_push_queue WHERE key = ?`, fmt.Sprintf("patch_resource:%d", public))
		db.Exec(`DELETE FROM anchor_presentation_queue WHERE anchor_id IN (?, ?)`, fmt.Sprint(deletedPage), fmt.Sprint(public))
	})
	kinds := func() []string {
		var out []string
		db.Raw(`SELECT kind FROM patch_resource_change WHERE resource_id IN (?, ?) ORDER BY seq`, public, disabled).Scan(&out)
		return out
	}
	db.Exec(`DELETE FROM patch_resource_change WHERE resource_id IN (?, ?)`, public, disabled)
	db.Exec(`DELETE FROM activity_push_queue WHERE key = ?`, fmt.Sprintf("patch_resource:%d", public))
	db.Exec(`DELETE FROM anchor_presentation_queue WHERE anchor_id = ?`, fmt.Sprint(deletedPage))

	if _, err := markDeletedPages(context.Background(), db, deletionAnswers{}, []int64{deletedPage}); err != nil {
		t.Fatal(err)
	}
	if got := kinds(); len(got) != 1 || got[0] != "hidden" {
		t.Fatalf("feed after the stamp = %v, want one hidden for the public resource only", got)
	}
	var queued, presented int64
	db.Raw(`SELECT count(*) FROM activity_push_queue WHERE key = ?`, fmt.Sprintf("patch_resource:%d", public)).Scan(&queued)
	db.Raw(`SELECT count(*) FROM anchor_presentation_queue WHERE anchor_kind = 1 AND anchor_id = ?`, fmt.Sprint(deletedPage)).Scan(&presented)
	if queued != 1 || presented != 1 {
		t.Errorf("activity queued %d, page presentation queued %d, want both", queued, presented)
	}

	if _, err := clearRestoredPages(db, []int64{deletedPage}); err != nil {
		t.Fatal(err)
	}
	if got := kinds(); len(got) != 2 || got[1] != "restored" {
		t.Fatalf("feed after the restore = %v, want restored to follow", got)
	}
}

func TestLiveWorkPredicates(t *testing.T) {
	db := lifecycleDB(t)
	mustExec(t, db, `UPDATE patch SET catalog_deleted_at = now() WHERE id = ?`, deletedPage)
	var res []int
	for _, page := range []int{deletedPage, livePage} {
		var id int
		db.Raw(`INSERT INTO patch_resource (storage, user_id, galgame_id, updated) VALUES ('user', ?, ?, now()) RETURNING id`,
			lifecycleUser, page).Scan(&id)
		res = append(res, id)
	}
	var pages, resources []int
	db.Raw(`SELECT id FROM patch p WHERE id IN (?, ?) AND `+utils.LiveWorkPage("p"), deletedPage, livePage).Scan(&pages)
	db.Raw(`SELECT id FROM patch_resource r WHERE id IN ? AND `+utils.LiveWorkResource("r"), res).Scan(&resources)
	if len(pages) != 1 || pages[0] != livePage || len(resources) != 1 || resources[0] != res[1] {
		t.Fatalf("pages %v resources %v, want only the live work's", pages, resources)
	}
}

func TestDisplayVerdictReachesThePage(t *testing.T) {
	db := lifecycleDB(t)
	work := func(id int, limit string) string {
		return fmt.Sprintf(`{"object":"work","id":"%d","medium":"galgame","display_name":"W","localized":{},`+
			`"olang":"ja","content_rating":"all_ages","content_limit":"%s","claim":null}`, id, limit)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"object":"list","items":[%s,%s]}`, work(livePage, "nsfw"), work(deletedPage, "sfw"))
	}))
	t.Cleanup(srv.Close)
	galgame := galgameClient.NewWithKey(srv.URL, "nmk_test_key")
	mustExec(t, db, `UPDATE patch SET content_limit = 'sfw' WHERE id = ?`, livePage)

	ids := []int64{livePage, deletedPage}
	n, err := applyDisplayVerdicts(context.Background(), db, galgame, ids)
	if err != nil || n != 2 {
		t.Fatalf("applyDisplayVerdicts = (%d, %v), want both pages written", n, err)
	}
	got := map[int]*string{}
	for _, id := range []int{livePage, deletedPage} {
		var cl *string
		db.Raw(`SELECT content_limit FROM patch WHERE id = ?`, id).Scan(&cl)
		got[id] = cl
	}
	if got[livePage] == nil || *got[livePage] != "nsfw" || got[deletedPage] == nil || *got[deletedPage] != "sfw" {
		t.Fatalf("content_limit = live %v, deleted %v, want nsfw and sfw", got[livePage], got[deletedPage])
	}

	if n, err := applyDisplayVerdicts(context.Background(), db, galgame, ids); err != nil || n != 0 {
		t.Fatalf("replayed tail = (%d, %v), want nothing left to write", n, err)
	}
}

func TestUnmirroredPageIsFilledOnceCaughtUp(t *testing.T) {
	db := lifecycleDB(t)
	mustExec(t, db, `UPDATE patch SET catalog_deleted_at = now() WHERE id = ?`, deletedPage)
	verdicts := map[string]string{fmt.Sprint(livePage): "nsfw", fmt.Sprint(mergedPage): "sfw", fmt.Sprint(deletedPage): "sfw"}
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v2/catalog/changes" {
			_, _ = w.Write([]byte(`{"object":"list","items":[],"next_cursor":null}`))
			return
		}
		var items []string
		for _, id := range strings.Split(r.URL.Query().Get("ids"), ",") {
			asked = append(asked, id)
			if cl, ok := verdicts[id]; ok {
				items = append(items, fmt.Sprintf(`{"object":"work","id":"%s","medium":"galgame","display_name":"W",`+
					`"localized":{},"olang":"ja","content_rating":"all_ages","content_limit":"%s","claim":null}`, id, cl))
			}
		}
		_, _ = fmt.Fprintf(w, `{"object":"list","items":[%s]}`, strings.Join(items, ","))
	}))
	t.Cleanup(srv.Close)
	galgame := galgameClient.NewWithKey(srv.URL, "nmk_test_key")

	n, caughtUp, err := RunCatalogMirrorSync(context.Background(), db, galgame.V2(), galgame)
	if err != nil || !caughtUp || n < 2 {
		t.Fatalf("RunCatalogMirrorSync = (%d, %v, %v), want both hydrated pages filled", n, caughtUp, err)
	}
	for id, want := range map[int]string{livePage: "nsfw", mergedPage: "sfw"} {
		var cl *string
		db.Raw(`SELECT content_limit FROM patch WHERE id = ?`, id).Scan(&cl)
		if cl == nil || *cl != want {
			t.Errorf("page %d content_limit = %v, want %s", id, cl, want)
		}
	}
	if slices.Contains(asked, fmt.Sprint(deletedPage)) {
		t.Error("a page catalog deleted was hydrated; the deletion stamp already hides it")
	}
}
