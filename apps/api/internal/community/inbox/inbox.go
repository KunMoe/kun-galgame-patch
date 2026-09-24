package inbox

import (
	"context"
	"log/slog"
	"time"

	"kun-galgame-patch-api/internal/community/anchor"
	"kun-galgame-patch-api/pkg/communityclient"

	"gorm.io/gorm"
)

const (
	feedCronName = "community_notification_feed"
	feedPageSize = 500
	feedMaxPages = 20
	readChunk    = 100
	mentionBatch = 100
)

type Inbox struct {
	community *communityclient.Client
	anchors   *anchor.Resolver
	db        *gorm.DB
}

func New(community *communityclient.Client, anchors *anchor.Resolver, db *gorm.DB) *Inbox {
	return &Inbox{community: community, anchors: anchors, db: db}
}

func (in *Inbox) Configured() bool {
	return in != nil && in.community != nil && in.community.Configured()
}

func (in *Inbox) Sync(ctx context.Context) (int, error) {
	if in == nil || !in.Configured() {
		return 0, nil
	}
	after, err := readFeedCursor(in.db)
	if err != nil {
		return 0, err
	}
	written := 0
	for page := 0; page < feedMaxPages; page++ {
		feed, err := in.community.NotificationFeed(ctx, after, feedPageSize)
		if err != nil {
			return written, err
		}
		if len(feed.Notifications) == 0 {
			break
		}
		n, err := in.applyPage(ctx, feed.Notifications, feed.NextAfter)
		if err != nil {
			return written, err
		}
		written += n
		if feed.NextAfter <= after || len(feed.Notifications) < feedPageSize {
			break
		}
		after = feed.NextAfter
	}
	return written, nil
}

func (in *Inbox) MarkThreadRead(userID int, threadID int64, upTo int32) {
	if in == nil || in.db == nil || threadID <= 0 {
		return
	}
	if err := in.db.Exec(`
		UPDATE user_message
		SET status = 1, updated = NOW()
		WHERE recipient_id = ?
		  AND community_thread_id = ?
		  AND status = 0
		  AND type IN ('comment','mention','commentWatch')
		  AND community_post_number <= ?
	`, userID, threadID, upTo).Error; err != nil {
		slog.Warn("community inbox: mark thread read failed",
			"user_id", userID, "thread_id", threadID, "error", err)
	}
}

func (in *Inbox) ForwardRead(ctx context.Context, userID int, ids []int64) {
	if in == nil || !in.Configured() || len(ids) == 0 {
		return
	}
	for start := 0; start < len(ids); start += readChunk {
		end := min(start+readChunk, len(ids))
		if _, err := in.community.MarkNotificationsRead(ctx, int64(userID), ids[start:end]); err != nil {
			communityclient.LogDegraded(ctx, "community inbox: forward read failed", err, "user_id", userID)
		}
	}
}

type messageRow struct {
	Type           string
	Content        string
	Status         int
	Link           string
	SenderID       *int
	RecipientID    int
	Created        time.Time
	Updated        time.Time
	NotificationID int64
	ThreadID       *int64
	PostNumber     *int32
}

func (in *Inbox) applyPage(ctx context.Context, notes []communityclient.NotificationView, nextAfter int64) (int, error) {
	rows, err := in.plan(ctx, notes)
	if err != nil {
		return 0, err
	}
	err = in.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range rows {
			if err := upsertMessage(tx, &rows[i]); err != nil {
				return err
			}
		}
		return writeFeedCursor(tx, nextAfter)
	})
	if err != nil {
		return 0, err
	}
	return len(rows), nil
}

func (in *Inbox) plan(ctx context.Context, notes []communityclient.NotificationView) ([]messageRow, error) {
	candidates := make([]communityclient.NotificationView, 0, len(notes))
	for _, n := range notes {
		if !mirrored(n.Kind) || !anchor.IsMoyu(n.AnchorKind, n.AnchorID) {
			continue
		}
		candidates = append(candidates, n)
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	targets, err := in.anchors.ResolveNamed(ctx, distinctRefs(candidates))
	if err != nil {
		return nil, err
	}
	excerpts := in.mentionExcerpts(ctx, candidates)

	users, err := in.localUserIDs(collectUserIDs(candidates))
	if err != nil {
		return nil, err
	}

	now := time.Now()
	rows := make([]messageRow, 0, len(candidates))
	for _, n := range candidates {
		ref := anchor.Ref{Kind: n.AnchorKind, ID: n.AnchorID}
		target, ok := targets[ref]
		if !ok {
			continue
		}
		// A resource wall whose resource is gone resolves to no game.
		if target.PatchID == 0 {
			continue
		}
		// user_message.recipient_id is a foreign key, and a mention can name
		// someone who has never signed in here.
		if _, ok := users[n.UserID]; !ok {
			continue
		}

		excerpt := ""
		if n.PostID != nil {
			excerpt = excerpts[*n.PostID]
		}
		mapped := messageFor(n, target, excerpt)

		row := messageRow{
			Type:           mapped.Type,
			Content:        mapped.Content,
			Status:         mapped.Status,
			Link:           mapped.Link,
			RecipientID:    int(n.UserID),
			Created:        parseTime(n.UpdatedAt, now),
			Updated:        now,
			NotificationID: n.ID,
			PostNumber:     n.PostNumber,
		}
		if n.ThreadID > 0 {
			tid := n.ThreadID
			row.ThreadID = &tid
		}
		if n.ActorID != nil && *n.ActorID > 0 {
			if _, ok := users[*n.ActorID]; ok {
				id := int(*n.ActorID)
				row.SenderID = &id
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (in *Inbox) mentionExcerpts(ctx context.Context, notes []communityclient.NotificationView) map[int64]string {
	ids := make([]int64, 0)
	seen := make(map[int64]struct{})
	for _, n := range notes {
		if n.Kind != communityclient.NotificationKindMentioned || n.PostID == nil || *n.PostID <= 0 {
			continue
		}
		id := *n.PostID
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	out := make(map[int64]string, len(ids))
	for start := 0; start < len(ids); start += mentionBatch {
		end := min(start+mentionBatch, len(ids))
		res, err := in.community.ResolvePosts(ctx, ids[start:end], 0)
		if err != nil {
			communityclient.LogDegraded(ctx, "community inbox: mention excerpt lookup failed", err)
			continue
		}
		for i := range res.Posts {
			p := res.Posts[i].Post
			out[p.ID] = p.ContentRaw
		}
	}
	return out
}

func (in *Inbox) localUserIDs(ids []int64) (map[int64]struct{}, error) {
	found := make(map[int64]struct{}, len(ids))
	if len(ids) == 0 {
		return found, nil
	}
	var rows []int64
	if err := in.db.Raw(`SELECT id FROM "user" WHERE id IN ?`, ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, id := range rows {
		found[id] = struct{}{}
	}
	return found, nil
}

func distinctRefs(notes []communityclient.NotificationView) []anchor.Ref {
	seen := make(map[anchor.Ref]struct{}, len(notes))
	refs := make([]anchor.Ref, 0, len(notes))
	for _, n := range notes {
		ref := anchor.Ref{Kind: n.AnchorKind, ID: n.AnchorID}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		refs = append(refs, ref)
	}
	return refs
}

func collectUserIDs(notes []communityclient.NotificationView) []int64 {
	seen := make(map[int64]struct{})
	ids := make([]int64, 0)
	add := func(id int64) {
		if id <= 0 {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	for _, n := range notes {
		add(n.UserID)
		if n.ActorID != nil {
			add(*n.ActorID)
		}
	}
	return ids
}

func parseTime(s string, fallback time.Time) time.Time {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return fallback
}

// upsertMessage keys a fold's version on `created`, which is the
// notification's upstream updated_at: community moves it with every seq bump
// (a fold that grew) and a local read never touches it. A page fetched before
// the reader read the row locally used to write status 0 back over that read.
// So an equal version keeps the local read, a newer one re-surfaces the row as
// unread, and an older one — a page another sync already moved past — is
// skipped.
func upsertMessage(tx *gorm.DB, row *messageRow) error {
	return tx.Exec(`
		INSERT INTO user_message (
			type, content, status, link, sender_id, recipient_id,
			created, updated, community_notification_id, community_thread_id, community_post_number
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (community_notification_id) WHERE community_notification_id IS NOT NULL
		DO UPDATE SET
			content = EXCLUDED.content,
			status = CASE WHEN EXCLUDED.created > user_message.created THEN EXCLUDED.status
				ELSE GREATEST(user_message.status, EXCLUDED.status) END,
			link = EXCLUDED.link,
			sender_id = EXCLUDED.sender_id,
			created = EXCLUDED.created,
			updated = EXCLUDED.updated,
			community_post_number = EXCLUDED.community_post_number
		WHERE EXCLUDED.created >= user_message.created
	`, row.Type, row.Content, row.Status, row.Link, row.SenderID, row.RecipientID,
		row.Created, row.Updated, row.NotificationID, row.ThreadID, row.PostNumber).Error
}

func readFeedCursor(db *gorm.DB) (int64, error) {
	var rows []int64
	if err := db.Raw(
		`SELECT last_id FROM cron_state WHERE name = ?`, feedCronName,
	).Scan(&rows).Error; err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return rows[0], nil
}

func writeFeedCursor(tx *gorm.DB, id int64) error {
	return tx.Exec(`
		INSERT INTO cron_state(name, last_id, updated_at)
		VALUES (?, ?, NOW())
		ON CONFLICT(name) DO UPDATE
		SET last_id = GREATEST(cron_state.last_id, EXCLUDED.last_id), updated_at = EXCLUDED.updated_at
	`, feedCronName, id).Error
}
