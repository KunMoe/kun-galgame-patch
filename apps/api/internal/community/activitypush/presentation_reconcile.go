package activitypush

import (
	"context"
	"fmt"
	"slices"

	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/communityclient"

	"gorm.io/gorm"
)

const wallSweepPage = 100

// ReconcilePresentations is Reconcile for anchor presentations. A catalog
// rename or a new rating changes a page without any local row changing, and
// this is the only thing that notices.
//
// Every stored anchor is checked as well as every local one, so an anchor whose
// row is gone is tombstoned here without a separate orphan pass.
func (p *Pusher) ReconcilePresentations(ctx context.Context) (ReconcileReport, error) {
	var report ReconcileReport
	stored, err := p.loadStoredPresentations(ctx)
	if err != nil {
		return report, err
	}
	report.Stored = len(stored)

	ledger := make([]presentationSent, 0, len(stored))
	for a, have := range stored {
		ledger = append(ledger, presentationSent{AnchorKind: a.Kind, AnchorID: a.ID, Revision: have.Revision, Removed: have.Removed})
	}
	if err := p.store.recordPresentations(ctx, ledger); err != nil {
		return report, err
	}

	local, err := p.localAnchors(ctx)
	if err != nil {
		return report, err
	}
	report.Local = len(local)
	for a := range stored {
		local = append(local, a)
	}
	refs := uniqueAnchors(local)

	for page := range slices.Chunk(refs, localPage) {
		current, err := p.resolver.presentations(ctx, page)
		if err != nil {
			return report, fmt.Errorf("read anchor presentations: %w", err)
		}
		var drift []anchorRef
		for _, a := range page {
			have, ok := stored[a]
			want := current[a]
			if want != nil && (!ok || have.Removed || presentationDiffers(want, have)) ||
				want == nil && ok && !have.Removed {
				drift = append(drift, a)
			}
		}
		if err := p.store.enqueuePresentations(ctx, drift); err != nil {
			return report, err
		}
		report.Enqueued += len(drift)
	}
	return report, nil
}

// PresentWalls queues every game wall community holds that has no live
// presentation yet. It is what gets the walls of patch-less pages presented
// before the first daily reconcile.
func (p *Pusher) PresentWalls(ctx context.Context) (int, error) {
	walls, err := p.gameWalls(ctx)
	if err != nil {
		return 0, err
	}
	return p.store.enqueueUnpresented(ctx, walls)
}

// EnsurePresented queues a wall that just took a comment unless community
// already holds a live presentation for it.
func EnsurePresented(ctx context.Context, db *gorm.DB, kind int32, anchorID string) error {
	_, err := store{db: db}.enqueueUnpresented(ctx, []anchorRef{{Kind: kind, ID: anchorID}})
	return err
}

func (p *Pusher) loadStoredPresentations(ctx context.Context) (map[anchorRef]communityclient.StoredAnchorPresentation, error) {
	out := map[anchorRef]communityclient.StoredAnchorPresentation{}
	cursor := ""
	for {
		page, err := p.community.ListAnchorPresentations(ctx, cursor, storedPage)
		if err != nil {
			return nil, err
		}
		for _, s := range page.Presentations {
			a := anchorRef{Kind: s.AnchorKind, ID: s.AnchorID}
			if _, ours := a.number(); ours {
				out[a] = s
			}
		}
		if page.NextCursor == "" || page.NextCursor == cursor || len(page.Presentations) == 0 {
			return out, nil
		}
		cursor = page.NextCursor
	}
}

// localAnchors is every wall this site could present: each game page with a
// patch row, each resource, and each game wall community holds. The last is
// not covered by the first: a game page renders for any catalog work, and a
// comment on one does not create its patch row.
func (p *Pusher) localAnchors(ctx context.Context) ([]anchorRef, error) {
	db := p.store.db.WithContext(ctx)
	var patchIDs, resourceIDs []int
	if err := db.Model(&patchModel.Patch{}).Order("id").Pluck("id", &patchIDs).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&patchModel.PatchResource{}).Order("id").Pluck("id", &resourceIDs).Error; err != nil {
		return nil, err
	}
	walls, err := p.gameWalls(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]anchorRef, 0, len(patchIDs)+len(resourceIDs)+len(walls))
	for _, id := range patchIDs {
		out = append(out, gameAnchor(id))
	}
	for _, id := range resourceIDs {
		out = append(out, resourceAnchor(id))
	}
	return append(out, walls...), nil
}

// gameWalls sweeps every comment on this site's game walls for their anchors.
// The sweep failing fails the caller: an outage is not an empty site.
func (p *Pusher) gameWalls(ctx context.Context) ([]anchorRef, error) {
	seen := map[anchorRef]bool{}
	var out []anchorRef
	cursor := ""
	for {
		res, err := p.community.ListSitePosts(ctx, communityclient.SitePostsQuery{
			Kind:       communityclient.KindComments,
			AnchorKind: communityclient.AnchorSiteGame,
			Cursor:     cursor,
			Limit:      wallSweepPage,
		})
		if err != nil {
			return nil, fmt.Errorf("game wall sweep: %w", err)
		}
		for _, row := range res.Posts {
			a := anchorRef{Kind: row.Thread.AnchorKind, ID: row.Thread.AnchorID}
			if _, ok := a.number(); ok && a.Kind == communityclient.AnchorSiteGame && !seen[a] {
				seen[a] = true
				out = append(out, a)
			}
		}
		if res.NextCursor == "" {
			return out, nil
		}
		if res.NextCursor == cursor {
			return nil, fmt.Errorf("game wall sweep: cursor %q did not advance", cursor)
		}
		cursor = res.NextCursor
	}
}

func uniqueAnchors(refs []anchorRef) []anchorRef {
	rows := presentationRows(refs)
	out := make([]anchorRef, len(rows))
	for i, r := range rows {
		out[i] = anchorRef{Kind: r.AnchorKind, ID: r.AnchorID}
	}
	return out
}

func presentationDiffers(want *communityclient.AnchorPresentation, have communityclient.StoredAnchorPresentation) bool {
	return want.Title != have.Title ||
		want.URL != have.URL ||
		want.ContentLimit != have.ContentLimit ||
		want.CoverImageHash != deref(have.CoverImageHash) ||
		!sameWork(want.WorkID, have.WorkID)
}
