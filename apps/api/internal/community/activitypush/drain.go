package activitypush

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/upstream"

	"gorm.io/gorm"
)

const claimLimit = 100

type Community interface {
	WriteActivities(ctx context.Context, items []communityclient.ActivityItem) (*communityclient.ActivityWriteResponse, error)
	ListSiteActivities(ctx context.Context, cursor string, limit int) (*communityclient.SiteActivityPage, error)
}

type Pusher struct {
	store     store
	resolver  resolver
	community Community
	now       func() time.Time
}

// New builds a pusher that links to origin (scheme://host, no trailing slash).
func New(db *gorm.DB, community Community, catalog Catalog, origin string) *Pusher {
	return &Pusher{
		store:     store{db: db},
		resolver:  resolver{db: db, catalog: catalog, origin: origin},
		community: community,
		now:       time.Now,
	}
}

// RunOnce pushes one claimed batch and reports how many queue rows it handled.
// An error leaves every claimed row in the queue.
//
// revision is stamped BEFORE the state is read: a change committed after the
// read enqueues the key again and is read by a later tick under a later stamp,
// so a newer state always carries the greater revision. A row's own update time
// cannot serve — hiding a resource does not always touch the row that is
// pushed, and a tombstone carrying the stored revision is refused as stale.
func (p *Pusher) RunOnce(ctx context.Context) (int, error) {
	claims, err := p.store.claim(ctx, claimLimit)
	if err != nil || len(claims) == 0 {
		return 0, err
	}
	rev := p.now().UnixMicro()

	keys := make([]string, len(claims))
	for i, c := range claims {
		keys[i] = c.Key
	}
	current, err := p.resolver.resolve(ctx, keys)
	if err != nil {
		return 0, fmt.Errorf("read activity state: %w", err)
	}
	sent, err := p.store.sent(ctx, keys)
	if err != nil {
		return 0, err
	}

	items := make([]communityclient.ActivityItem, 0, len(claims))
	for _, c := range claims {
		live, known := current[c.Key]
		switch {
		case !known:
			slog.Warn("activity push: dropping an unrecognised queue key", "key", c.Key)
		case live != nil:
			item := *live
			item.Revision = rev
			_, pushedBefore := sent[c.Key]
			item.Notify = item.Verb == verbPublish && !pushedBefore && !c.Backfill
			items = append(items, item)
		default:
			// Only a key community holds live gets a tombstone. One it never
			// accepted would be stored as "seen", and restoring the content later
			// would then never notify.
			if prev, ok := sent[c.Key]; ok && !prev.Removed {
				items = append(items, communityclient.ActivityItem{
					Key: c.Key, ActorID: int64(prev.ActorID), Revision: rev, Removed: true,
				})
			}
		}
	}

	accepted, err := p.send(ctx, items)
	if err != nil {
		return 0, err
	}
	if err := p.store.record(ctx, accepted); err != nil {
		return 0, err
	}
	if err := p.store.ack(ctx, claims); err != nil {
		return 0, err
	}
	return len(claims), nil
}

// send writes items and returns the ones community accepted. A 422 fails the
// whole batch for one malformed item, so the batch is halved until that item
// stands alone, and it alone is dropped; the daily reconcile finds it missing
// and offers it again.
func (p *Pusher) send(ctx context.Context, items []communityclient.ActivityItem) ([]sentRow, error) {
	if len(items) == 0 {
		return nil, nil
	}
	res, err := p.community.WriteActivities(ctx, items)
	if err != nil {
		if e, ok := upstream.As(err); !ok || e.Status != http.StatusUnprocessableEntity {
			return nil, err
		}
		if len(items) == 1 {
			slog.Error("activity push: community rejected an item", "key", items[0].Key, "error", err)
			return nil, nil
		}
		half := len(items) / 2
		left, err := p.send(ctx, items[:half])
		if err != nil {
			return nil, err
		}
		right, err := p.send(ctx, items[half:])
		return append(left, right...), err
	}
	if len(res.Results) != len(items) {
		return nil, fmt.Errorf("activity push: %d outcomes for %d items", len(res.Results), len(items))
	}

	accepted := make([]sentRow, 0, len(items))
	for i, r := range res.Results {
		it := items[i]
		if r.Key != it.Key {
			return nil, fmt.Errorf("activity push: outcome %d is for %q, sent %q", i, r.Key, it.Key)
		}
		switch r.Outcome {
		case communityclient.ActivityCreated, communityclient.ActivityUpdated,
			communityclient.ActivityRemoved, communityclient.ActivityRestored:
			accepted = append(accepted, sentRow{Key: it.Key, ActorID: int(it.ActorID), Revision: it.Revision, Removed: it.Removed})
		case communityclient.ActivityStale:
		case communityclient.ActivityInvalid:
			slog.Warn("activity push: community found an item invalid", "key", it.Key, "reason", r.Reason)
		default:
			slog.Warn("activity push: unknown outcome", "key", it.Key, "outcome", r.Outcome)
		}
	}
	return accepted, nil
}
