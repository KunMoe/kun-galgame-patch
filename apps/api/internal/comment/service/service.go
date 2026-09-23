// Package service is moyu's comment wall, backed by the NextMoe community
// primitive. Nothing here stores a comment or a like: a post and its reactions
// live in kun_community, and only the legacy id map stays local.
package service

import (
	"context"
	stderrors "errors"
	"log/slog"
	"net/http"
	"strconv"

	"kun-galgame-patch-api/internal/comment/repository"
	"kun-galgame-patch-api/internal/community/anchor"
	"kun-galgame-patch-api/internal/community/inbox"
	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/infrastructure/markdown"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/moemoepoint"
	"kun-galgame-patch-api/pkg/userclient"

	"gorm.io/gorm"
)

// AuditLogger records a moderator action. Nil in tests.
type AuditLogger interface {
	CreateLog(userID int, action string, detail any) error
}

type Service struct {
	community *communityclient.Client
	repo      *repository.Repository
	anchors   *anchor.Resolver
	users     *userclient.Client
	galgame   *galgameClient.Client
	db        *gorm.DB
	mp        *moemoepoint.Awarder
	audit     AuditLogger
	inbox     *inbox.Inbox
}

func New(
	community *communityclient.Client,
	repo *repository.Repository,
	anchors *anchor.Resolver,
	users *userclient.Client,
	galgame *galgameClient.Client,
	db *gorm.DB,
	mp *moemoepoint.Awarder,
	audit AuditLogger,
	notifications *inbox.Inbox,
) *Service {
	return &Service{
		community: community, repo: repo, anchors: anchors,
		users: users, galgame: galgame, db: db, mp: mp, audit: audit,
		inbox: notifications,
	}
}

func (s *Service) Configured() bool { return s.community.Configured() }

// Surface addresses one comment wall. moyu has two, and everything that differs
// between them is decided here: the anchor, the page a comment lives on, and
// the game whose counters a comment moves.
type Surface struct {
	AnchorKind int32
	AnchorID   string
	PatchID    int
	// ResourceID is 0 for the game wall.
	ResourceID int
}

func PatchSurface(patchID int) Surface {
	kind, id := anchor.PatchAnchor(patchID)
	return Surface{AnchorKind: kind, AnchorID: id, PatchID: patchID}
}

// ResourceSurface still carries the game id: a report's evidence link and the
// mention policy are patch-scoped even on a resource wall.
func ResourceSurface(resourceID, patchID int) Surface {
	kind, id := anchor.ResourceAnchor(resourceID)
	return Surface{AnchorKind: kind, AnchorID: id, PatchID: patchID, ResourceID: resourceID}
}

func (s Surface) resourceRef() *int {
	if s.ResourceID == 0 {
		return nil
	}
	id := s.ResourceID
	return &id
}

// Item is one comment as moyu renders it. Replies are NOT nested: community
// numbers a wall's posts in one flat sequence and reports the parent by id, so
// the tree is built where it is drawn.
type Item struct {
	ID          int64  `json:"id"`
	ThreadID    int64  `json:"thread_id"`
	Content     string `json:"content"`
	ContentHTML string `json:"content_html"`
	GalgameID   int    `json:"galgame_id"`
	ResourceID  *int   `json:"resource_id,omitempty"`

	User       *patchModel.PatchUser `json:"user,omitempty"`
	TargetUser *patchModel.PatchUser `json:"target_user,omitempty"`

	ParentCommentID *int64 `json:"parent_comment_id"`
	RootCommentID   *int64 `json:"root_comment_id"`

	LikeCount int  `json:"like_count"`
	IsLiked   bool `json:"is_liked"`

	Created           string  `json:"created"`
	Edited            *string `json:"edited"`
	EditedByModerator bool    `json:"edited_by_moderator"`
	Status            int32   `json:"status"`
	Deleted           bool    `json:"deleted"`
	Held              bool    `json:"held"`
}

type Page struct {
	ThreadID   int64   `json:"thread_id"`
	Posts      []*Item `json:"posts"`
	NextCursor string  `json:"next_cursor"`
	Total      int     `json:"total"`
}

const (
	defaultPageLimit = 30
	maxPageLimit     = 50
)

// Wall reads one anchor's comments. Before the first comment there is no thread
// and this says so — it does not create one. Its predecessor
// POST /comments/resolve was get-or-create, and because every site called it to
// RENDER a page it had minted 110,918 empty threads upstream by 2026-09-15.
func (s *Service) Wall(ctx context.Context, surface Surface, viewerID int, after string, limit int) (*Page, *errors.AppError) {
	page, err := s.community.GetComments(ctx, surface.AnchorKind, surface.AnchorID,
		after, clampLimit(limit), int64(viewerID))
	if err != nil {
		if isCommunityDown(err) {
			return emptyPage(), nil
		}
		return nil, mapError(err)
	}
	if page.Thread == nil {
		return emptyPage(), nil
	}
	return &Page{
		ThreadID:   page.Thread.ID,
		Posts:      s.render(ctx, surface, viewerID, page.Posts),
		NextCursor: page.NextCursor,
		Total:      int(page.Thread.PostsCount),
	}, nil
}

func (s *Service) render(ctx context.Context, surface Surface, viewerID int, posts []communityclient.PostView) []*Item {
	uids := make([]int, 0, len(posts)*2)
	for _, p := range posts {
		uids = append(uids, int(p.AuthorID))
		if p.TargetUserID != 0 {
			uids = append(uids, int(p.TargetUserID))
		}
	}
	briefs := userclient.BriefMapByInt(ctx, s.users, uids)

	out := make([]*Item, 0, len(posts))
	for _, p := range posts {
		if !visibleTo(p.Status, p.AuthorID, viewerID) {
			continue
		}
		item := buildItem(p, surface, patchModel.NewPatchUser(briefs[int(p.AuthorID)]))
		if p.TargetUserID != 0 {
			item.TargetUser = patchModel.NewPatchUser(briefs[int(p.TargetUserID)])
		}
		out = append(out, item)
	}
	return out
}

// A held post — the TL0 newcomer sandbox holds a first post for review — is
// visible only to its author. A tombstone stays in place: its post_number is
// what keeps the wall's numbering, so it renders as a stub rather than vanishing.
func visibleTo(status int32, authorID int64, viewerID int) bool {
	if status == communityclient.PostHeld {
		return viewerID != 0 && int64(viewerID) == authorID
	}
	return true
}

func buildItem(p communityclient.PostView, surface Surface, author *patchModel.PatchUser) *Item {
	deleted := p.Status == communityclient.PostDeleted
	raw := p.ContentRaw
	if deleted {
		raw = ""
	}
	item := &Item{
		ID:                p.ID,
		ThreadID:          p.ThreadID,
		Content:           raw,
		ContentHTML:       markdown.MustRender(raw),
		GalgameID:         surface.PatchID,
		ResourceID:        surface.resourceRef(),
		User:              author,
		ParentCommentID:   nonZero(p.ReplyToPostID),
		RootCommentID:     nonZero(p.RootPostID),
		LikeCount:         int(p.ReactionCount),
		IsLiked:           p.ViewerReacted,
		Created:           p.CreatedAt,
		EditedByModerator: p.EditedByModerator,
		Status:            p.Status,
		Deleted:           deleted,
		Held:              p.Status == communityclient.PostHeld,
	}
	if p.EditedAt != "" {
		edited := p.EditedAt
		item.Edited = &edited
	}
	return item
}

// Markdown answers the editor with the post's source. It is read through the
// id-addressed face rather than the wall, so opening the editor does not depend
// on knowing which page the comment is on.
func (s *Service) Markdown(ctx context.Context, postID int64) (string, *errors.AppError) {
	post, _, appErr := s.resolvePost(ctx, postID, 0)
	if appErr != nil {
		return "", appErr
	}
	if post.Status == communityclient.PostDeleted {
		return "", errors.ErrNotFound("comment not found")
	}
	return post.ContentRaw, nil
}

// LocateResult points a deep link at the post that now holds the comment.
//
// Paging is a cursor now, so there is no page number to hand back: the reader's
// wall loads forward until the post appears.
type LocateResult struct {
	PostID     int64 `json:"post_id"`
	ThreadID   int64 `json:"thread_id"`
	GalgameID  int   `json:"galgame_id"`
	ResourceID *int  `json:"resource_id,omitempty"`
}

// Locate resolves a PRE-CUTOVER comment id through the import's map. Links
// minted since the cutover already carry a post id and never reach this.
func (s *Service) Locate(legacyID int) (*LocateResult, *errors.AppError) {
	row, err := s.repo.FindMapByLegacyID(legacyID)
	if err != nil {
		return nil, errors.ErrInternal("")
	}
	if row == nil {
		return nil, errors.ErrNotFound("comment not found")
	}
	return &LocateResult{
		PostID:     row.PostID,
		ThreadID:   row.ThreadID,
		GalgameID:  row.GalgameID,
		ResourceID: row.ResourceID,
	}, nil
}

// resolvePost answers a post plus the wall it belongs to, and refuses a post
// whose anchor is not one of moyu's. The tenant guard upstream already 404s
// another site's post; this is what turns a catalog-anchored one — network-wide
// by design, and reachable by id — into the same clean refusal.
func (s *Service) resolvePost(ctx context.Context, postID int64, viewerID int) (*communityclient.PostView, Surface, *errors.AppError) {
	resolved, err := s.community.ResolvePosts(ctx, []int64{postID}, int64(viewerID))
	if err != nil {
		return nil, Surface{}, mapError(err)
	}
	for i := range resolved.Posts {
		row := resolved.Posts[i]
		if row.Post.ID != postID {
			continue
		}
		surface, ok := s.surfaceFor(row.Thread.AnchorKind, row.Thread.AnchorID)
		if !ok {
			return nil, Surface{}, errors.ErrNotFound("comment not found")
		}
		return &row.Post, surface, nil
	}
	return nil, Surface{}, errors.ErrNotFound("comment not found")
}

func (s *Service) surfaceFor(anchorKind int32, anchorID string) (Surface, bool) {
	targets := s.anchors.Resolve([]anchor.Ref{{Kind: anchorKind, ID: anchorID}})
	target, ok := targets[anchor.Ref{Kind: anchorKind, ID: anchorID}]
	if !ok {
		return Surface{}, false
	}
	if target.ResourceID > 0 {
		return ResourceSurface(target.ResourceID, target.PatchID), true
	}
	return PatchSurface(target.PatchID), true
}

func nonZero(id int64) *int64 {
	if id == 0 {
		return nil
	}
	return &id
}

func clampLimit(limit int) string {
	if limit < 1 || limit > maxPageLimit {
		limit = defaultPageLimit
	}
	return strconv.Itoa(limit)
}

func emptyPage() *Page { return &Page{Posts: []*Item{}} }

// isCommunityDown separates "the service could not be reached" from "the
// service refused this request". The first degrades a read to an empty wall;
// the second is an answer the reader has to see.
func isCommunityDown(err error) bool {
	if stderrors.Is(err, communityclient.ErrNotConfigured) || stderrors.Is(err, communityclient.ErrForbidden) {
		return true
	}
	var apiErr *communityclient.APIError
	return err != nil && !stderrors.As(err, &apiErr) && !stderrors.Is(err, communityclient.ErrRateLimited)
}

func mapError(err error) *errors.AppError {
	switch {
	case stderrors.Is(err, communityclient.ErrRateLimited):
		return errors.ErrTooManyRequests("发表过于频繁，请稍后再试（新人限制）")
	case stderrors.Is(err, communityclient.ErrForbidden):
		return errors.ErrForbidden()
	case stderrors.Is(err, communityclient.ErrNotConfigured):
		return errors.ErrCommunityUnavailable("")
	}
	var apiErr *communityclient.APIError
	if stderrors.As(err, &apiErr) {
		if apiErr.Status == http.StatusNotFound {
			return errors.ErrNotFound("comment not found")
		}
		if apiErr.Status >= 400 && apiErr.Status < 500 {
			return errors.New(apiErr.Code, apiErr.Msg, apiErr.Status)
		}
	}
	slog.Error("community upstream error", "error", err)
	return errors.ErrCommunityUnavailable("")
}

func (s *Service) brief(ctx context.Context, userID int) *userclient.Brief {
	return userclient.BriefMapByInt(ctx, s.users, []int{userID})[userID]
}

// ResourceRef answers who published a resource and which game it hangs off —
// the two facts a resource wall needs before it can be addressed at all.
func (s *Service) ResourceRef(resourceID int) (*repository.ResourceRef, error) {
	return s.repo.ResourceRef(resourceID)
}
