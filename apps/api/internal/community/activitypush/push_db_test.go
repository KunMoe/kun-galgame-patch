package activitypush

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/upstream"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Runs only against the launcher-provided TEST_DATABASE_DSN, migrated through
// 045 (`go run ./cmd/migrate -yes`), with `go test -count=1 -p 1`. The queues and
// the ledgers are emptied per test, so the database must be a throwaway one.
func testDB(t *testing.T) *gorm.DB {
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
	if err := db.Raw(`SELECT to_regclass('anchor_presentation_queue') IS NOT NULL`).Scan(&migrated).Error; err != nil || !migrated {
		t.Fatalf("TEST_DATABASE_DSN is not migrated through 045 (go run ./cmd/migrate -yes): %v", err)
	}
	reset := func() {
		db.Exec(`DELETE FROM patch_resource_revision WHERE resource_id IN (SELECT id FROM patch_resource WHERE galgame_id BETWEEN 990000 AND 990099)`)
		db.Exec(`DELETE FROM patch_resource WHERE galgame_id BETWEEN 990000 AND 990099`)
		db.Exec(`DELETE FROM patch WHERE id BETWEEN 990000 AND 990099`)
		db.Exec(`DELETE FROM activity_push_queue`)
		db.Exec(`DELETE FROM activity_push_sent`)
		db.Exec(`DELETE FROM anchor_presentation_queue`)
		db.Exec(`DELETE FROM anchor_presentation_sent`)
	}
	reset()
	t.Cleanup(reset)
	for _, uid := range []int{990001, 990002} {
		if err := db.Exec(`INSERT INTO "user" (id, updated) VALUES (?, now()) ON CONFLICT DO NOTHING`, uid).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func addPatch(t *testing.T, db *gorm.DB, id int) {
	t.Helper()
	if err := db.Exec(`INSERT INTO patch (id, vndb_id, user_id, updated) VALUES (?, ?, 990001, now())`,
		id, "test-"+time.Now().Format("150405.000000")).Error; err != nil {
		t.Fatal(err)
	}
}

func addResource(t *testing.T, db *gorm.DB, patchID int, name string) int {
	t.Helper()
	var id int
	err := db.Raw(`
		INSERT INTO patch_resource (storage, name, size, type, language, platform, user_id, galgame_id, updated)
		VALUES ('user', ?, '1.2GB', '["manual"]', '["zh-Hans"]', '["windows"]', 990001, ?, now())
		RETURNING id`, name, patchID).Scan(&id).Error
	if err != nil {
		t.Fatal(err)
	}
	return id
}

type fakeCatalog struct {
	mu   sync.Mutex
	err  error
	gone map[int]bool
}

func (f *fakeCatalog) GalgameBatch(_ context.Context, ids []int, contentLimit string) ([]galgameClient.GalgameBrief, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if contentLimit != "" {
		return nil, errors.New("the pusher must read with both gates open")
	}
	if f.err != nil {
		return nil, f.err
	}
	var out []galgameClient.GalgameBrief
	for _, id := range ids {
		if !f.gone[id] {
			out = append(out, galgameClient.GalgameBrief{
				ID: id, NameZhCn: "千恋＊万花", ContentLimit: "sfw",
				EffectiveBannerHash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			})
		}
	}
	return out, nil
}

type fakeCommunity struct {
	mu      sync.Mutex
	batches [][]communityclient.ActivityItem
	reject  map[string]bool // a batch holding one of these answers 422
	during  func()
	stored  []communityclient.SiteActivity

	presentations       [][]communityclient.AnchorPresentation
	storedPresentations []communityclient.StoredAnchorPresentation
	walls               []communityclient.AuthorPostView
	wallsErr            error
}

func (f *fakeCommunity) WriteAnchorPresentations(_ context.Context, items []communityclient.AnchorPresentation) (*communityclient.AnchorPresentationWriteResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, it := range items {
		if f.reject[it.AnchorID] {
			return nil, &upstream.Error{Service: "community", Status: http.StatusUnprocessableEntity, Kind: upstream.Internal}
		}
	}
	f.presentations = append(f.presentations, items)
	out := &communityclient.AnchorPresentationWriteResponse{}
	for _, it := range items {
		outcome := communityclient.ActivityCreated
		if it.Removed {
			outcome = communityclient.ActivityRemoved
		}
		out.Results = append(out.Results, communityclient.AnchorPresentationOutcome{
			AnchorKind: it.AnchorKind, AnchorID: it.AnchorID, Outcome: outcome,
		})
	}
	return out, nil
}

func (f *fakeCommunity) ListAnchorPresentations(context.Context, string, int) (*communityclient.AnchorPresentationPage, error) {
	return &communityclient.AnchorPresentationPage{Presentations: f.storedPresentations}, nil
}

// ListSitePosts answers the walls one post per page, so a sweep has to follow
// the cursor to see them all.
func (f *fakeCommunity) ListSitePosts(_ context.Context, q communityclient.SitePostsQuery) (*communityclient.PostFeedResponse, error) {
	if q.Kind != communityclient.KindComments || q.AnchorKind != communityclient.AnchorSiteGame {
		return nil, errors.New("the wall sweep must ask for comments on game walls only")
	}
	if f.wallsErr != nil {
		return nil, f.wallsErr
	}
	i := 0
	if q.Cursor != "" {
		i, _ = strconv.Atoi(q.Cursor)
	}
	if i >= len(f.walls) {
		return &communityclient.PostFeedResponse{}, nil
	}
	next := ""
	if i+1 < len(f.walls) {
		next = strconv.Itoa(i + 1)
	}
	return &communityclient.PostFeedResponse{Posts: f.walls[i : i+1], NextCursor: next}, nil
}

func (f *fakeCommunity) sentPresentations() []communityclient.AnchorPresentation {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []communityclient.AnchorPresentation
	for _, b := range f.presentations {
		out = append(out, b...)
	}
	return out
}

func (f *fakeCommunity) WriteActivities(_ context.Context, items []communityclient.ActivityItem) (*communityclient.ActivityWriteResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.during != nil {
		f.during()
	}
	for _, it := range items {
		if f.reject[it.Key] {
			return nil, &upstream.Error{Service: "community", Status: http.StatusUnprocessableEntity, Kind: upstream.Internal}
		}
	}
	f.batches = append(f.batches, items)
	out := &communityclient.ActivityWriteResponse{}
	for _, it := range items {
		outcome := communityclient.ActivityCreated
		if it.Removed {
			outcome = communityclient.ActivityRemoved
		}
		out.Results = append(out.Results, communityclient.ActivityOutcome{Key: it.Key, Outcome: outcome})
	}
	return out, nil
}

func (f *fakeCommunity) ListSiteActivities(context.Context, string, int) (*communityclient.SiteActivityPage, error) {
	return &communityclient.SiteActivityPage{Activities: f.stored}, nil
}

func (f *fakeCommunity) sentItems() []communityclient.ActivityItem {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []communityclient.ActivityItem
	for _, b := range f.batches {
		out = append(out, b...)
	}
	return out
}

func newTestPusher(db *gorm.DB, com *fakeCommunity, cat *fakeCatalog) *Pusher {
	return New(db, com, cat, "https://www.moyu.moe")
}

// queued maps each queued key to its backfill flag.
func queued(t *testing.T, db *gorm.DB) map[string]bool {
	t.Helper()
	var rows []claim
	if err := db.Raw(`SELECT key, backfill FROM activity_push_queue`).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, r := range rows {
		out[r.Key] = r.Backfill
	}
	return out
}

func drain(t *testing.T, p *Pusher) {
	t.Helper()
	for range 10 {
		n, err := p.RunOnce(context.Background())
		if err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if n == 0 {
			return
		}
	}
	t.Fatal("queue never emptied")
}

func TestInsertPushesThePublishWithNotify(t *testing.T) {
	db := testDB(t)
	addPatch(t, db, 990010)
	rid := addResource(t, db, 990010, "汉化补丁")

	if backfill, ok := queued(t, db)[resourceKey(rid)]; !ok || backfill {
		t.Fatalf("queue after insert = %v", queued(t, db))
	}
	com := &fakeCommunity{}
	before := time.Now().UnixMicro()
	drain(t, newTestPusher(db, com, &fakeCatalog{}))
	after := time.Now().UnixMicro()

	items := com.sentItems()
	if len(items) != 1 {
		t.Fatalf("pushed %d items", len(items))
	}
	it := items[0]
	if it.Key != resourceKey(rid) || it.ActorID != 990001 || it.Verb != "publish" || !it.Notify || it.Removed ||
		it.ObjectKind != "patch_resource" || it.ObjectLabel != "Galgame 补丁" ||
		it.Title != "千恋＊万花 · 汉化补丁" || it.Excerpt != "人工翻译补丁 · 简体中文 · Windows · 1.2GB" ||
		it.URL != "https://www.moyu.moe/resource/"+itoa(rid) || it.ContentLimit != "sfw" ||
		it.WorkID == nil || *it.WorkID != 990010 || it.CoverImageHash == "" || it.OccurredAt == nil {
		t.Errorf("item = %+v", it)
	}
	if it.Revision < before || it.Revision > after {
		t.Errorf("revision %d outside [%d, %d]", it.Revision, before, after)
	}
	if len(queued(t, db)) != 0 {
		t.Errorf("queue = %v", queued(t, db))
	}
	sent, _ := store{db: db}.sent(context.Background(), []string{it.Key})
	if s := sent[it.Key]; s.ActorID != 990001 || s.Revision != it.Revision || s.Removed {
		t.Errorf("ledger = %+v", s)
	}
}

// Infra review (2): community stores a tombstone for a key it never saw and
// counts it as seen, so a resource hidden before its first push and restored
// later would never notify anyone.
func TestHiddenBeforeItsFirstPushSendsNothingAndItsRestoreNotifies(t *testing.T) {
	db := testDB(t)
	addPatch(t, db, 990011)
	rid := addResource(t, db, 990011, "")
	if err := db.Exec(`UPDATE patch_resource SET status = 2 WHERE id = ?`, rid).Error; err != nil {
		t.Fatal(err)
	}

	com := &fakeCommunity{}
	p := newTestPusher(db, com, &fakeCatalog{})
	drain(t, p)
	if items := com.sentItems(); len(items) != 0 {
		t.Fatalf("a never-pushed hidden resource sent %+v", items)
	}
	if len(queued(t, db)) != 0 {
		t.Fatalf("queue = %v", queued(t, db))
	}

	if err := db.Exec(`UPDATE patch_resource SET status = 0 WHERE id = ?`, rid).Error; err != nil {
		t.Fatal(err)
	}
	drain(t, p)
	items := com.sentItems()
	if len(items) != 1 || items[0].Removed || !items[0].Notify {
		t.Fatalf("restore pushed %+v", items)
	}
	if items[0].Title != "千恋＊万花" {
		t.Errorf("unnamed resource title = %q", items[0].Title)
	}
}

// Infra review (3): a catalog read that fails is an outage, not a verdict.
func TestACatalogFailurePushesNothingAndKeepsTheQueue(t *testing.T) {
	db := testDB(t)
	addPatch(t, db, 990012)
	rid := addResource(t, db, 990012, "x")

	com := &fakeCommunity{}
	cat := &fakeCatalog{err: &upstream.Error{Service: "catalog", Kind: upstream.Unavailable, Status: 503}}
	p := newTestPusher(db, com, cat)
	if _, err := p.RunOnce(context.Background()); err == nil {
		t.Fatal("RunOnce swallowed the catalog failure")
	}
	if items := com.sentItems(); len(items) != 0 {
		t.Fatalf("pushed %+v during a catalog outage", items)
	}
	if backfill, ok := queued(t, db)[resourceKey(rid)]; !ok || backfill {
		t.Fatalf("queue = %v", queued(t, db))
	}

	cat.err = nil
	drain(t, p)
	if items := com.sentItems(); len(items) != 1 || items[0].Removed || !items[0].Notify {
		t.Errorf("after recovery pushed %+v", items)
	}
}

func TestDeletingAPushedResourceTombstonesIt(t *testing.T) {
	db := testDB(t)
	addPatch(t, db, 990013)
	rid := addResource(t, db, 990013, "x")
	com := &fakeCommunity{}
	p := newTestPusher(db, com, &fakeCatalog{})
	drain(t, p)

	if err := db.Exec(`DELETE FROM patch_resource WHERE id = ?`, rid).Error; err != nil {
		t.Fatal(err)
	}
	drain(t, p)
	items := com.sentItems()
	if len(items) != 2 {
		t.Fatalf("pushed %+v", items)
	}
	tomb := items[1]
	if !tomb.Removed || tomb.Key != resourceKey(rid) || tomb.ActorID != 990001 || tomb.Revision <= items[0].Revision ||
		tomb.Verb != "" || tomb.Title != "" || tomb.WorkID != nil || tomb.OccurredAt != nil {
		t.Errorf("tombstone = %+v", tomb)
	}

	// Already a tombstone: nothing more to say about it.
	if err := (store{db: db}).enqueueBackfill(context.Background(), []string{resourceKey(rid)}); err != nil {
		t.Fatal(err)
	}
	drain(t, p)
	if n := len(com.sentItems()); n != 2 {
		t.Errorf("a second tombstone went out (%d items)", n)
	}
}

func TestAWorkCatalogNoLongerRendersTombstonesItsResources(t *testing.T) {
	db := testDB(t)
	addPatch(t, db, 990014)
	rid := addResource(t, db, 990014, "x")
	com := &fakeCommunity{}
	cat := &fakeCatalog{}
	p := newTestPusher(db, com, cat)
	drain(t, p)

	cat.gone = map[int]bool{990014: true}
	if err := db.Exec(`UPDATE patch SET published = NOT published WHERE id = 990014`).Error; err != nil {
		t.Fatal(err)
	}
	if _, ok := queued(t, db)[resourceKey(rid)]; !ok {
		t.Fatalf("unpublishing the page enqueued %v", queued(t, db))
	}
	drain(t, p)
	items := com.sentItems()
	if len(items) != 2 || !items[1].Removed {
		t.Errorf("pushed %+v", items)
	}
}

func TestAChangeDuringTheTickKeepsTheQueueRow(t *testing.T) {
	db := testDB(t)
	addPatch(t, db, 990015)
	rid := addResource(t, db, 990015, "before")
	com := &fakeCommunity{}
	com.during = func() {
		com.during = nil
		if err := db.Exec(`UPDATE patch_resource SET name = 'after' WHERE id = ?`, rid).Error; err != nil {
			t.Error(err)
		}
	}
	p := newTestPusher(db, com, &fakeCatalog{})
	if _, err := p.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := queued(t, db)[resourceKey(rid)]; !ok {
		t.Fatal("the rename made during the push was acked away")
	}
	drain(t, p)
	items := com.sentItems()
	if len(items) != 2 || items[1].Title != "千恋＊万花 · after" || !items[0].Notify || items[1].Notify {
		t.Errorf("pushed %+v", items)
	}
}

// Infra review of 1bf9c79b: notify used to be fixed at enqueue time, and the INSERT's
// was spent on a drain that pushed nothing because catalog did not yet show a
// newly claimed work. Its first resource never notified anyone.
func TestAWorkShownAfterItsFirstResourceStillNotifies(t *testing.T) {
	db := testDB(t)
	addPatch(t, db, 990019)
	rid := addResource(t, db, 990019, "x")
	com := &fakeCommunity{}
	cat := &fakeCatalog{gone: map[int]bool{990019: true}}
	p := newTestPusher(db, com, cat)
	drain(t, p)
	if items := com.sentItems(); len(items) != 0 {
		t.Fatalf("pushed %+v for a work catalog does not show", items)
	}

	cat.gone = nil
	if err := db.Exec(`UPDATE patch SET published = true WHERE id = 990019`).Error; err != nil {
		t.Fatal(err)
	}
	drain(t, p)
	items := com.sentItems()
	if len(items) != 1 || items[0].Key != resourceKey(rid) || !items[0].Notify {
		t.Errorf("pushed %+v", items)
	}
}

func TestABackfillRowNeverNotifiesUntilALiveChangeClearsIt(t *testing.T) {
	db := testDB(t)
	addPatch(t, db, 990020)
	old := addResource(t, db, 990020, "old")
	edited := addResource(t, db, 990020, "edited")
	if err := db.Exec(`UPDATE activity_push_queue SET backfill = true`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE patch_resource SET name = 'renamed' WHERE id = ?`, edited).Error; err != nil {
		t.Fatal(err)
	}
	if q := queued(t, db); !q[resourceKey(old)] || q[resourceKey(edited)] {
		t.Fatalf("queue = %v", q)
	}

	com := &fakeCommunity{}
	drain(t, newTestPusher(db, com, &fakeCatalog{}))
	notify := map[string]bool{}
	for _, it := range com.sentItems() {
		notify[it.Key] = it.Notify
	}
	if len(notify) != 2 || notify[resourceKey(old)] || !notify[resourceKey(edited)] {
		t.Errorf("notify by key = %v", notify)
	}
}

func TestOneRejectedItemDoesNotHoldTheBatch(t *testing.T) {
	db := testDB(t)
	addPatch(t, db, 990016)
	var ids []int
	for range 5 {
		ids = append(ids, addResource(t, db, 990016, "x"))
	}
	com := &fakeCommunity{reject: map[string]bool{resourceKey(ids[2]): true}}
	drain(t, newTestPusher(db, com, &fakeCatalog{}))

	items := com.sentItems()
	if len(items) != 4 {
		t.Fatalf("accepted %d items, want 4", len(items))
	}
	for _, it := range items {
		if it.Key == resourceKey(ids[2]) {
			t.Error("the rejected item was reported as accepted")
		}
	}
	if len(queued(t, db)) != 0 {
		t.Errorf("queue = %v", queued(t, db))
	}
}

func TestEditsFoldPerEditorAndBeijingDay(t *testing.T) {
	db := testDB(t)
	addPatch(t, db, 990017)
	rid := addResource(t, db, 990017, "x")
	// 16:30 UTC on the 25th is already the 26th in Beijing.
	late := time.Date(2026, 9, 25, 16, 30, 0, 0, time.UTC)
	for i, at := range []time.Time{late, late.Add(time.Hour), late.Add(-time.Hour)} {
		label := []string{"资源名称", "备注", "语言"}[i]
		if err := db.Exec(`INSERT INTO patch_resource_revision (resource_id, changes, actor_id, actor_role, created_at)
			VALUES (?, ?::jsonb, 990002, 1, ?)`, rid, `[{"field":"x","label":"`+label+`"},{"field":"download","label":"下载文件 / 链接 / 提取码 / 密码"}]`, at).Error; err != nil {
			t.Fatal(err)
		}
	}
	q := queued(t, db)
	day26 := editKey(rid, 990002, late)
	day25 := editKey(rid, 990002, late.Add(-time.Hour))
	if day26 != "patch_resource_edit:"+itoa(rid)+":990002:20260926" || day25[len(day25)-8:] != "20260925" {
		t.Fatalf("keys %q %q", day26, day25)
	}
	if _, ok := q[day26]; !ok {
		t.Fatalf("the trigger's key differs from Go's: queue = %v", q)
	}
	if _, ok := q[day25]; !ok {
		t.Fatalf("queue = %v", q)
	}

	com := &fakeCommunity{}
	drain(t, newTestPusher(db, com, &fakeCatalog{}))
	var edit *communityclient.ActivityItem
	for _, it := range com.sentItems() {
		if it.Key == day26 {
			edit = &it
		}
	}
	if edit == nil || edit.Verb != "edit" || edit.ActorID != 990002 || edit.Notify ||
		edit.Excerpt != "修改了 资源名称、下载信息、备注" || !edit.OccurredAt.Equal(late.Add(time.Hour)) {
		t.Errorf("edit = %+v", edit)
	}
}

func TestReconcileRepairsDriftAndTombstonesWhatIsGone(t *testing.T) {
	db := testDB(t)
	addPatch(t, db, 990018)
	live := addResource(t, db, 990018, "x")
	com := &fakeCommunity{}
	p := newTestPusher(db, com, &fakeCatalog{})
	drain(t, p)
	pushed := com.sentItems()[0]

	stale := communityclient.SiteActivity{
		Key: pushed.Key, ActorID: pushed.ActorID, Verb: pushed.Verb, ObjectKind: pushed.ObjectKind,
		ObjectLabel: pushed.ObjectLabel, Title: pushed.Title, Excerpt: pushed.Excerpt, URL: pushed.URL,
		CoverImageHash: &pushed.CoverImageHash, WorkID: pushed.WorkID, ContentLimit: "nsfw",
		OccurredAt: *pushed.OccurredAt, Revision: pushed.Revision,
	}
	// A key community holds live that this site has no row for, and whose push
	// was never recorded here: only community's listing names its actor.
	orphan := communityclient.SiteActivity{Key: "patch_resource:989999", ActorID: 990002, Verb: "publish", Revision: 5}
	// A row community writes for itself under this site: not ours to tombstone.
	own := communityclient.SiteActivity{Key: "community:post:1", ActorID: 990002, Verb: "comment", Revision: 5}
	com.stored = []communityclient.SiteActivity{stale, orphan, own}

	report, err := p.Reconcile(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	q := queued(t, db)
	if _, ok := q[resourceKey(live)]; !ok || report.Enqueued != 2 || len(q) != 2 {
		t.Fatalf("report %+v, queue %v", report, q)
	}
	if !q[orphan.Key] || !q[resourceKey(live)] {
		t.Errorf("reconcile queued a live row: %v", q)
	}
	if sent, _ := (store{db: db}).sent(context.Background(), []string{own.Key}); len(sent) != 0 {
		t.Errorf("community's own row reached the ledger: %+v", sent)
	}

	drain(t, p)
	items := com.sentItems()[1:]
	byKey := map[string]communityclient.ActivityItem{}
	for _, it := range items {
		byKey[it.Key] = it
	}
	if fix := byKey[resourceKey(live)]; fix.ContentLimit != "sfw" || fix.Notify || fix.Removed {
		t.Errorf("repair = %+v", fix)
	}
	if tomb := byKey[orphan.Key]; !tomb.Removed || tomb.ActorID != 990002 {
		t.Errorf("orphan = %+v", tomb)
	}
}

func itoa(n int) string { return resourceKey(n)[len(prefixResource):] }
