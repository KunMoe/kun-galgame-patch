// Package engagement is the read receipt / subscription half of the community
// primitive: which walls a reader follows, and how much of each is unread.
package engagement

import (
	"context"
	stderrors "errors"
	"log/slog"
	"math"
	"net/http"

	"kun-galgame-patch-api/internal/community/anchor"
	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/errors"
)

type Service struct {
	community *communityclient.Client
	anchors   *anchor.Resolver
}

func New(community *communityclient.Client, anchors *anchor.Resolver) *Service {
	return &Service{community: community, anchors: anchors}
}

// State is a viewer's standing on one comment wall. Subscribed is false for a
// wall they have never written on or followed, which is the common case: the
// community service keeps no row until someone interacts.
type State struct {
	ThreadID          int64 `json:"thread_id"`
	Subscribed        bool  `json:"subscribed"`
	NotificationLevel int32 `json:"notification_level"`
	UnreadCount       int32 `json:"unread_count"`
}

type UnreadItem struct {
	ThreadID          int64  `json:"thread_id"`
	Link              string `json:"link"`
	Title             string `json:"title"`
	Label             string `json:"label"`
	PatchID           int    `json:"patch_id,omitempty"`
	ResourceID        int    `json:"resource_id,omitempty"`
	UnreadCount       int32  `json:"unread_count"`
	NotificationLevel int32  `json:"notification_level"`
	LastPostedAt      string `json:"last_posted_at"`
}

type UnreadResult struct {
	Items      []UnreadItem `json:"items"`
	NextCursor string       `json:"next_cursor"`
	Total      int64        `json:"total"`
}

// MarkRead reports a read receipt for a wall the viewer already follows, and
// deliberately does not create the row for one they do not.
//
// The community service's POST /threads/{id}/read inserts at notification level
// normal, and its unread listing counts every level except muted. Reporting a
// receipt for every wall a reader merely opened would therefore enrol them in
// every game page they ever visited, and the red dot would light for a game
// they glanced at once.
func (s *Service) MarkRead(ctx context.Context, userID int, threadID int64) *State {
	if !s.community.Configured() || threadID <= 0 {
		return &State{ThreadID: threadID}
	}
	existing, err := s.community.ThreadStates(ctx, int64(userID), []int64{threadID})
	if err != nil {
		slog.Warn("community engagement: thread state lookup failed", "thread_id", threadID, "error", err)
		return &State{ThreadID: threadID}
	}
	if len(existing.States) == 0 {
		return &State{ThreadID: threadID}
	}
	// Community clamps to the thread's highest post number, so this means "all
	// of it" without the caller having to know what that number is.
	view, err := s.community.MarkThreadRead(ctx, threadID, int64(userID), math.MaxInt32)
	if err != nil {
		slog.Warn("community engagement: mark read failed", "thread_id", threadID, "error", err)
		return toState(threadID, &existing.States[0])
	}
	return toState(threadID, view)
}

// SetLevel follows or mutes a wall. Following it also clears its backlog: the
// upsert behind the level starts a fresh row at post 0, so without the receipt
// the reader would subscribe and immediately owe themselves every post on the
// wall as unread.
func (s *Service) SetLevel(ctx context.Context, userID int, threadID int64, level int32) (*State, *errors.AppError) {
	if level < communityclient.NotificationMuted || level > communityclient.NotificationWatching {
		return nil, errors.ErrBadRequest("订阅级别不正确")
	}
	if !s.community.Configured() {
		return nil, errors.ErrCommunityUnavailable("")
	}
	view, err := s.community.SetThreadNotification(ctx, threadID, int64(userID), level)
	if err != nil {
		return nil, mapErr(err, "设置订阅失败")
	}
	if level != communityclient.NotificationMuted {
		if read, rerr := s.community.MarkThreadRead(ctx, threadID, int64(userID), math.MaxInt32); rerr == nil {
			view = read
		}
	}
	return toState(threadID, view), nil
}

func (s *Service) Unread(ctx context.Context, userID int, cursor string, limit int) (*UnreadResult, *errors.AppError) {
	empty := &UnreadResult{Items: []UnreadItem{}}
	if !s.community.Configured() {
		return empty, nil
	}
	page, err := s.community.ListUnread(ctx, int64(userID), cursor, limit)
	if err != nil {
		return nil, mapErr(err, "获取未读评论失败")
	}

	refs := make([]anchor.Ref, 0, len(page.Threads))
	for _, row := range page.Threads {
		refs = append(refs, anchor.Ref{Kind: row.Thread.AnchorKind, ID: row.Thread.AnchorID})
	}
	targets := s.anchors.ResolveNamed(ctx, refs)

	items := make([]UnreadItem, 0, len(page.Threads))
	for _, row := range page.Threads {
		target, ok := targets[anchor.Ref{Kind: row.Thread.AnchorKind, ID: row.Thread.AnchorID}]
		if !ok {
			continue
		}
		title := target.Title
		if title == "" {
			title = target.Label
		}
		items = append(items, UnreadItem{
			ThreadID:          row.Thread.ID,
			Link:              target.Link,
			Title:             title,
			Label:             target.Label,
			PatchID:           target.PatchID,
			ResourceID:        target.ResourceID,
			UnreadCount:       row.State.UnreadCount,
			NotificationLevel: row.State.NotificationLevel,
			LastPostedAt:      row.Thread.LastPostedAt,
		})
	}
	return &UnreadResult{Items: items, NextCursor: page.NextCursor, Total: page.Total}, nil
}

// Count is the red dot. It is best-effort: a community service that is down
// must not blank the notification bell the rest of the site fills.
func (s *Service) Count(ctx context.Context, userID int) int64 {
	if !s.community.Configured() {
		return 0
	}
	page, err := s.community.ListUnread(ctx, int64(userID), "", 1)
	if err != nil {
		slog.Warn("community engagement: unread count failed (best-effort)", "user_id", userID, "error", err)
		return 0
	}
	return page.Total
}

func toState(threadID int64, view *communityclient.ThreadUserView) *State {
	if view == nil {
		return &State{ThreadID: threadID}
	}
	return &State{
		ThreadID:          threadID,
		Subscribed:        view.NotificationLevel > communityclient.NotificationMuted,
		NotificationLevel: view.NotificationLevel,
		UnreadCount:       view.UnreadCount,
	}
}

func mapErr(err error, fallback string) *errors.AppError {
	var apiErr *communityclient.APIError
	if stderrors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		return errors.ErrNotFound("评论区不存在")
	}
	slog.Warn("community engagement request failed", "error", err)
	return errors.ErrInternal(fallback)
}
