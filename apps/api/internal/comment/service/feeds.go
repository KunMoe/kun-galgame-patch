package service

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"unicode/utf8"

	"kun-galgame-patch-api/internal/community/anchor"
	"kun-galgame-patch-api/internal/galgame/enricher"
	"kun-galgame-patch-api/internal/infrastructure/markdown"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/userclient"
)

// FeedItem is one comment as the mixed lists print it — the global feed, a
// user's profile, the home page, the search lane and the admin queue. Unlike a
// wall Item it carries where the comment lives, because the reader is looking
// at rows from many walls at once.
type FeedItem struct {
	ID          int64  `json:"id"`
	ThreadID    int64  `json:"thread_id"`
	Content     string `json:"content"`
	ContentHTML string `json:"content_html"`
	GalgameID   int    `json:"galgame_id"`
	ResourceID  *int   `json:"resource_id,omitempty"`

	User *patchModel.PatchUser `json:"user,omitempty"`

	LikeCount int    `json:"like_count"`
	Created   string `json:"created"`
	Status    int32  `json:"status"`

	// Link is the permalink moyu answers for this row. It is built from the
	// anchor rather than guessed from the ids, because only the anchor says
	// which of the two walls the comment is on.
	Link  string                   `json:"link"`
	Patch *patchModel.PatchSummary `json:"patch,omitempty"`
}

type FeedPage struct {
	Items      []*FeedItem `json:"items"`
	NextCursor string      `json:"next_cursor"`
}

// PatchSummaryDB is the one thing the feeds need from moyu's own tables: the
// vndb id behind a patch id, which the summary builder turns into a name.
type PatchSummaryDB = enricher.PatchSummaryDB

const (
	searchMinRunes = 2
	searchMaxRunes = 100

	// The window the other moyu lanes cut a snippet with.
	snippetLen  = 233
	snippetLead = 30
)

// AuthorFeed is a user's own comments. This face keysets by post id, which is
// the existing upstream contract for it.
func (s *Service) AuthorFeed(ctx context.Context, authorID int, after string, limit int, cl string, db PatchSummaryDB) (*FeedPage, *errors.AppError) {
	if !s.community.Configured() {
		return emptyFeed(), nil
	}
	// Every anchor kind: a user's profile lists both of moyu's walls.
	page, err := s.community.AuthorPosts(ctx, int64(authorID), after, limit, int(communityclient.AnyKind))
	if err != nil {
		if isCommunityDown(err) {
			return emptyFeed(), nil
		}
		return nil, mapError(err)
	}
	return s.renderFeed(ctx, page.Posts, page.NextCursor, "", cl, db), nil
}

// SiteFeed is the newest comments across every wall. It keysets on creation
// time, not id: the import gives historical comments fresh ids, so id order is
// import order and a feed keyed on it would open with 2024.
func (s *Service) SiteFeed(ctx context.Context, cursor string, limit int, cl string, db PatchSummaryDB) (*FeedPage, *errors.AppError) {
	if !s.community.Configured() {
		return emptyFeed(), nil
	}
	page, err := s.community.ListSitePosts(ctx, communityclient.SitePostsQuery{
		Kind:       communityclient.KindComments,
		AnchorKind: communityclient.AnyKind,
		Cursor:     cursor,
		Limit:      limit,
	})
	if err != nil {
		if isCommunityDown(err) {
			return emptyFeed(), nil
		}
		return nil, mapError(err)
	}
	return s.renderFeed(ctx, page.Posts, page.NextCursor, "", cl, db), nil
}

// Search matches a comment's markdown source, not its cooked HTML: searching
// the HTML makes `nofollow` match every comment that carries a link.
func (s *Service) Search(ctx context.Context, q, cursor string, limit int, cl string, db PatchSummaryDB) (*FeedPage, *errors.AppError) {
	q = strings.TrimSpace(q)
	if n := utf8.RuneCountInString(q); n < searchMinRunes || n > searchMaxRunes {
		return nil, errors.ErrBadRequest("搜索评论需要 2-100 个字符")
	}
	if !s.community.Configured() {
		return emptyFeed(), nil
	}
	page, err := s.community.SearchPosts(ctx, q, communityclient.KindComments, cursor, limit)
	if err != nil {
		if isCommunityDown(err) {
			return emptyFeed(), nil
		}
		return nil, mapError(err)
	}
	return s.renderFeed(ctx, page.Posts, page.NextCursor, q, cl, db), nil
}

// AuthorCounts is how many visible comments each user has — the number the
// profile and the user search lane print.
func (s *Service) AuthorCounts(ctx context.Context, userIDs []int) map[int]int64 {
	out := make(map[int]int64, len(userIDs))
	if !s.community.Configured() || len(userIDs) == 0 {
		return out
	}
	ids := make([]int64, 0, len(userIDs))
	for _, id := range userIDs {
		if id > 0 {
			ids = append(ids, int64(id))
		}
	}
	stats, err := s.community.AuthorStats(ctx, ids, communityclient.KindComments, communityclient.AnyKind)
	if err != nil {
		slog.Warn("comment: author stats failed (best-effort)", "error", err)
		return out
	}
	for _, stat := range stats.Stats {
		out[int(stat.AuthorID)] = stat.VisiblePosts
	}
	return out
}

// AuthorBoard is the site's most-commented authors, in order. The per-author
// batch above answers a caller that already knows whose numbers it wants, which
// is why ranking the whole population needs a face of its own.
func (s *Service) AuthorBoard(ctx context.Context, limit int) []int {
	if !s.community.Configured() || limit <= 0 {
		return nil
	}
	top, err := s.community.TopAuthors(ctx, communityclient.KindComments, communityclient.AnyKind, limit)
	if err != nil {
		slog.Warn("comment: top authors failed (best-effort)", "error", err)
		return nil
	}
	ids := make([]int, 0, len(top.Stats))
	for _, stat := range top.Stats {
		ids = append(ids, int(stat.AuthorID))
	}
	return ids
}

// renderFeed drops every row whose anchor moyu cannot link to. The site feed and
// the post search answer this site's threads PLUS every catalog-anchored one,
// which are a network-wide conversation by design — a row from one would point
// the reader at a moyu page that never held it.
func (s *Service) renderFeed(ctx context.Context, rows []communityclient.AuthorPostView, next, highlight, cl string, db PatchSummaryDB) *FeedPage {
	refs := make([]anchor.Ref, 0, len(rows))
	uids := make([]int, 0, len(rows))
	for _, row := range rows {
		refs = append(refs, anchor.Ref{Kind: row.Thread.AnchorKind, ID: row.Thread.AnchorID})
		uids = append(uids, int(row.Post.AuthorID))
	}
	targets := s.anchors.Resolve(refs)
	briefs := userclient.BriefMapByInt(ctx, s.users, uids)

	items := make([]*FeedItem, 0, len(rows))
	for _, row := range rows {
		target, ok := targets[anchor.Ref{Kind: row.Thread.AnchorKind, ID: row.Thread.AnchorID}]
		if !ok {
			continue
		}
		if row.Post.Status != communityclient.PostVisible {
			continue
		}
		var resourceID *int
		if target.ResourceID > 0 {
			id := target.ResourceID
			resourceID = &id
		}
		content := row.Post.ContentRaw
		if highlight != "" {
			content = snippet(content, highlight)
		}
		items = append(items, &FeedItem{
			ID:          row.Post.ID,
			ThreadID:    row.Thread.ThreadID,
			Content:     content,
			ContentHTML: markdown.MustRender(content),
			GalgameID:   target.PatchID,
			ResourceID:  resourceID,
			User:        briefToUser(briefs[int(row.Post.AuthorID)]),
			LikeCount:   int(row.Post.ReactionCount),
			Created:     row.Post.CreatedAt,
			Status:      row.Post.Status,
			Link:        permalink(target, row.Post.ID),
		})
	}

	items = enricher.FilterByGalgameContentLimit(ctx, s.galgame, items, func(it *FeedItem) int { return it.GalgameID }, cl)
	s.attachSummaries(ctx, items, db)
	return &FeedPage{Items: items, NextCursor: next}
}

func (s *Service) attachSummaries(ctx context.Context, items []*FeedItem, db PatchSummaryDB) {
	if len(items) == 0 || db == nil {
		return
	}
	seen := make(map[int]struct{}, len(items))
	ids := make([]int, 0, len(items))
	for _, it := range items {
		if it.GalgameID <= 0 {
			continue
		}
		if _, ok := seen[it.GalgameID]; ok {
			continue
		}
		seen[it.GalgameID] = struct{}{}
		ids = append(ids, it.GalgameID)
	}
	summaries := enricher.BuildPatchSummaryMap(ctx, s.galgame, db, ids)
	for _, it := range items {
		if summary, ok := summaries[it.GalgameID]; ok {
			row := summary
			it.Patch = &row
		}
	}
}

// permalink anchors on the POST id, never on `#comment-<n>`: that shape is the
// pre-cutover comment id and still has to resolve through the import's map, so
// the two must not collide when a post id happens to equal an old comment id.
func permalink(target anchor.Target, postID int64) string {
	return target.Link + "#post-" + strconv.FormatInt(postID, 10)
}

// snippet windows the plain text around the first hit. Without it the card shows
// the opening of a long comment and highlights nothing, because the match the
// reader searched for is a thousand characters further down.
func snippet(raw, q string) string {
	runes := []rune(raw)
	if len(runes) <= snippetLen {
		return raw
	}
	at := utf8.RuneCountInString(raw[:max(strings.Index(strings.ToLower(raw), strings.ToLower(q)), 0)])
	if at <= snippetLead {
		return string(runes[:snippetLen])
	}
	start := at - snippetLead
	end := min(start+snippetLen, len(runes))
	return "…" + string(runes[start:end])
}

func emptyFeed() *FeedPage { return &FeedPage{Items: []*FeedItem{}} }

// PatchIDsForRefs answers which game page each `comment:<id>` reference belongs
// to — the moemoepoint ledger prints one row per awarded comment and links it.
//
// The ids arrive from two eras and cannot be told apart by shape: a pre-cutover
// ref carries a patch_comment id, a later one carries a community post id, and
// the ranges overlap. The map is consulted first because it is the only thing
// that can answer the old ones, and whatever it misses is looked up upstream.
func (s *Service) PatchIDsForRefs(ctx context.Context, ids []int) map[int]int {
	out := make(map[int]int, len(ids))
	if len(ids) == 0 {
		return out
	}
	legacy := s.repo.MapByLegacyIDs(ids)
	remaining := make([]int64, 0, len(ids))
	for _, id := range ids {
		if row, ok := legacy[id]; ok {
			out[id] = row.GalgameID
			continue
		}
		remaining = append(remaining, int64(id))
	}
	if len(remaining) == 0 || !s.community.Configured() {
		return out
	}
	resolved, err := s.community.ResolvePosts(ctx, remaining, 0)
	if err != nil {
		slog.Warn("comment: moemoepoint ref resolve failed (best-effort)", "error", err)
		return out
	}
	for _, row := range resolved.Posts {
		if surface, ok := s.surfaceFor(row.Thread.AnchorKind, row.Thread.AnchorID); ok {
			out[int(row.Post.ID)] = surface.PatchID
		}
	}
	return out
}
