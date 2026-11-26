package service

import (
	"context"

	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/userclient"
)

// SetPinned pins a top-level comment on its wall, or unpins it. Pins are moyu's
// own: on a wall shared with another site, each site pins its own.
func (s *Service) SetPinned(ctx context.Context, postID int64, actorID int, pinned bool) (*Item, error) {
	_, surface, err := s.resolvePost(ctx, postID, 0)
	if err != nil {
		return nil, err
	}
	set, action := s.community.PinPost, "pinComment"
	if !pinned {
		set, action = s.community.UnpinPost, "unpinComment"
	}
	res, err := set(ctx, postID, int64(actorID))
	if communityclient.RefusalOf(err) == communityclient.RefusalPinMoved {
		res, err = set(ctx, postID, int64(actorID))
	}
	if err != nil {
		return nil, err
	}
	if res.Changed && s.audit != nil {
		_ = s.audit.CreateLog(actorID, action, map[string]any{
			"post_id": postID, "owner_id": res.Post.AuthorID, "galgame_id": surface.PatchID,
		})
	}
	return s.rendered(ctx, surface, res.Post), nil
}

// Restore undoes a removal by a moderator or by review, back to the status the
// post had before: one a moderator deleted while it was held comes back held.
// An author's own delete, a purge and a trust removal are refused upstream.
func (s *Service) Restore(ctx context.Context, postID int64, actorID int) (*Item, error) {
	before, surface, err := s.resolvePost(ctx, postID, 0)
	if err != nil {
		return nil, err
	}
	res, err := s.community.RestorePost(ctx, postID, int64(actorID))
	if err != nil {
		return nil, err
	}
	if res.Changed {
		// Only moyu's own moderator delete stepped the counter down; a review
		// removal never reached it, and adding one back for that would inflate it.
		if before.DeleteSource == communityclient.DeletedByModerator {
			s.repo.BumpCommentCount(surface.PatchID, 1)
		}
		owner := int(res.Post.AuthorID)
		if owner != actorID {
			notice := "您发布的评论已被版主恢复。"
			if res.Post.Status == communityclient.PostHeld {
				notice = "您发布的评论已被版主恢复，目前仍在审核中，暂时仅您可见。"
			}
			s.notifyModeratorAction(owner, notice, s.postLink(surface, postID))
		}
		if s.audit != nil {
			_ = s.audit.CreateLog(actorID, "restoreComment", map[string]any{
				"post_id": postID, "owner_id": owner, "galgame_id": surface.PatchID,
				"delete_source": before.DeleteSource,
			})
		}
	}
	return s.rendered(ctx, surface, res.Post), nil
}

// rendered answers the post an action returned even when the actor could not
// read it on the wall: a restored post can come back held.
func (s *Service) rendered(ctx context.Context, surface Surface, post communityclient.PostView) *Item {
	uids := []int{int(post.AuthorID)}
	if post.TargetUserID != 0 {
		uids = append(uids, int(post.TargetUserID))
	}
	briefs := userclient.BriefMapByInt(ctx, s.users, uids)
	item := buildItem(post, surface, patchModel.NewPatchUser(briefs[int(post.AuthorID)]))
	if post.TargetUserID != 0 {
		item.TargetUser = patchModel.NewPatchUser(briefs[int(post.TargetUserID)])
	}
	return item
}
