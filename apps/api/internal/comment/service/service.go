// Package service is moyu's comment wall, backed by the NextMoe community
// primitive. Nothing here stores a comment or a like: a post and its reactions
// live in kun_community, and only the legacy id map stays local.
package service

import (
	"context"
	"log/slog"

	"kun-galgame-patch-api/internal/comment/repository"
	"kun-galgame-patch-api/internal/community/anchor"
	"kun-galgame-patch-api/internal/community/inbox"
	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/infrastructure/markdown"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/moemoepoint"
	"kun-galgame-patch-api/pkg/upstream"
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

	PostNumber      int32  `json:"post_number"`
	ParentCommentID *int64 `json:"parent_comment_id"`
	RootCommentID   *int64 `json:"root_comment_id"`
	// ReplyCount is set on a top-level comment and counts visible replies only.
	ReplyCount *int `json:"reply_count,omitempty"`

	LikeCount int  `json:"like_count"`
	IsLiked   bool `json:"is_liked"`

	Created           string  `json:"created"`
	Edited            *string `json:"edited"`
	EditedByModerator bool    `json:"edited_by_moderator"`
	Status            int32   `json:"status"`
	Deleted           bool    `json:"deleted"`
	DeleteSource      string  `json:"delete_source,omitempty"`
	Held              bool    `json:"held"`
	PinnedAt          *string `json:"pinned_at"`
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
		PostNumber:        p.PostNumber,
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
	if deleted {
		item.DeleteSource = p.DeleteSource
	}
	if p.ReplyCount != nil {
		n := int(*p.ReplyCount)
		item.ReplyCount = &n
	}
	if p.PinnedAt != "" {
		pinned := p.PinnedAt
		item.PinnedAt = &pinned
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
func (s *Service) Markdown(ctx context.Context, postID int64) (string, error) {
	post, _, err := s.resolvePost(ctx, postID, 0)
	if err != nil {
		return "", err
	}
	if post.Status == communityclient.PostDeleted {
		return "", errCommentNotFound
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

var errCommentNotFound = errors.ErrNotFound("comment not found")

// resolvePost answers a post plus the wall it belongs to, and refuses a post
// whose anchor is not one of moyu's. The tenant guard upstream already 404s
// another site's post; this is what turns a catalog-anchored one — network-wide
// by design, and reachable by id — into the same clean refusal.
func (s *Service) resolvePost(ctx context.Context, postID int64, viewerID int) (*communityclient.PostView, Surface, error) {
	resolved, err := s.community.ResolvePosts(ctx, []int64{postID}, int64(viewerID))
	if err != nil {
		return nil, Surface{}, err
	}
	for i := range resolved.Posts {
		row := resolved.Posts[i]
		if row.Post.ID != postID {
			continue
		}
		surface, ok, err := s.surfaceFor(row.Thread.AnchorKind, row.Thread.AnchorID)
		if err != nil {
			return nil, Surface{}, err
		}
		if !ok {
			return nil, Surface{}, errCommentNotFound
		}
		return &row.Post, surface, nil
	}
	return nil, Surface{}, errCommentNotFound
}

func (s *Service) surfaceFor(anchorKind int32, anchorID string) (Surface, bool, error) {
	targets, err := s.anchors.Resolve([]anchor.Ref{{Kind: anchorKind, ID: anchorID}})
	if err != nil {
		return Surface{}, false, err
	}
	target, ok := targets[anchor.Ref{Kind: anchorKind, ID: anchorID}]
	if !ok {
		return Surface{}, false, nil
	}
	if target.ResourceID > 0 {
		return ResourceSurface(target.ResourceID, target.PatchID), true, nil
	}
	return PatchSurface(target.PatchID), true, nil
}

func nonZero(id int64) *int64 {
	if id == 0 {
		return nil
	}
	return &id
}

// unavailable is the one failure a wall or feed read answers as empty. Every
// other one — moyu's credential, its tenant binding, a malformed request — goes
// back to the handler as a logged 500: this used to render every wall empty
// without a log line, including for a 403 that meant moyu had lost its site.
func unavailable(ctx context.Context, err error, read string) bool {
	if upstream.KindOf(err) != upstream.Unavailable {
		return false
	}
	slog.WarnContext(ctx, "comment: "+read+" answered empty, community unavailable", "error", err)
	return true
}

func (s *Service) brief(ctx context.Context, userID int) *userclient.Brief {
	return userclient.BriefMapByInt(ctx, s.users, []int{userID})[userID]
}

// ResourceRef answers who published a resource and which game it hangs off —
// the two facts a resource wall needs before it can be addressed at all.
func (s *Service) ResourceRef(resourceID int) (*repository.ResourceRef, error) {
	return s.repo.ResourceRef(resourceID)
}
