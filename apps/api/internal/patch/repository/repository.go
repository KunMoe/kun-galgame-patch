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

// ===== Patch CRUD =====

func (r *PatchRepository) CreatePatch(patch *model.Patch) error {
	return r.db.Create(patch).Error
}

func (r *PatchRepository) GetPatchByID(id int) (*model.Patch, error) {
	var patch model.Patch
	err := r.db.First(&patch, id).Error
	return &patch, err
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
	return r.db.Delete(&model.Patch{}, id).Error
}

func (r *PatchRepository) FindPatchByVndbID(vndbID string) (*model.Patch, error) {
	var patch model.Patch
	err := r.db.Where("vndb_id = ?", vndbID).First(&patch).Error
	return &patch, err
}

func (r *PatchRepository) IncrementView(id int) error {
	return r.db.Model(&model.Patch{}).Where("id = ?", id).
		UpdateColumn("view", gorm.Expr("view + 1")).Error
}

func (r *PatchRepository) GetRandomPatchID() (int, error) {
	var id int
	err := r.db.Model(&model.Patch{}).Select("id").Order("RANDOM()").Limit(1).Scan(&id).Error
	return id, err
}

// NOTE: ReplaceAliases is deprecated per D12 (2026-04-21). Aliases are owned by Wiki /galgame/:gid/aliases.

// ===== Comments =====

func (r *PatchRepository) GetComments(patchID, offset, limit int) ([]model.PatchComment, int64, error) {
	var comments []model.PatchComment
	var total int64

	query := r.db.Model(&model.PatchComment{}).Where("galgame_id = ? AND parent_id IS NULL", patchID)
	query.Count(&total)

	err := query.Order("created DESC").Offset(offset).Limit(limit).
		Preload("Replies", func(db *gorm.DB) *gorm.DB {
			return db.Order("created ASC")
		}).
		Find(&comments).Error

	return comments, total, err
}

func (r *PatchRepository) CreateComment(comment *model.PatchComment) error {
	return r.db.Create(comment).Error
}

func (r *PatchRepository) GetCommentByID(id int) (*model.PatchComment, error) {
	var comment model.PatchComment
	err := r.db.First(&comment, id).Error
	return &comment, err
}

func (r *PatchRepository) UpdateComment(comment *model.PatchComment) error {
	return r.db.Save(comment).Error
}

func (r *PatchRepository) DeleteComment(id int) error {
	return r.db.Delete(&model.PatchComment{}, id).Error
}

func (r *PatchRepository) CountCommentAndReplies(commentID int) (int64, error) {
	var count int64
	r.db.Model(&model.PatchComment{}).
		Where("id = ? OR parent_id = ?", commentID, commentID).
		Count(&count)
	return count, nil
}

func (r *PatchRepository) GetCommentMarkdown(commentID int) (string, error) {
	var content string
	err := r.db.Model(&model.PatchComment{}).Where("id = ?", commentID).Pluck("content", &content).Error
	return content, err
}

// ===== Comment Likes =====

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

// ===== Resources =====

func (r *PatchRepository) GetResources(patchID int) ([]model.PatchResource, error) {
	var resources []model.PatchResource
	err := r.db.Where("galgame_id = ?", patchID).
		Order("created DESC").
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

func (r *PatchRepository) UpdateResource(resource *model.PatchResource) error {
	return r.db.Save(resource).Error
}

func (r *PatchRepository) DeleteResource(id int) error {
	return r.db.Delete(&model.PatchResource{}, id).Error
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
		UpdateColumn("status", gorm.Expr("CASE WHEN status = 0 THEN 1 ELSE 0 END")).Error
}

// ===== Resource Likes =====

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

// ===== Favorites =====

func (r *PatchRepository) FindFavorite(userID, patchID int) (*model.UserPatchFavoriteRelation, error) {
	var rel model.UserPatchFavoriteRelation
	err := r.db.Where("user_id = ? AND galgame_id = ?", userID, patchID).First(&rel).Error
	return &rel, err
}

func (r *PatchRepository) CreateFavorite(rel *model.UserPatchFavoriteRelation) error {
	return r.db.Create(rel).Error
}

func (r *PatchRepository) DeleteFavorite(id int) error {
	return r.db.Delete(&model.UserPatchFavoriteRelation{}, id).Error
}

// ===== Contributors =====

// GetContributorIDs returns the user_ids of every contributor on a patch.
// The handler layer batches these via OAuth /users/batch (pkg/userclient)
// to assemble the wire shape; we no longer SELECT name/avatar from the local
// user table because those columns are owned by OAuth (migration 005).
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

// ===== Aggregate Updates =====

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

// ===== User moemoepoint =====

func (r *PatchRepository) UpdateMoemoepoint(userID int, delta int) error {
	return r.db.Table("user").Where("id = ?", userID).
		UpdateColumn("moemoepoint", gorm.Expr("moemoepoint + ?", delta)).Error
}

// ===== Liked resource IDs for a user =====

func (r *PatchRepository) GetLikedResourceIDs(userID int, resourceIDs []int) ([]int, error) {
	var ids []int
	err := r.db.Model(&model.UserPatchResourceLikeRelation{}).
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
