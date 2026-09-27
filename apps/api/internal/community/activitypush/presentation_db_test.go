package activitypush

import (
	"context"
	"testing"
	"time"

	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/upstream"

	"gorm.io/gorm"
)

func queuedAnchors(t *testing.T, db *gorm.DB) map[anchorRef]bool {
	t.Helper()
	var rows []presentationClaim
	if err := db.Raw(`SELECT anchor_kind, anchor_id FROM anchor_presentation_queue`).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	out := map[anchorRef]bool{}
	for _, r := range rows {
		out[r.anchorRef()] = true
	}
	return out
}

func drainPresentations(t *testing.T, p *Pusher) {
	t.Helper()
	for range 10 {
		n, err := p.RunPresentationsOnce(context.Background())
		if err != nil {
			t.Fatalf("RunPresentationsOnce: %v", err)
		}
		if n == 0 {
			return
		}
	}
	t.Fatal("presentation queue never emptied")
}

func byAnchor(items []communityclient.AnchorPresentation) map[anchorRef]communityclient.AnchorPresentation {
	out := map[anchorRef]communityclient.AnchorPresentation{}
	for _, it := range items {
		out[anchorRef{Kind: it.AnchorKind, ID: it.AnchorID}] = it
	}
	return out
}

func wallPost(anchorKind int32, anchorID string) communityclient.AuthorPostView {
	return communityclient.AuthorPostView{Thread: communityclient.PostThreadContext{AnchorKind: anchorKind, AnchorID: anchorID}}
}

func TestAPatchAndItsResourcePresentTheirWalls(t *testing.T) {
	db := testDB(t)
	addPatch(t, db, 990030)
	rid := addResource(t, db, 990030, "汉化补丁")
	game, res := gameAnchor(990030), resourceAnchor(rid)
	if q := queuedAnchors(t, db); !q[game] || !q[res] {
		t.Fatalf("queue after insert = %v", q)
	}

	com := &fakeCommunity{}
	before := time.Now().UnixMicro()
	drainPresentations(t, newTestPusher(db, com, &fakeCatalog{}))
	after := time.Now().UnixMicro()

	got := byAnchor(com.sentPresentations())
	if len(got) != 2 {
		t.Fatalf("pushed %+v", got)
	}
	g := got[game]
	if g.Title != "千恋＊万花" || g.URL != "https://www.moyu.moe/galgame/990030?tab=comment" ||
		g.WorkID == nil || *g.WorkID != 990030 || g.ContentLimit != "sfw" || g.CoverImageHash == "" || g.Removed {
		t.Errorf("game wall = %+v", g)
	}
	if g.Revision < before || g.Revision > after {
		t.Errorf("revision %d outside [%d, %d]", g.Revision, before, after)
	}
	r := got[res]
	if r.Title != "千恋＊万花 · 汉化补丁" || r.URL != "https://www.moyu.moe/resource/"+itoa(rid) ||
		r.WorkID == nil || *r.WorkID != 990030 || r.ContentLimit != "sfw" || r.Removed {
		t.Errorf("resource wall = %+v", r)
	}
	if q := queuedAnchors(t, db); len(q) != 0 {
		t.Errorf("queue = %v", q)
	}
	sent, _ := store{db: db}.presentationsSent(context.Background(), []anchorRef{game, res})
	if s := sent[res]; s.Revision != r.Revision || s.Removed {
		t.Errorf("ledger = %+v", sent)
	}
}

// Agreed with infra for D5: a disabled resource's page still shows its wall and
// takes comments, so only a hidden one (status 2) takes the wall out of the feed.
func TestADisabledResourceKeepsItsWallAndAHiddenOneTombstonesIt(t *testing.T) {
	db := testDB(t)
	addPatch(t, db, 990031)
	rid := addResource(t, db, 990031, "x")
	com := &fakeCommunity{}
	p := newTestPusher(db, com, &fakeCatalog{})
	drainPresentations(t, p)

	if err := db.Exec(`UPDATE patch_resource SET status = 1 WHERE id = ?`, rid).Error; err != nil {
		t.Fatal(err)
	}
	drainPresentations(t, p)
	items := com.sentPresentations()
	if last := items[len(items)-1]; last.AnchorID != itoa(rid) || last.Removed {
		t.Fatalf("disabling pushed %+v", last)
	}

	if err := db.Exec(`UPDATE patch_resource SET status = 2 WHERE id = ?`, rid).Error; err != nil {
		t.Fatal(err)
	}
	drainPresentations(t, p)
	items = com.sentPresentations()
	tomb := items[len(items)-1]
	if !tomb.Removed || tomb.AnchorKind != communityclient.AnchorSiteResource || tomb.AnchorID != itoa(rid) ||
		tomb.Title != "" || tomb.URL != "" || tomb.WorkID != nil || tomb.ContentLimit != "" || tomb.CoverImageHash != "" ||
		tomb.Revision <= items[len(items)-2].Revision {
		t.Errorf("tombstone = %+v", tomb)
	}
}

func TestAWallCommunityNeverAcceptedGetsNoTombstone(t *testing.T) {
	db := testDB(t)
	addPatch(t, db, 990032)
	rid := addResource(t, db, 990032, "x")
	if err := db.Exec(`UPDATE patch_resource SET status = 2 WHERE id = ?`, rid).Error; err != nil {
		t.Fatal(err)
	}
	com := &fakeCommunity{}
	drainPresentations(t, newTestPusher(db, com, &fakeCatalog{}))
	for _, it := range com.sentPresentations() {
		if it.AnchorID == itoa(rid) {
			t.Errorf("a never-presented hidden resource sent %+v", it)
		}
	}
}

func TestACatalogFailurePresentsNothingAndKeepsTheQueue(t *testing.T) {
	db := testDB(t)
	addPatch(t, db, 990033)
	com := &fakeCommunity{}
	cat := &fakeCatalog{err: &upstream.Error{Service: "catalog", Kind: upstream.Unavailable, Status: 503}}
	p := newTestPusher(db, com, cat)
	if _, err := p.RunPresentationsOnce(context.Background()); err == nil {
		t.Fatal("RunPresentationsOnce swallowed the catalog failure")
	}
	if len(com.sentPresentations()) != 0 || !queuedAnchors(t, db)[gameAnchor(990033)] {
		t.Fatalf("pushed %+v, queue %v", com.sentPresentations(), queuedAnchors(t, db))
	}
}

func TestAWorkCatalogHidesTombstonesEveryWallOnIt(t *testing.T) {
	db := testDB(t)
	addPatch(t, db, 990034)
	rid := addResource(t, db, 990034, "x")
	com := &fakeCommunity{}
	cat := &fakeCatalog{}
	p := newTestPusher(db, com, cat)
	drainPresentations(t, p)

	cat.gone = map[int]bool{990034: true}
	if err := db.Exec(`UPDATE patch SET published = NOT published WHERE id = 990034`).Error; err != nil {
		t.Fatal(err)
	}
	if q := queuedAnchors(t, db); !q[gameAnchor(990034)] || !q[resourceAnchor(rid)] {
		t.Fatalf("unpublishing the page enqueued %v", q)
	}
	drainPresentations(t, p)
	got := byAnchor(com.sentPresentations()[2:])
	if len(got) != 2 || !got[gameAnchor(990034)].Removed || !got[resourceAnchor(rid)].Removed {
		t.Errorf("pushed %+v", got)
	}
}

// A game page renders for any catalog work, and commenting on one does not
// create its patch row: production had two such walls when D5 shipped.
func TestTheWallSweepPresentsAGamePageWithNoPatchRow(t *testing.T) {
	db := testDB(t)
	addPatch(t, db, 990035)
	com := &fakeCommunity{walls: []communityclient.AuthorPostView{
		wallPost(communityclient.AnchorSiteGame, "990035"),
		wallPost(communityclient.AnchorSiteGame, "990099"),
		wallPost(communityclient.AnchorSiteGame, "990099"),
	}}
	p := newTestPusher(db, com, &fakeCatalog{})
	drainPresentations(t, p)

	n, err := p.PresentWalls(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if q := queuedAnchors(t, db); n != 1 || len(q) != 1 || !q[gameAnchor(990099)] {
		t.Fatalf("enqueued %d, queue %v", n, q)
	}
	drainPresentations(t, p)
	got := byAnchor(com.sentPresentations())
	if w := got[gameAnchor(990099)]; w.Title != "千恋＊万花" || w.URL != "https://www.moyu.moe/galgame/990099?tab=comment" || w.Removed {
		t.Errorf("patch-less wall = %+v", w)
	}

	// The comment hook is a no-op for a wall already presented.
	if err := EnsurePresented(context.Background(), db, communityclient.AnchorSiteGame, "990099"); err != nil {
		t.Fatal(err)
	}
	if q := queuedAnchors(t, db); len(q) != 0 {
		t.Errorf("an already presented wall was queued again: %v", q)
	}
	if err := EnsurePresented(context.Background(), db, communityclient.AnchorSiteGame, "990098"); err != nil {
		t.Fatal(err)
	}
	if q := queuedAnchors(t, db); !q[gameAnchor(990098)] {
		t.Errorf("a new wall was not queued: %v", q)
	}
}

func TestReconcileRepairsPresentationDriftAndTombstonesWhatIsGone(t *testing.T) {
	db := testDB(t)
	addPatch(t, db, 990036)
	rid := addResource(t, db, 990036, "x")
	com := &fakeCommunity{walls: []communityclient.AuthorPostView{wallPost(communityclient.AnchorSiteGame, "990097")}}
	p := newTestPusher(db, com, &fakeCatalog{})
	drainPresentations(t, p)
	pushed := byAnchor(com.sentPresentations())

	stored := func(a anchorRef, title string) communityclient.StoredAnchorPresentation {
		it := pushed[a]
		return communityclient.StoredAnchorPresentation{
			AnchorKind: it.AnchorKind, AnchorID: it.AnchorID, Title: title, URL: it.URL,
			CoverImageHash: &it.CoverImageHash, WorkID: it.WorkID, ContentLimit: it.ContentLimit, Revision: it.Revision,
		}
	}
	com.storedPresentations = []communityclient.StoredAnchorPresentation{
		stored(gameAnchor(990036), "an old catalog name"),
		stored(resourceAnchor(rid), pushed[resourceAnchor(rid)].Title),
		// A resource wall community holds live that this site has no row for.
		{AnchorKind: communityclient.AnchorSiteResource, AnchorID: "989999", Title: "t", URL: "u", ContentLimit: "sfw", Revision: 5},
		// Not an anchor moyu mints: left alone.
		{AnchorKind: communityclient.AnchorCatalogWork, AnchorID: "42", Title: "t", Revision: 5},
	}

	report, err := p.ReconcilePresentations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	q := queuedAnchors(t, db)
	orphan := anchorRef{Kind: communityclient.AnchorSiteResource, ID: "989999"}
	if report.Enqueued != 3 || len(q) != 3 || !q[gameAnchor(990036)] || !q[orphan] || !q[gameAnchor(990097)] {
		t.Fatalf("report %+v, queue %v", report, q)
	}

	drainPresentations(t, p)
	got := byAnchor(com.sentPresentations()[len(pushed):])
	if fix := got[gameAnchor(990036)]; fix.Title != "千恋＊万花" || fix.Removed {
		t.Errorf("repair = %+v", fix)
	}
	if tomb := got[orphan]; !tomb.Removed {
		t.Errorf("orphan = %+v", tomb)
	}
	if w := got[gameAnchor(990097)]; w.Removed || w.Title == "" {
		t.Errorf("swept wall = %+v", w)
	}
}

func TestAFailedWallSweepFailsTheReconcile(t *testing.T) {
	db := testDB(t)
	addPatch(t, db, 990037)
	com := &fakeCommunity{wallsErr: &upstream.Error{Service: "community", Kind: upstream.Unavailable, Status: 503}}
	p := newTestPusher(db, com, &fakeCatalog{})
	drainPresentations(t, p)
	if _, err := p.ReconcilePresentations(context.Background()); err == nil {
		t.Fatal("the reconcile ran on without the wall sweep")
	}
	if _, err := p.PresentWalls(context.Background()); err == nil {
		t.Fatal("PresentWalls swallowed the sweep failure")
	}
}
