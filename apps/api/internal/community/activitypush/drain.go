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
	WriteAnchorPresentations(ctx context.Context, items []communityclient.AnchorPresentation) (*communityclient.AnchorPresentationWriteResponse, error)
	ListAnchorPresentations(ctx context.Context, cursor string, limit int) (*communityclient.AnchorPresentationPage, error)
	ListSitePosts(ctx context.Context, q communityclient.SitePostsQuery) (*communityclient.PostFeedResponse, error)
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

// send writes items and returns the ones community accepted.
func (p *Pusher) send(ctx context.Context, items []communityclient.ActivityItem) ([]sentRow, error) {
	var accepted []sentRow
	err := writeHalving(items, func(batch []communityclient.ActivityItem) error {
		res, err := p.community.WriteActivities(ctx, batch)
		if err != nil {
			return err
		}
		if len(res.Results) != len(batch) {
			return fmt.Errorf("activity push: %d outcomes for %d items", len(res.Results), len(batch))
		}
		for i, r := range res.Results {
			it := batch[i]
			if r.Key != it.Key {
				return fmt.Errorf("activity push: outcome %d is for %q, sent %q", i, r.Key, it.Key)
			}
			if acceptedOutcome(r.Outcome, r.Reason, "key", it.Key) {
				accepted = append(accepted, sentRow{Key: it.Key, ActorID: int(it.ActorID), Revision: it.Revision, Removed: it.Removed})
			}
		}
		return nil
	}, func(it communityclient.ActivityItem, err error) {
		slog.Error("activity push: community rejected an item", "key", it.Key, "error", err)
	})
	return accepted, err
}

// writeHalving writes items in one call. A 422 fails the whole batch for one
// malformed item, so the batch is halved until that item stands alone, and it
// alone is dropped; the daily reconcile finds it missing and offers it again.
func writeHalving[T any](items []T, write func([]T) error, reject func(T, error)) error {
	if len(items) == 0 {
		return nil
	}
	err := write(items)
	if e, ok := upstream.As(err); !ok || e.Status != http.StatusUnprocessableEntity {
		return err
	}
	if len(items) == 1 {
		reject(items[0], err)
		return nil
	}
	half := len(items) / 2
	if err := writeHalving(items[:half], write, reject); err != nil {
		return err
	}
	return writeHalving(items[half:], write, reject)
}

func acceptedOutcome(outcome, reason string, id ...any) bool {
	switch outcome {
	case communityclient.ActivityCreated, communityclient.ActivityUpdated,
		communityclient.ActivityRemoved, communityclient.ActivityRestored:
		return true
	case communityclient.ActivityStale:
	case communityclient.ActivityInvalid:
		slog.Warn("activity push: community found an item invalid", append(id, "reason", reason)...)
	default:
		slog.Warn("activity push: unknown outcome", append(id, "outcome", outcome)...)
	}
	return false
}
