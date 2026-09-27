package activitypush

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/pkg/communityclient"
)

// Community projects the comments on moyu's walls into the feed itself (key
// `community:post:<id>`), but it knows a wall only by its anchor. An anchor
// presentation tells it which page that is: migration 045 enqueues an anchor
// whenever its page may have changed, and the drainer pushes what the page is
// now, exactly as it does for activities.

const gameWallTitle = "Galgame"

type anchorRef struct {
	Kind int32
	ID   string
}

func gameAnchor(patchID int) anchorRef {
	return anchorRef{Kind: communityclient.AnchorSiteGame, ID: strconv.Itoa(patchID)}
}

func resourceAnchor(resourceID int) anchorRef {
	return anchorRef{Kind: communityclient.AnchorSiteResource, ID: strconv.Itoa(resourceID)}
}

func (a anchorRef) number() (int, bool) {
	if a.Kind != communityclient.AnchorSiteGame && a.Kind != communityclient.AnchorSiteResource {
		return 0, false
	}
	n, err := strconv.Atoi(a.ID)
	return n, err == nil && n > 0
}

// presentations reads what each wall's page is now: the presentation to push,
// or nil when the page is not public. An anchor that is not moyu's is left out.
//
// Public is what the page itself shows. A game page renders for any work
// catalog renders, with or without a patch row. A resource page 404s only at
// status 2: a disabled one (1) still shows its wall and takes comments, so its
// presentation stays although its publish activity is tombstoned.
func (r *resolver) presentations(ctx context.Context, refs []anchorRef) (map[anchorRef]*communityclient.AnchorPresentation, error) {
	var gameIDs, resourceIDs []int
	for _, a := range refs {
		n, ok := a.number()
		switch {
		case !ok:
		case a.Kind == communityclient.AnchorSiteGame:
			gameIDs = append(gameIDs, n)
		default:
			resourceIDs = append(resourceIDs, n)
		}
	}

	resources, err := r.resources(ctx, resourceIDs)
	if err != nil {
		return nil, err
	}
	workIDs := gameIDs
	for _, res := range resources {
		if res.Status != 2 {
			workIDs = append(workIDs, res.GalgameID)
		}
	}
	works, err := r.worksByID(ctx, workIDs)
	if err != nil {
		return nil, err
	}

	out := make(map[anchorRef]*communityclient.AnchorPresentation, len(refs))
	for _, a := range refs {
		n, ok := a.number()
		if !ok {
			continue
		}
		out[a] = nil
		if a.Kind == communityclient.AnchorSiteGame {
			if work, ok := works[n]; ok {
				out[a] = r.gamePresentation(a, n, work)
			}
			continue
		}
		res, ok := resources[n]
		if !ok || res.Status == 2 {
			continue
		}
		if work, ok := works[res.GalgameID]; ok {
			workID := int64(res.GalgameID)
			out[a] = &communityclient.AnchorPresentation{
				AnchorKind: a.Kind, AnchorID: a.ID,
				Title:          resourceTitle(res, work),
				URL:            r.origin + "/resource/" + a.ID,
				CoverImageHash: coverHash(work.EffectiveBannerHash),
				WorkID:         &workID,
				ContentLimit:   contentLimit(work),
			}
		}
	}
	return out, nil
}

func (r *resolver) gamePresentation(a anchorRef, patchID int, work *galgameClient.GalgameBrief) *communityclient.AnchorPresentation {
	title := cutRunes(singleLine(work.PreferredName()), titleRunes)
	if title == "" {
		title = gameWallTitle
	}
	workID := int64(patchID)
	return &communityclient.AnchorPresentation{
		AnchorKind: a.Kind, AnchorID: a.ID,
		Title:          title,
		URL:            r.origin + "/galgame/" + a.ID + "?tab=comment",
		CoverImageHash: coverHash(work.EffectiveBannerHash),
		WorkID:         &workID,
		ContentLimit:   contentLimit(work),
	}
}

// RunPresentationsOnce is RunOnce for anchor presentations, under the same
// revision rule: stamped before the state is read.
func (p *Pusher) RunPresentationsOnce(ctx context.Context) (int, error) {
	claims, err := p.store.claimPresentations(ctx, claimLimit)
	if err != nil || len(claims) == 0 {
		return 0, err
	}
	rev := p.now().UnixMicro()

	refs := make([]anchorRef, len(claims))
	for i, c := range claims {
		refs[i] = c.anchorRef()
	}
	current, err := p.resolver.presentations(ctx, refs)
	if err != nil {
		return 0, fmt.Errorf("read anchor presentations: %w", err)
	}
	sent, err := p.store.presentationsSent(ctx, refs)
	if err != nil {
		return 0, err
	}

	items := make([]communityclient.AnchorPresentation, 0, len(claims))
	for _, a := range refs {
		live, known := current[a]
		switch {
		case !known:
			slog.Warn("anchor presentation push: dropping an unrecognised anchor", "anchor_kind", a.Kind, "anchor_id", a.ID)
		case live != nil:
			item := *live
			item.Revision = rev
			items = append(items, item)
		default:
			// Never tombstone an anchor community has not accepted: the
			// contract forbids it for presentations as for activities.
			if prev, ok := sent[a]; ok && !prev.Removed {
				items = append(items, communityclient.AnchorPresentation{
					AnchorKind: a.Kind, AnchorID: a.ID, Revision: rev, Removed: true,
				})
			}
		}
	}

	accepted, err := p.sendPresentations(ctx, items)
	if err != nil {
		return 0, err
	}
	if err := p.store.recordPresentations(ctx, accepted); err != nil {
		return 0, err
	}
	if err := p.store.ackPresentations(ctx, claims); err != nil {
		return 0, err
	}
	return len(claims), nil
}

func (p *Pusher) sendPresentations(ctx context.Context, items []communityclient.AnchorPresentation) ([]presentationSent, error) {
	var accepted []presentationSent
	err := writeHalving(items, func(batch []communityclient.AnchorPresentation) error {
		res, err := p.community.WriteAnchorPresentations(ctx, batch)
		if err != nil {
			return err
		}
		if len(res.Results) != len(batch) {
			return fmt.Errorf("anchor presentation push: %d outcomes for %d items", len(res.Results), len(batch))
		}
		for i, r := range res.Results {
			it := batch[i]
			if r.AnchorKind != it.AnchorKind || r.AnchorID != it.AnchorID {
				return fmt.Errorf("anchor presentation push: outcome %d is for %d:%s, sent %d:%s",
					i, r.AnchorKind, r.AnchorID, it.AnchorKind, it.AnchorID)
			}
			if acceptedOutcome(r.Outcome, r.Reason, "anchor_kind", it.AnchorKind, "anchor_id", it.AnchorID) {
				accepted = append(accepted, presentationSent{
					AnchorKind: it.AnchorKind, AnchorID: it.AnchorID, Revision: it.Revision, Removed: it.Removed,
				})
			}
		}
		return nil
	}, func(it communityclient.AnchorPresentation, err error) {
		slog.Error("anchor presentation push: community rejected an item",
			"anchor_kind", it.AnchorKind, "anchor_id", it.AnchorID, "error", err)
	})
	return accepted, err
}
