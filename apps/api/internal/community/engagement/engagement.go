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
	"kun-galgame-patch-api/internal/community/inbox"
	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/errors"
)

type Service struct {
	community *communityclient.Client
	anchors   *anchor.Resolver
	inbox     *inbox.Inbox
}

func New(community *communityclient.Client, anchors *anchor.Resolver, in *inbox.Inbox) *Service {
	return &Service{community: community, anchors: anchors, inbox: in}
}

// State is a viewer's standing on one comment wall. Subscribed is false at
// muted and normal; tracking and watching (level >= 2) are followed.
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
}

// ReadWall reports a read receipt only for a wall the viewer already has a row
// on or follows. A receipt creates the thread row, and upstream's unread
// listing counts every level except muted: a receipt for every wall a reader
// merely opened would fill it with game pages they visited once, which Unread
// then has to drop from every page it reads.
//
// A follower of the wall's anchor does get the receipt. Their thread row is
// seeded watching upstream, which is what puts a wall they followed before its
// first comment onto the unread list.
func (s *Service) ReadWall(ctx context.Context, userID int, anchorKind int32, anchorID string, threadID int64) *State {
	if !s.community.Configured() {
		return &State{ThreadID: threadID}
	}

	var row *communityclient.ThreadUserView
	if threadID > 0 {
		existing, err := s.community.ThreadStates(ctx, int64(userID), []int64{threadID})
		if err != nil {
			slog.Warn("community engagement: thread state lookup failed", "thread_id", threadID, "error", err)
			return &State{ThreadID: threadID}
		}
		if len(existing.States) > 0 {
			row = &existing.States[0]
		}
	}

	anchorLevel := int32(communityclient.NotificationNormal)
	if row == nil {
		states, err := s.community.AnchorStates(ctx, int64(userID), []communityclient.AnchorRef{
			{AnchorKind: anchorKind, AnchorID: anchorID},
		})
		if err != nil {
			slog.Warn("community engagement: anchor state lookup failed", "anchor_id", anchorID, "error", err)
			return &State{ThreadID: threadID}
		}
		if len(states.States) > 0 {
			anchorLevel = states.States[0].NotificationLevel
		}
	}

	if threadID > 0 && (row != nil || anchorLevel == communityclient.NotificationWatching) {
		view, err := s.community.MarkThreadRead(ctx, threadID, int64(userID), math.MaxInt32)
		if err != nil {
			slog.Warn("community engagement: mark read failed", "thread_id", threadID, "error", err)
		} else {
			row = view
			s.inbox.MarkThreadRead(userID, threadID, view.LastReadPostNumber)
		}
	}

	if row != nil {
		return toState(threadID, row)
	}
	return stateFromLevel(threadID, anchorLevel)
}

func (s *Service) SetWallLevel(ctx context.Context, userID int, anchorKind int32, anchorID string, threadID int64, level int32) (*State, *errors.AppError) {
	if level != communityclient.NotificationNormal && level != communityclient.NotificationWatching {
		return nil, errors.ErrBadRequest("订阅级别不正确")
	}
	if !s.community.Configured() {
		return nil, errors.ErrCommunityUnavailable("")
	}

	anchorView, err := s.community.SetAnchorNotification(ctx, int64(userID), anchorKind, anchorID, level)
	if err != nil {
		return nil, mapErr(err, "设置订阅失败")
	}

	var threadView *communityclient.ThreadUserView
	if threadID > 0 {
		view, err := s.community.SetThreadNotification(ctx, threadID, int64(userID), level)
		if err != nil {
			return nil, mapErr(err, "设置订阅失败")
		}
		threadView = view
		if level == communityclient.NotificationWatching {
			if read, rerr := s.community.MarkThreadRead(ctx, threadID, int64(userID), math.MaxInt32); rerr == nil {
				threadView = read
				s.inbox.MarkThreadRead(userID, threadID, read.LastReadPostNumber)
			}
		}
	}

	if threadView != nil {
		return toState(threadID, threadView), nil
	}
	return stateFromLevel(threadID, anchorView.NotificationLevel), nil
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
		if row.State.NotificationLevel < communityclient.NotificationTracking {
			continue
		}
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
	return &UnreadResult{Items: items, NextCursor: page.NextCursor}, nil
}

func toState(threadID int64, view *communityclient.ThreadUserView) *State {
	if view == nil {
		return &State{ThreadID: threadID}
	}
	return &State{
		ThreadID:          threadID,
		Subscribed:        view.NotificationLevel >= communityclient.NotificationTracking,
		NotificationLevel: view.NotificationLevel,
		UnreadCount:       view.UnreadCount,
	}
}

func stateFromLevel(threadID int64, level int32) *State {
	return &State{
		ThreadID:          threadID,
		Subscribed:        level >= communityclient.NotificationTracking,
		NotificationLevel: level,
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
