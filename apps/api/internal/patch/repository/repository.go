package repository

import (
	"fmt"

	"kun-galgame-patch-api/internal/patch/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PatchRepository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *PatchRepository {
	return &PatchRepository{db: db}
}

func (r *PatchRepository) CreatePatch(patch *model.Patch) error {
	return r.db.Create(patch).Error
}

func (r *PatchRepository) GetPatchByID(id int) (*model.Patch, error) {
	var patch model.Patch
	err := r.db.First(&patch, id).Error
	return &patch, err
}

func (r *PatchRepository) MarkIndexed(gid int) error {
	return r.db.Model(&model.Patch{}).Where("id = ?", gid).
		Updates(map[string]any{"published": true, "is_stub": false}).Error
}

func (r *PatchRepository) Unpublish(gid int) error {
	return r.db.Model(&model.Patch{}).Where("id = ?", gid).
		Update("published", false).Error
}

func (r *PatchRepository) GetPatchesByIDs(ids []int) ([]model.Patch, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []model.Patch
	if err := r.db.Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	byID := make(map[int]model.Patch, len(rows))
	for _, p := range rows {
		byID[p.ID] = p
	}
	ordered := make([]model.Patch, 0, len(rows))
	for _, id := range ids {
		if p, ok := byID[id]; ok {
			ordered = append(ordered, p)
		}
	}
	return ordered, nil
}

func (r *PatchRepository) GetPatchDetail(id int) (*model.Patch, error) {
	var patch model.Patch
	err := r.db.First(&patch, id).Error
	return &patch, err
}

func (r *PatchRepository) UpdatePatch(patch *model.Patch) error {
	return r.db.Save(patch).Error
}

func (r *PatchRepository) DeletePatch(id int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			`DELETE FROM user_message
			 WHERE link = ?
			    OR link LIKE ?
			    OR link IN (SELECT '/resource/' || id FROM patch_resource WHERE galgame_id = ?)`,
			fmt.Sprintf("/patch/%d", id), fmt.Sprintf("/patch/%d/%%", id), id,
		).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Patch{}, id).Error
	})
}

func (r *PatchRepository) GetPatchLiveArtifactUUIDs(patchID int) ([]string, error) {
	var uuids []string
	err := r.db.Model(&model.PatchResource{}).
		Where("galgame_id = ? AND artifact_uuid <> ''", patchID).
		Pluck("artifact_uuid", &uuids).Error
	return uuids, err
}

func (r *PatchRepository) IncrementView(id int) error {
	return r.db.Model(&model.Patch{}).Where("id = ?", id).
		UpdateColumn("view", gorm.Expr("view + 1")).Error
}

func (r *PatchRepository) GetRandomPatchID(includeEmpty bool) (int, error) {
	var id int
	q := r.db.Model(&model.Patch{}).Select("id")
	if !includeEmpty {
		q = q.Where("resource_count > 0")
	}
	err := q.Order("RANDOM()").Limit(1).Scan(&id).Error
	return id, err
}

func (r *PatchRepository) GetRandomPatchIDs(n int, includeEmpty bool) ([]int, error) {
	if n <= 0 {
		return nil, nil
	}
	var ids []int
	q := r.db.Model(&model.Patch{}).Select("id")
	if !includeEmpty {
		q = q.Where("resource_count > 0")
	}
	err := q.Order("RANDOM()").Limit(n).Scan(&ids).Error
	return ids, err
}

func (r *PatchRepository) GetComments(patchID, offset, limit int) ([]model.PatchComment, int64, error) {
	var comments []model.PatchComment
	var total int64

	base := r.db.Model(&model.PatchComment{}).
		Where("galgame_id = ? AND resource_id IS NULL AND parent_id IS NULL AND status = 0", patchID)
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).Order("created DESC, id DESC").Offset(offset).Limit(limit).
		Preload("Replies", func(db *gorm.DB) *gorm.DB {
			return db.Where("status = 0").Order("created ASC, id ASC")
		}).
		Find(&comments).Error

	return comments, total, err
}

func (r *PatchRepository) GetResourceComments(resourceID, offset, limit int) ([]model.PatchComment, int64, error) {
	var comments []model.PatchComment
	var total int64

	base := r.db.Model(&model.PatchComment{}).
		Where("resource_id = ? AND parent_id IS NULL AND status = 0", resourceID)
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).Order("created DESC, id DESC").Offset(offset).Limit(limit).
		Preload("Replies", func(db *gorm.DB) *gorm.DB {
			return db.Where("status = 0").Order("created ASC, id ASC")
		}).
		Find(&comments).Error

	return comments, total, err
}

func (r *PatchRepository) CountResourceComments(resourceID int) (int64, error) {
	var n int64
	err := r.db.Model(&model.PatchComment{}).
		Where("resource_id = ? AND status = 0", resourceID).
		Count(&n).Error
	return n, err
}

func (r *PatchRepository) CreateComment(comment *model.PatchComment) error {
	return r.db.Create(comment).Error
}

func (r *PatchRepository) UpdateCommentStatus(commentID, status int) error {
	return r.db.Model(&model.PatchComment{}).Where("id = ?", commentID).
		Update("status", status).Error
}

func (r *PatchRepository) GetCommentByID(id int) (*model.PatchComment, error) {
	var comment model.PatchComment
	err := r.db.First(&comment, id).Error
	return &comment, err
}

func (r *PatchRepository) CountRootCommentsBefore(root *model.PatchComment) (int64, error) {
	var n int64
	q := r.db.Model(&model.PatchComment{}).
		Where("parent_id IS NULL AND status = 0").
		Where("(created, id) > (?, ?)", root.Created, root.ID)
	if root.ResourceID != nil {
		q = q.Where("resource_id = ?", *root.ResourceID)
	} else {
		q = q.Where("galgame_id = ? AND resource_id IS NULL", root.GalgameID)
	}
	err := q.Count(&n).Error
	return n, err
}

func (r *PatchRepository) UpdateComment(comment *model.PatchComment) error {
	return r.db.Save(comment).Error
}

func (r *PatchRepository) DeleteComment(id int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			`WITH RECURSIVE doomed AS (
			   SELECT id FROM patch_comment WHERE id = ?
			   UNION
			   SELECT c.id FROM patch_comment c JOIN doomed d ON c.parent_id = d.id
			 )
			 DELETE FROM user_message
			 WHERE link ~ '#comment-[0-9]+$'
			   AND substring(link FROM '#comment-([0-9]+)$')::int IN (SELECT id FROM doomed)`,
			id,
		).Error; err != nil {
			return err
		}
		return tx.Delete(&model.PatchComment{}, id).Error
	})
}

func (r *PatchRepository) CountCommentAndReplies(commentID int) (int64, error) {
	var count int64
	r.db.Model(&model.PatchComment{}).
		Where("(id = ? OR parent_id = ?) AND status = 0", commentID, commentID).
		Count(&count)
	return count, nil
}

func (r *PatchRepository) GetCommentMarkdown(commentID int) (string, error) {
	var content string
	err := r.db.Model(&model.PatchComment{}).Where("id = ?", commentID).Pluck("content", &content).Error
	return content, err
}

func (r *PatchRepository) GetResourcePatchID(resourceID int) (int, error) {
	var patchID int
	err := r.db.Model(&model.PatchResource{}).Where("id = ?", resourceID).
		Pluck("galgame_id", &patchID).Error
	if err != nil {
		return 0, err
	}
	if patchID == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	return patchID, nil
}

func (r *PatchRepository) GetCommentPatchID(commentID int) (int, error) {
	var patchID int
	err := r.db.Model(&model.PatchComment{}).Where("id = ?", commentID).Pluck("galgame_id", &patchID).Error
	if err != nil {
		return 0, err
	}
	if patchID == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	return patchID, nil
}

func (r *PatchRepository) FindCommentLike(userID, commentID int) (*model.UserPatchCommentLikeRelation, error) {
	var rel model.UserPatchCommentLikeRelation
	err := r.db.Where("user_id = ? AND comment_id = ?", userID, commentID).First(&rel).Error
	return &rel, err
}

func (r *PatchRepository) CreateCommentLike(rel *model.UserPatchCommentLikeRelation) error {
	return r.db.Create(rel).Error
}

func (r *PatchRepository) DeleteCommentLike(id int) error {
	return r.db.Delete(&model.UserPatchCommentLikeRelation{}, id).Error
}

func (r *PatchRepository) GetResources(patchID int) ([]model.PatchResource, error) {
	var resources []model.PatchResource
	err := r.db.Where("galgame_id = ? AND status <> 2", patchID).
		Order("created DESC, id DESC").
		Find(&resources).Error
	return resources, err
}

func (r *PatchRepository) CreateResource(resource *model.PatchResource) error {
	return r.db.Create(resource).Error
}

func (r *PatchRepository) GetResourceByID(id int) (*model.PatchResource, error) {
	var resource model.PatchResource
	err := r.db.First(&resource, id).Error
	return &resource, err
}

func (r *PatchRepository) DeleteResource(id int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			"DELETE FROM user_message WHERE link = ? OR link LIKE ?",
			fmt.Sprintf("/resource/%d", id),
			fmt.Sprintf("/resource/%d#%%", id),
		).Error; err != nil {
			return err
		}
		return tx.Delete(&model.PatchResource{}, id).Error
	})
}

func (r *PatchRepository) IncrementResourceDownload(resourceID, patchID int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.PatchResource{}).Where("id = ?", resourceID).
			UpdateColumn("download", gorm.Expr("download + 1")).Error; err != nil {
			return err
		}
		return tx.Model(&model.Patch{}).Where("id = ?", patchID).
			UpdateColumn("download", gorm.Expr("download + 1")).Error
	})
}

func (r *PatchRepository) ToggleResourceStatus(resourceID int) error {
	return r.db.Model(&model.PatchResource{}).Where("id = ?", resourceID).
		UpdateColumn("status", gorm.Expr("CASE WHEN status = 0 THEN 1 WHEN status = 1 THEN 0 ELSE status END")).Error
}

func (r *PatchRepository) SetResourceStatus(id, status int) error {
	return r.db.Model(&model.PatchResource{}).Where("id = ?", id).
		Update("status", status).Error
}

func (r *PatchRepository) RestoreResourceFromModHide(id int) error {
	return r.db.Model(&model.PatchResource{}).Where("id = ? AND status = 2", id).
		Update("status", 0).Error
}

func (r *PatchRepository) FindResourceLike(userID, resourceID int) (*model.UserPatchResourceLikeRelation, error) {
	var rel model.UserPatchResourceLikeRelation
	err := r.db.Where("user_id = ? AND resource_id = ?", userID, resourceID).First(&rel).Error
	return &rel, err
}

func (r *PatchRepository) CreateResourceLike(rel *model.UserPatchResourceLikeRelation) error {
	return r.db.Create(rel).Error
}

func (r *PatchRepository) DeleteResourceLike(id int) error {
	return r.db.Delete(&model.UserPatchResourceLikeRelation{}, id).Error
}

func (r *PatchRepository) FindResourceFavorite(userID, resourceID int) (*model.UserPatchResourceFavoriteRelation, error) {
	var rel model.UserPatchResourceFavoriteRelation
	err := r.db.Where("user_id = ? AND resource_id = ?", userID, resourceID).First(&rel).Error
	return &rel, err
}

func (r *PatchRepository) CreateResourceFavorite(rel *model.UserPatchResourceFavoriteRelation) error {
	return r.db.Create(rel).Error
}

func (r *PatchRepository) DeleteResourceFavorite(id int) error {
	return r.db.Delete(&model.UserPatchResourceFavoriteRelation{}, id).Error
}

func (r *PatchRepository) GetContributorIDs(patchID int) ([]int, error) {
	var ids []int
	err := r.db.Table("user_patch_contribute_relation").
		Where("galgame_id = ?", patchID).
		Pluck("user_id", &ids).Error
	return ids, err
}

func (r *PatchRepository) EnsureContributor(userID, patchID int) error {
	rel := model.UserPatchContributeRelation{UserID: userID, GalgameID: patchID}
	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&rel)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return r.db.Model(&model.Patch{}).Where("id = ?", patchID).
			UpdateColumn("contribute_count", gorm.Expr("contribute_count + 1")).Error
	}
	return nil
}

func (r *PatchRepository) RecalculatePatchAggregates(patchID int) error {
	var resources []model.PatchResource
	r.db.Where("galgame_id = ?", patchID).Select("type", "language", "platform").Find(&resources)

	typeSet := make(map[string]bool)
	langSet := make(map[string]bool)
	platSet := make(map[string]bool)
	for _, res := range resources {
		for _, t := range res.Type {
			typeSet[t] = true
		}
		for _, l := range res.Language {
			langSet[l] = true
		}
		for _, p := range res.Platform {
			platSet[p] = true
		}
	}

	return r.db.Model(&model.Patch{}).Where("id = ?", patchID).Updates(map[string]any{
		"type":     model.JSONArray(setToSlice(typeSet)),
		"language": model.JSONArray(setToSlice(langSet)),
		"platform": model.JSONArray(setToSlice(platSet)),
	}).Error
}

func (r *PatchRepository) UpdateCount(patchID int, field string, delta int) error {
	expr := fmt.Sprintf("%s + %d", field, delta)
	if delta < 0 {
		expr = fmt.Sprintf("GREATEST(%s + %d, 0)", field, delta)
	}
	return r.db.Model(&model.Patch{}).Where("id = ?", patchID).
		UpdateColumn(field, gorm.Expr(expr)).Error
}

func (r *PatchRepository) GetLikedResourceIDs(userID int, resourceIDs []int) ([]int, error) {
	var ids []int
	err := r.db.Model(&model.UserPatchResourceLikeRelation{}).
		Where("user_id = ? AND resource_id IN ?", userID, resourceIDs).
		Pluck("resource_id", &ids).Error
	return ids, err
}

func (r *PatchRepository) GetFavoritedResourceIDs(userID int, resourceIDs []int) ([]int, error) {
	var ids []int
	err := r.db.Model(&model.UserPatchResourceFavoriteRelation{}).
		Where("user_id = ? AND resource_id IN ?", userID, resourceIDs).
		Pluck("resource_id", &ids).Error
	return ids, err
}

func (r *PatchRepository) GetLikedCommentIDs(userID int, commentIDs []int) ([]int, error) {
	var ids []int
	err := r.db.Model(&model.UserPatchCommentLikeRelation{}).
		Where("user_id = ? AND comment_id IN ?", userID, commentIDs).
		Pluck("comment_id", &ids).Error
	return ids, err
}

func setToSlice(s map[string]bool) []string {
	result := make([]string, 0, len(s))
	for k := range s {
		result = append(result, k)
	}
	return result
}

func (r *PatchRepository) GetResourceRevisions(resourceID, offset, limit int) ([]model.PatchResourceRevision, int64, error) {
	var rows []model.PatchResourceRevision
	var total int64
	base := r.db.Model(&model.PatchResourceRevision{}).Where("resource_id = ?", resourceID)
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).
		Order("created_at DESC, id DESC").
		Offset(offset).Limit(limit).
		Find(&rows).Error
	return rows, total, err
}
