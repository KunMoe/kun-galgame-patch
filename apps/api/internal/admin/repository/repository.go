package repository

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"time"

	adminModel "kun-galgame-patch-api/internal/admin/model"
	authModel "kun-galgame-patch-api/internal/auth/model"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	userModel "kun-galgame-patch-api/internal/user/model"

	"gorm.io/gorm"
)

var ErrUserOwnsPatches = stderrors.New("user still owns patches")

type AdminRepository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

func (r *AdminRepository) GetComments(search, status string, offset, limit int) ([]patchModel.PatchComment, int64, error) {
	var comments []patchModel.PatchComment
	var total int64

	base := r.db.Model(&patchModel.PatchComment{})
	if search != "" {
		base = base.Where("content ILIKE ?", "%"+search+"%")
	}
	switch status {
	case "pending":
		base = base.Where("status <> 0")
	case "approved":
		base = base.Where("status = 0")
	}
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).Order("created DESC, id DESC").Offset(offset).Limit(limit).
		Find(&comments).Error
	return comments, total, err
}

func (r *AdminRepository) UpdateComment(commentID int, content string) error {
	return r.db.Model(&patchModel.PatchComment{}).Where("id = ?", commentID).
		Update("content", content).Error
}

func (r *AdminRepository) GetResources(search string, offset, limit int) ([]patchModel.PatchResource, int64, error) {
	var resources []patchModel.PatchResource
	var total int64

	base := r.db.Model(&patchModel.PatchResource{})
	if search != "" {
		base = base.Where("name ILIKE ? OR content ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).Order("created DESC, id DESC").Offset(offset).Limit(limit).
		Find(&resources).Error
	return resources, total, err
}

func (r *AdminRepository) UpdateResource(resourceID int, note string) error {
	return r.db.Model(&patchModel.PatchResource{}).Where("id = ?", resourceID).
		Update("note", note).Error
}

func (r *AdminRepository) DeleteResource(resourceID int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			"DELETE FROM user_message WHERE link = ?",
			fmt.Sprintf("/resource/%d", resourceID),
		).Error; err != nil {
			return err
		}
		return tx.Delete(&patchModel.PatchResource{}, resourceID).Error
	})
}

func (r *AdminRepository) GetStats(since time.Time) (newUser, newActive, newGalgame, newResource, newComment int64) {
	r.db.Model(&authModel.User{}).Where("created >= ?", since).Count(&newUser)
	r.db.Model(&authModel.User{}).Where("last_login_time >= ?", since.Format(time.RFC3339)).Count(&newActive)
	r.db.Model(&patchModel.Patch{}).Where("created >= ?", since).Count(&newGalgame)
	r.db.Model(&patchModel.PatchResource{}).Where("created >= ?", since).Count(&newResource)
	r.db.Model(&patchModel.PatchComment{}).Where("created >= ?", since).Count(&newComment)
	return
}

func (r *AdminRepository) GetStatsSum() (userCount, galgameCount, resourceCount, commentCount int64) {
	r.db.Model(&authModel.User{}).Count(&userCount)
	r.db.Model(&patchModel.Patch{}).Count(&galgameCount)
	r.db.Model(&patchModel.PatchResource{}).Count(&resourceCount)
	r.db.Model(&patchModel.PatchComment{}).Count(&commentCount)
	return
}

func (r *AdminRepository) GetResourceFileHistory(
	resourceID, offset, limit int,
) ([]patchModel.PatchResourceFileHistory, int64, error) {
	var rows []patchModel.PatchResourceFileHistory
	var total int64

	base := r.db.Model(&patchModel.PatchResourceFileHistory{}).
		Where("resource_id = ?", resourceID)
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).
		Order("created_at DESC, id DESC").
		Offset(offset).Limit(limit).
		Find(&rows).Error
	return rows, total, err
}

func (r *AdminRepository) GetLogs(offset, limit int) ([]adminModel.AdminLog, int64, error) {
	var logs []adminModel.AdminLog
	var total int64

	base := r.db.Model(&adminModel.AdminLog{})
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).Order("created DESC, id DESC").Offset(offset).Limit(limit).
		Find(&logs).Error
	return logs, total, err
}

func (r *AdminRepository) CreateLog(adminUID int, logType string, data any) error {
	content, _ := json.Marshal(data)
	log := &adminModel.AdminLog{
		Type:    logType,
		Content: string(content),
		UserID:  adminUID,
	}
	return r.db.Create(log).Error
}

func (r *AdminRepository) GetAllPatches(search string, offset, limit int) ([]patchModel.Patch, int64, error) {
	var patches []patchModel.Patch
	var total int64

	base := r.db.Model(&patchModel.Patch{})
	if search != "" {
		base = base.Where("vndb_id ILIKE ?", "%"+search+"%")
	}
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).Order("created DESC, id DESC").Offset(offset).Limit(limit).
		Find(&patches).Error
	return patches, total, err
}

func (r *AdminRepository) LookupPatchesByIDs(ids []int) ([]patchModel.Patch, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []patchModel.Patch
	err := r.db.Select("id", "vndb_id").Where("id IN ?", ids).Find(&rows).Error
	return rows, err
}

const orphanCond = "vndb_id !~ '^v[0-9]+$'"

func (r *AdminRepository) GetOrphanCandidateIDs() ([]int, error) {
	var ids []int
	err := r.db.Model(&patchModel.Patch{}).Where(orphanCond).Pluck("id", &ids).Error
	return ids, err
}

func (r *AdminRepository) GetOrphanPatches(offset, limit int, excludeIDs []int) ([]patchModel.Patch, int64, error) {
	var patches []patchModel.Patch
	var total int64
	base := r.db.Model(&patchModel.Patch{}).Where(orphanCond)
	if len(excludeIDs) > 0 {
		base = base.Where("id NOT IN ?", excludeIDs)
	}
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).
		Order("resource_count DESC, comment_count DESC, favorite_count DESC, id ASC").
		Offset(offset).Limit(limit).
		Find(&patches).Error
	return patches, total, err
}

func (r *AdminRepository) CountOrphanPatches(excludeIDs []int) (pendingCount, badVndbCount int64, err error) {
	pend := r.db.Model(&patchModel.Patch{}).Where("vndb_id LIKE 'pending-%'")
	bad := r.db.Model(&patchModel.Patch{}).Where(orphanCond + " AND vndb_id NOT LIKE 'pending-%'")
	if len(excludeIDs) > 0 {
		pend = pend.Where("id NOT IN ?", excludeIDs)
		bad = bad.Where("id NOT IN ?", excludeIDs)
	}
	if err = pend.Count(&pendingCount).Error; err != nil {
		return
	}
	err = bad.Count(&badVndbCount).Error
	return
}

type PurgePreviewCounts struct {
	UserExists          bool
	Comments            int64
	Resources           int64
	CommentLikes        int64
	ResourceLikes       int64
	Contributes         int64
	Following           int64
	Followers           int64
	ChatMemberships     int64
	ChatMessages        int64
	PrivateMessages     int64
	OwnedPatches        int64
	OwnedPatchResources int64
	OwnedPatchComments  int64
	MiscTraces          int64
}

func (r *AdminRepository) ownedPatchIDsSubquery(userID int) *gorm.DB {
	return r.db.Model(&patchModel.Patch{}).Select("id").Where("user_id = ?", userID)
}

func (r *AdminRepository) PurgePreview(userID int, includeOwnedPatches bool) (*PurgePreviewCounts, error) {
	var c PurgePreviewCounts

	var userCount int64
	if err := r.db.Model(&authModel.User{}).Where("id = ?", userID).Count(&userCount).Error; err != nil {
		return nil, err
	}
	c.UserExists = userCount > 0

	var firstErr error
	count := func(dst *int64, q *gorm.DB) {
		if firstErr != nil {
			return
		}
		if err := q.Count(dst).Error; err != nil {
			firstErr = err
		}
	}

	count(&c.Comments, r.db.Model(&patchModel.PatchComment{}).Where("user_id = ?", userID))
	count(&c.Resources, r.db.Model(&patchModel.PatchResource{}).Where("user_id = ?", userID))
	count(&c.CommentLikes, r.db.Model(&patchModel.UserPatchCommentLikeRelation{}).Where("user_id = ?", userID))
	count(&c.ResourceLikes, r.db.Model(&patchModel.UserPatchResourceLikeRelation{}).Where("user_id = ?", userID))
	count(&c.Contributes, r.db.Model(&patchModel.UserPatchContributeRelation{}).Where("user_id = ?", userID))
	count(&c.Following, r.db.Model(&userModel.UserFollowRelation{}).Where("follower_id = ?", userID))
	count(&c.Followers, r.db.Model(&userModel.UserFollowRelation{}).Where("following_id = ?", userID))
	count(&c.ChatMemberships, r.db.Table("chat_member").Where("user_id = ?", userID))
	count(&c.ChatMessages, r.db.Table("chat_message").Where("sender_id = ?", userID))
	count(&c.PrivateMessages, r.db.Table("user_message").Where("sender_id = ? OR recipient_id = ?", userID, userID))
	count(&c.OwnedPatches, r.db.Model(&patchModel.Patch{}).Where("user_id = ?", userID))

	var fileHistory int64
	count(&fileHistory, r.db.Table("patch_resource_file_history").Where("actor_id = ?", userID))
	c.MiscTraces = fileHistory

	if includeOwnedPatches {
		count(&c.OwnedPatchResources, r.db.Model(&patchModel.PatchResource{}).Where("galgame_id IN (?)", r.ownedPatchIDsSubquery(userID)))
		count(&c.OwnedPatchComments, r.db.Model(&patchModel.PatchComment{}).Where("galgame_id IN (?)", r.ownedPatchIDsSubquery(userID)))
	}

	if firstErr != nil {
		return nil, firstErr
	}
	return &c, nil
}

func (r *AdminRepository) CollectUserArtifactUUIDs(userID int, includeOwnedPatches bool) ([]string, error) {
	seen := make(map[string]struct{})
	add := func(uuids []string) {
		for _, u := range uuids {
			if u != "" {
				seen[u] = struct{}{}
			}
		}
	}
	var own []string
	if err := r.db.Model(&patchModel.PatchResource{}).
		Where("user_id = ? AND artifact_uuid <> ''", userID).
		Pluck("artifact_uuid", &own).Error; err != nil {
		return nil, err
	}
	add(own)
	if includeOwnedPatches {
		var op []string
		if err := r.db.Model(&patchModel.PatchResource{}).
			Where("galgame_id IN (?) AND artifact_uuid <> ''", r.ownedPatchIDsSubquery(userID)).
			Pluck("artifact_uuid", &op).Error; err != nil {
			return nil, err
		}
		add(op)
	}
	out := make([]string, 0, len(seen))
	for u := range seen {
		out = append(out, u)
	}
	return out, nil
}

func (r *AdminRepository) PurgeUser(userID int, purgeOwnedPatches bool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var ownedPatches int64
		if err := tx.Model(&patchModel.Patch{}).Where("user_id = ?", userID).Count(&ownedPatches).Error; err != nil {
			return err
		}
		if ownedPatches > 0 && !purgeOwnedPatches {
			return ErrUserOwnsPatches
		}

		distinctInts := func(table, col, where string, args ...any) ([]int, error) {
			var ids []int
			err := tx.Table(table).Where(where, args...).Distinct().Pluck(col, &ids).Error
			return ids, err
		}
		pc, err := distinctInts("patch_comment", "galgame_id", "user_id = ?", userID)
		if err != nil {
			return err
		}
		pr, err := distinctInts("patch_resource", "galgame_id", "user_id = ?", userID)
		if err != nil {
			return err
		}
		pco, err := distinctInts("user_patch_contribute_relation", "galgame_id", "user_id = ?", userID)
		if err != nil {
			return err
		}
		affectedPatchIDs := unionInts(pc, pr, pco)

		likedCommentIDs, err := distinctInts("user_patch_comment_like_relation", "comment_id", "user_id = ?", userID)
		if err != nil {
			return err
		}
		likedResourceIDs, err := distinctInts("user_patch_resource_like_relation", "resource_id", "user_id = ?", userID)
		if err != nil {
			return err
		}
		followingPeers, err := distinctInts("user_follow_relation", "following_id", "follower_id = ?", userID)
		if err != nil {
			return err
		}
		followerPeers, err := distinctInts("user_follow_relation", "follower_id", "following_id = ?", userID)
		if err != nil {
			return err
		}
		peers := unionInts(followingPeers, followerPeers)

		if err := tx.Where("follower_id = ? OR following_id = ?", userID, userID).
			Delete(&userModel.UserFollowRelation{}).Error; err != nil {
			return err
		}

		if err := tx.Where("actor_id = ?", userID).
			Delete(&patchModel.PatchResourceFileHistory{}).Error; err != nil {
			return err
		}

		if err := tx.Exec(
			`DELETE FROM user_message
			 WHERE link IN (SELECT '/resource/' || id FROM patch_resource WHERE user_id = ?)`,
			userID,
		).Error; err != nil {
			return err
		}

		if purgeOwnedPatches && ownedPatches > 0 {
			if err := tx.Exec(
				`DELETE FROM user_message m
				 USING patch p
				 WHERE p.user_id = ?
				   AND (m.link = '/patch/' || p.id OR m.link LIKE '/patch/' || p.id || '/%')`,
				userID,
			).Error; err != nil {
				return err
			}
			if err := tx.Exec(
				`DELETE FROM user_message
				 WHERE link IN (
				   SELECT '/resource/' || r.id FROM patch_resource r
				   JOIN patch p ON p.id = r.galgame_id WHERE p.user_id = ?
				 )`,
				userID,
			).Error; err != nil {
				return err
			}
			if err := tx.Where("user_id = ?", userID).Delete(&patchModel.Patch{}).Error; err != nil {
				return err
			}
		}

		if err := tx.Delete(&authModel.User{}, userID).Error; err != nil {
			return err
		}

		// favorite_count is deliberately not recounted here. Favourites live in
		// catalog folders since the 2026-09-07 cutover and the local counter is
		// kept by settleFavoriteSideEffects; recomputing it from the frozen
		// user_patch_favorite_relation rolled every affected game back to its
		// snapshot value and threw away every heart since.
		if len(affectedPatchIDs) > 0 {
			if err := tx.Exec(`UPDATE patch SET
				comment_count    = (SELECT COUNT(*) FROM patch_comment WHERE patch_comment.galgame_id = patch.id),
				resource_count   = (SELECT COUNT(*) FROM patch_resource WHERE patch_resource.galgame_id = patch.id),
				contribute_count = (SELECT COUNT(*) FROM user_patch_contribute_relation WHERE user_patch_contribute_relation.galgame_id = patch.id)
				WHERE id IN ?`, affectedPatchIDs).Error; err != nil {
				return err
			}
		}
		if len(likedCommentIDs) > 0 {
			if err := tx.Exec(`UPDATE patch_comment SET like_count =
				(SELECT COUNT(*) FROM user_patch_comment_like_relation WHERE user_patch_comment_like_relation.comment_id = patch_comment.id)
				WHERE id IN ?`, likedCommentIDs).Error; err != nil {
				return err
			}
		}
		if len(likedResourceIDs) > 0 {
			if err := tx.Exec(`UPDATE patch_resource SET like_count =
				(SELECT COUNT(*) FROM user_patch_resource_like_relation WHERE user_patch_resource_like_relation.resource_id = patch_resource.id)
				WHERE id IN ?`, likedResourceIDs).Error; err != nil {
				return err
			}
		}
		if len(peers) > 0 {
			if err := tx.Exec(`UPDATE "user" SET
				follower_count  = (SELECT COUNT(*) FROM user_follow_relation WHERE user_follow_relation.following_id = "user".id),
				following_count = (SELECT COUNT(*) FROM user_follow_relation WHERE user_follow_relation.follower_id = "user".id)
				WHERE id IN ?`, peers).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func unionInts(slices ...[]int) []int {
	seen := make(map[int]struct{})
	for _, s := range slices {
		for _, v := range s {
			seen[v] = struct{}{}
		}
	}
	out := make([]int, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	return out
}
