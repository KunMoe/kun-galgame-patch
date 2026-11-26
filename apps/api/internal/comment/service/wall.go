package service

import (
	"context"

	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/userclient"
)

// Page is one page of a wall's top-level comments. Replies come separately:
// the first few of every listed root in Replies, the rest from Replies().
type Page struct {
	ThreadID int64 `json:"thread_id"`
	// Pinned is on the first page only, newest pin first. The same comments
	// also appear in Posts at their own place.
	Pinned     []*Item `json:"pinned"`
	Posts      []*Item `json:"posts"`
	Replies    []*Item `json:"replies"`
	NextCursor string  `json:"next_cursor"`
	Total      int     `json:"total"`
}

type RepliesPage struct {
	Posts      []*Item `json:"posts"`
	NextCursor string  `json:"next_cursor"`
}

type WallQuery struct {
	Sort   string
	Cursor string
	Limit  int
}

const (
	defaultPageLimit  = 30
	maxPageLimit      = 50
	replyPreviewCount = 3
)

// Wall reads one anchor's comments. Before the first comment there is no thread
// and this says so — it does not create one. Its predecessor
// POST /comments/resolve was get-or-create, and because every site called it to
// RENDER a page it had minted 110,918 empty threads upstream by 2026-09-15.
func (s *Service) Wall(ctx context.Context, surface Surface, viewerID int, q WallQuery) (*Page, error) {
	if !s.community.Configured() {
		return emptyPage(), nil
	}
	page, err := s.community.GetComments(ctx, surface.AnchorKind, surface.AnchorID, communityclient.CommentsQuery{
		Sort: q.Sort, Cursor: q.Cursor, Limit: clampLimit(q.Limit),
		RootsOnly: true, ReplyPreview: replyPreviewCount,
	}, int64(viewerID))
	if err != nil {
		if unavailable(ctx, err, "wall") {
			return emptyPage(), nil
		}
		return nil, err
	}
	if page.Thread == nil {
		return emptyPage(), nil
	}
	lists := s.render(ctx, surface, viewerID, page.PinnedPosts, page.Posts, page.ReplyPreviews)
	return &Page{
		ThreadID:   page.Thread.ID,
		Pinned:     lists[0],
		Posts:      lists[1],
		Replies:    lists[2],
		NextCursor: page.NextCursor,
		Total:      int(page.Thread.PostsCount),
	}, nil
}

// Replies pages one top-level comment's replies, oldest first. `after` is the
// post_number of the last reply the reader already has.
func (s *Service) Replies(ctx context.Context, rootID int64, viewerID int, after string, limit int) (*RepliesPage, Surface, error) {
	_, surface, err := s.resolvePost(ctx, rootID, viewerID)
	if err != nil {
		return nil, Surface{}, err
	}
	page, err := s.community.ListReplies(ctx, rootID, after, clampLimit(limit), int64(viewerID))
	if err != nil {
		return nil, Surface{}, err
	}
	return &RepliesPage{
		Posts:      s.render(ctx, surface, viewerID, page.Posts)[0],
		NextCursor: page.NextCursor,
	}, surface, nil
}

// Post answers one comment, which is how a deep link learns the root it has to
// load before it can scroll to a reply.
func (s *Service) Post(ctx context.Context, postID int64, viewerID int) (*Item, Surface, error) {
	post, surface, err := s.resolvePost(ctx, postID, viewerID)
	if err != nil {
		return nil, Surface{}, err
	}
	items := s.render(ctx, surface, viewerID, []communityclient.PostView{*post})[0]
	if len(items) == 0 {
		return nil, Surface{}, errCommentNotFound
	}
	return items[0], surface, nil
}

// render resolves every author across all lists in one batch.
func (s *Service) render(ctx context.Context, surface Surface, viewerID int, lists ...[]communityclient.PostView) [][]*Item {
	var uids []int
	for _, posts := range lists {
		for _, p := range posts {
			uids = append(uids, int(p.AuthorID))
			if p.TargetUserID != 0 {
				uids = append(uids, int(p.TargetUserID))
			}
		}
	}
	briefs := userclient.BriefMapByInt(ctx, s.users, uids)

	out := make([][]*Item, len(lists))
	for i, posts := range lists {
		out[i] = make([]*Item, 0, len(posts))
		for _, p := range posts {
			if !visibleTo(p.Status, p.AuthorID, viewerID) {
				continue
			}
			item := buildItem(p, surface, patchModel.NewPatchUser(briefs[int(p.AuthorID)]))
			if p.TargetUserID != 0 {
				item.TargetUser = patchModel.NewPatchUser(briefs[int(p.TargetUserID)])
			}
			out[i] = append(out[i], item)
		}
	}
	return out
}

func clampLimit(limit int) int {
	if limit < 1 || limit > maxPageLimit {
		return defaultPageLimit
	}
	return limit
}

func emptyPage() *Page {
	return &Page{Pinned: []*Item{}, Posts: []*Item{}, Replies: []*Item{}}
}
