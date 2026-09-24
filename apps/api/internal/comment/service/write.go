package service

import (
	"context"
	stderrors "errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"kun-galgame-patch-api/internal/infrastructure/markdown"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/upstream"
)

type LikeResult struct {
	Liked     bool `json:"liked"`
	LikeCount int  `json:"like_count"`
}

// Create posts to a wall. The thread is created upstream in the same
// transaction when this is the wall's first comment, which is why the reply
// path and the first comment are the same call.
//
// submitKey is the page's id for one press of 发布, kept across its retries.
// Community scopes an idempotency key to the whole site, so the page's value is
// never forwarded as is: under another author's id it would replay their post.
// No submitKey sends no key; one derived from the text would merge two comments
// a reader meant to post twice.
func (s *Service) Create(ctx context.Context, surface Surface, userID int, content string, replyToPostID *int64, submitKey string) (*Item, error) {
	content = markdown.NormalizeContentImageURLs(content)

	req := communityclient.CommentRequest{
		AnchorKind:     surface.AnchorKind,
		AnchorID:       surface.AnchorID,
		ContentRating:  communityclient.RatingAll,
		AuthorID:       int64(userID),
		Body:           content,
		MentionUserIDs: mentionUserIDs(content, userID),
	}

	if replyToPostID != nil && *replyToPostID > 0 {
		parent, parentSurface, err := s.resolvePost(ctx, *replyToPostID, 0)
		if stderrors.Is(err, errCommentNotFound) {
			return nil, errors.ErrBadRequest("parent comment not found")
		}
		if err != nil {
			return nil, err
		}
		// A post id is global and this site has two walls, so a reply has to be
		// proved to be on THIS one — otherwise a crafted id hangs a comment off a
		// conversation the reader is not looking at.
		if parentSurface.AnchorID != surface.AnchorID {
			return nil, errors.ErrBadRequest("parent comment belongs to another discussion")
		}
		req.ReplyToPostID = parent.ID
		req.TargetUserID = parent.AuthorID
	}

	key := ""
	if submitKey != "" {
		key = upstream.IdempotencyKey(strconv.Itoa(userID), "comment", submitKey)
	}
	res, err := s.community.CommentOnAnchor(ctx, req, key)
	if err != nil {
		return nil, err
	}

	if !res.Replayed {
		s.afterCreate(ctx, surface, userID, &res.Post)
	}
	s.inbox.MarkThreadRead(userID, res.Thread.ID, res.Post.PostNumber)

	item := buildItem(res.Post, surface, patchModel.NewPatchUser(s.brief(ctx, userID)))
	item.ThreadID = res.Thread.ID
	return item, nil
}

// afterCreate is everything moyu still owns once the post is committed upstream:
// the cached counters, the contributor row and the moemoepoint award. All of it
// is best-effort — the comment is already published and a failed counter must
// not fail the request.
func (s *Service) afterCreate(ctx context.Context, surface Surface, userID int, post *communityclient.PostView) {
	s.repo.BumpCommentCount(surface.PatchID, 1)
	s.repo.EnsureContributor(userID, surface.PatchID)

	if owner := s.repo.PatchOwner(surface.PatchID); owner != 0 && owner != userID {
		// The idempotency key is namespaced on the POST id, not the comment id the
		// pre-cutover awards used: the two id spaces overlap, and a new post whose
		// id equals an old comment id would silently dedupe against that award.
		s.mp.Award(ctx, owner, 1, "liked",
			fmt.Sprintf("comment:%d", post.ID),
			fmt.Sprintf("moyu:comment_post:%d", post.ID))
	}
}

// Update edits a post. A moderator edit is declared to community with
// as_moderator so the post carries the "edited by a moderator" bit; an author
// editing their own post never sets it.
func (s *Service) Update(ctx context.Context, postID int64, userID int, isModerator bool, content, reason string) (*Item, error) {
	post, surface, err := s.resolvePost(ctx, postID, 0)
	if err != nil {
		return nil, err
	}
	asModerator := isModerator && post.AuthorID != int64(userID)

	content = markdown.NormalizeContentImageURLs(content)
	updated, err := s.community.EditPost(ctx, postID, communityclient.EditPostRequest{
		AuthorID: int64(userID), Body: content, AsModerator: asModerator,
	})
	if err != nil {
		return nil, err
	}

	link := s.postLink(surface, postID)
	if asModerator {
		// No mention pass here: a moderator's save sent every mention the author
		// wrote out again, in the moderator's name.
		s.notifyModeratorAction(int(updated.AuthorID), moderatorNotice("编辑", reason), link)
		if s.audit != nil {
			_ = s.audit.CreateLog(userID, "updateComment", map[string]any{
				"post_id": postID, "owner_id": updated.AuthorID, "galgame_id": surface.PatchID, "reason": reason,
			})
		}
	} else {
		// Community has no edit-time mention notice. Diffing against the pre-edit
		// body is what keeps this from duplicating the one community sent when
		// the post was created.
		s.notifyMentions(userID, addedMentionIDs(post.ContentRaw, content, userID), content, link)
	}

	return buildItem(*updated, surface, patchModel.NewPatchUser(s.brief(ctx, int(updated.AuthorID)))), nil
}

// Delete tombstones a post. Community keeps its post_number so the wall's
// numbering never collapses, which is why the reader sees a stub rather than a
// gap.
func (s *Service) Delete(ctx context.Context, postID int64, userID int, isModerator bool, reason string) error {
	post, surface, err := s.resolvePost(ctx, postID, 0)
	if err != nil {
		return err
	}
	owner := int(post.AuthorID)
	asModerator := isModerator && owner != userID
	if owner != userID && !asModerator {
		return errors.ErrForbidden()
	}

	if err := s.community.DeletePost(ctx, postID, int64(userID), asModerator); err != nil {
		return err
	}

	// One post, one counter step: a tombstoned reply keeps its own number and its
	// children are not removed with it, so there is nothing to subtract but this.
	s.repo.BumpCommentCount(surface.PatchID, -1)

	if asModerator {
		s.notifyModeratorAction(owner, moderatorNotice("删除", reason), wallLink(surface))
		if s.audit != nil && userID != 0 {
			_ = s.audit.CreateLog(userID, "deleteComment", map[string]any{
				"post_id": postID, "owner_id": owner, "galgame_id": surface.PatchID, "reason": reason,
			})
		}
	}
	return nil
}

// SetLike puts the reader's like in the state they asked for. It was a toggle,
// and a click retried after a timeout undid itself. The count it answers is the
// one community returns: community is authoritative for both, and a second read
// to confirm its own write buys nothing.
func (s *Service) SetLike(ctx context.Context, postID int64, userID int, liked bool) (*LikeResult, error) {
	// Resolved BEFORE the write, not after. A post id is global, so a crafted id
	// would otherwise have its reaction written upstream and only then be refused
	// here.
	if _, _, err := s.resolvePost(ctx, postID, userID); err != nil {
		return nil, err
	}

	set := s.community.SetReaction
	if !liked {
		set = s.community.UnsetReaction
	}
	res, err := set(ctx, postID, int64(userID), communityclient.ReactionLike)
	if err != nil {
		return nil, err
	}

	if res.Changed && res.AuthorID != int64(userID) {
		// Keyed on the pair, because the like row that used to key it lived in a
		// local mirror this site no longer keeps. The cost is that liking again
		// after an unlike does not award again; the alternative — a key carrying
		// the attempt — is not an idempotency key at all.
		delta, event := 1, "comment_like"
		if !res.Added {
			delta, event = -1, "comment_unlike"
		}
		s.mp.Award(ctx, int(res.AuthorID), delta, "liked",
			fmt.Sprintf("comment:%d", postID),
			fmt.Sprintf("moyu:%s:%d:%d", event, postID, userID))
	}

	return &LikeResult{Liked: res.Added, LikeCount: int(res.ReactionCount)}, nil
}

// Flag reports a post. The weight a report carries is the reporter's, computed
// upstream from their trust level and their past accuracy; enough weight hides
// the post and opens a review item. moyu decides nothing here.
func (s *Service) Flag(ctx context.Context, postID int64, userID int, reason int32, note string) error {
	if reason < communityclient.FlagReasonSpam || reason > communityclient.FlagReasonNsfwMislabel {
		return errors.ErrValidation("非法的举报理由")
	}
	if _, _, err := s.resolvePost(ctx, postID, 0); err != nil {
		return err
	}
	return s.community.SubmitFlag(ctx, postID, communityclient.FlagRequest{
		FlaggerID: int64(userID), Reason: reason, Note: note,
	})
}

func wallLink(surface Surface) string {
	if surface.ResourceID != 0 {
		return fmt.Sprintf("/resource/%d", surface.ResourceID)
	}
	return fmt.Sprintf("/galgame/%d?tab=comment", surface.PatchID)
}

func (s *Service) postLink(surface Surface, postID int64) string {
	return wallLink(surface) + fmt.Sprintf("#post-%d", postID)
}

func moderatorNotice(action, reason string) string {
	content := "您发布的评论已被版主" + action + "。"
	if reason != "" {
		return content + "原因：" + reason
	}
	return content + "如有疑问可联系管理员。"
}

func (s *Service) notifyModeratorAction(ownerID int, content, link string) {
	if err := s.db.Table("user_message").Create(map[string]any{
		"type": "system", "content": content, "status": 0, "link": link,
		"sender_id": nil, "recipient_id": ownerID,
		"created": time.Now(), "updated": time.Now(),
	}).Error; err != nil {
		slog.Warn("comment: moderator-action notice insert failed",
			"owner", ownerID, "error", err)
	}
}

const maxMentionUserIDs = 20

func mentionUserIDs(content string, authorID int) []int64 {
	ids := markdown.ExtractMentionedUserIDs(content)
	out := make([]int64, 0, min(len(ids), maxMentionUserIDs))
	for _, id := range ids {
		if id == authorID {
			continue
		}
		out = append(out, int64(id))
		if len(out) == maxMentionUserIDs {
			break
		}
	}
	return out
}

func addedMentionIDs(oldBody, newBody string, authorID int) []int {
	had := make(map[int]struct{})
	for _, id := range markdown.ExtractMentionedUserIDs(oldBody) {
		had[id] = struct{}{}
	}
	var added []int
	for _, id := range markdown.ExtractMentionedUserIDs(newBody) {
		if id == authorID {
			continue
		}
		if _, ok := had[id]; ok {
			continue
		}
		had[id] = struct{}{}
		added = append(added, id)
	}
	return added
}

func (s *Service) notifyMentions(senderID int, ids []int, content, link string) {
	excerpt := truncate(content, 233)
	for _, uid := range ids {
		s.notifyDedup(senderID, uid, "mention", excerpt, link)
	}
}

func (s *Service) notifyDedup(senderID, recipientID int, msgType, content, link string) {
	if recipientID <= 0 || recipientID == senderID {
		return
	}
	var count int64
	s.db.Table("user_message").
		Where("type = ? AND sender_id = ? AND recipient_id = ? AND link = ?",
			msgType, senderID, recipientID, link).
		Count(&count)
	if count > 0 {
		return
	}
	if err := s.db.Table("user_message").Create(map[string]any{
		"type": msgType, "content": content, "status": 0, "link": link,
		"sender_id": senderID, "recipient_id": recipientID,
		"created": time.Now(), "updated": time.Now(),
	}).Error; err != nil {
		slog.Warn("comment: notification insert failed (best-effort)",
			"type", msgType, "recipient", recipientID, "link", link, "error", err)
	}
}

func truncate(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes])
}
