package activitypush

import (
	"context"
	"fmt"
	"slices"
	"time"

	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/communityclient"
)

const (
	storedPage = 1000
	localPage  = 100
)

type ReconcileReport struct {
	Stored   int
	Local    int
	Enqueued int
}

// Reconcile compares every field community stores with what this site would
// push now, and enqueues each key that differs; the drainer does the pushing.
// A field derived from catalog — the title, the cover, the content limit —
// changes without any local row changing, and this is the only thing that
// notices.
//
// It first copies community's listing into activity_push_sent. That ledger is
// what lets the drainer tombstone a key, and a crash between a push and its
// record would otherwise leave a live item nothing can ever remove.
func (p *Pusher) Reconcile(ctx context.Context) (ReconcileReport, error) {
	var report ReconcileReport
	stored, err := p.loadStored(ctx)
	if err != nil {
		return report, err
	}
	report.Stored = len(stored)

	ledger := make([]sentRow, 0, len(stored))
	for _, a := range stored {
		ledger = append(ledger, sentRow{Key: a.Key, ActorID: int(a.ActorID), Revision: a.Revision, Removed: a.Removed})
	}
	if err := p.store.record(ctx, ledger); err != nil {
		return report, err
	}

	keys, err := p.localKeys(ctx)
	if err != nil {
		return report, err
	}
	report.Local = len(keys)
	local := make(map[string]bool, len(keys))

	for page := range slices.Chunk(keys, localPage) {
		current, err := p.resolver.resolve(ctx, page)
		if err != nil {
			return report, fmt.Errorf("read activity state: %w", err)
		}
		var drift []string
		for _, k := range page {
			local[k] = true
			have, ok := stored[k]
			want := current[k]
			if want != nil && (!ok || have.Removed || differs(want, have)) ||
				want == nil && ok && !have.Removed {
				drift = append(drift, k)
			}
		}
		if err := p.store.enqueue(ctx, drift); err != nil {
			return report, err
		}
		report.Enqueued += len(drift)
	}

	var orphans []string
	for k, have := range stored {
		if !have.Removed && !local[k] {
			orphans = append(orphans, k)
		}
	}
	if err := p.store.enqueue(ctx, orphans); err != nil {
		return report, err
	}
	report.Enqueued += len(orphans)
	return report, nil
}

func (p *Pusher) loadStored(ctx context.Context) (map[string]communityclient.SiteActivity, error) {
	out := map[string]communityclient.SiteActivity{}
	cursor := ""
	for {
		page, err := p.community.ListSiteActivities(ctx, cursor, storedPage)
		if err != nil {
			return nil, err
		}
		for _, a := range page.Activities {
			out[a.Key] = a
		}
		if page.NextCursor == "" || page.NextCursor == cursor || len(page.Activities) == 0 {
			return out, nil
		}
		cursor = page.NextCursor
	}
}

// localKeys is every key this site could push, public now or not.
func (p *Pusher) localKeys(ctx context.Context) ([]string, error) {
	db := p.store.db.WithContext(ctx)
	var resourceIDs []int
	if err := db.Model(&patchModel.PatchResource{}).Order("id").Pluck("id", &resourceIDs).Error; err != nil {
		return nil, err
	}
	var revs []revisionRow
	if err := db.Model(&patchModel.PatchResourceRevision{}).
		Select("resource_id", "actor_id", "created_at").
		Where("actor_id > 0 AND action = ?", "updated").
		Find(&revs).Error; err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(resourceIDs)+len(revs))
	for _, id := range resourceIDs {
		keys = append(keys, resourceKey(id))
	}
	edits := make([]string, 0, len(revs))
	for _, r := range revs {
		edits = append(edits, editKey(r.ResourceID, r.ActorID, r.CreatedAt))
	}
	return append(keys, slices.Compact(slices.Sorted(slices.Values(edits)))...), nil
}

// differs compares every field a push writes. Notify and revision are not
// compared: neither is content, and a notify difference is expected on every
// key that was ever pushed live.
func differs(want *communityclient.ActivityItem, have communityclient.SiteActivity) bool {
	return want.ActorID != have.ActorID ||
		want.Verb != have.Verb ||
		want.ObjectKind != have.ObjectKind ||
		want.ObjectLabel != have.ObjectLabel ||
		want.Title != have.Title ||
		want.Excerpt != have.Excerpt ||
		want.URL != have.URL ||
		want.ContentLimit != have.ContentLimit ||
		want.CoverImageHash != deref(have.CoverImageHash) ||
		!sameWork(want.WorkID, have.WorkID) ||
		!sameInstant(want.OccurredAt, have.OccurredAt)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func sameWork(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func sameInstant(want *time.Time, have time.Time) bool {
	return want != nil && want.Truncate(time.Microsecond).Equal(have.Truncate(time.Microsecond))
}
