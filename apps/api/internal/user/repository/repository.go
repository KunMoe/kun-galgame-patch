package repository

import (
	authModel "kun-galgame-patch-api/internal/auth/model"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/internal/user/model"

	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// ===== User Info =====

func (r *UserRepository) FindByID(id int) (*authModel.User, error) {
	var user authModel.User
	err := r.db.First(&user, id).Error
	return &user, err
}

func (r *UserRepository) UpdateFields(userID int, fields map[string]any) error {
	return r.db.Model(&authModel.User{}).Where("id = ?", userID).Updates(fields).Error
}

func (r *UserRepository) CountUserPatches(userID int) int64 {
	var count int64
	r.db.Model(&patchModel.Patch{}).Where("user_id = ?", userID).Count(&count)
	return count
}

func (r *UserRepository) CountUserResources(userID int) int64 {
	var count int64
	r.db.Model(&patchModel.PatchResource{}).Where("user_id = ?", userID).Count(&count)
	return count
}

func (r *UserRepository) CountUserComments(userID int) int64 {
	var count int64
	r.db.Model(&patchModel.PatchComment{}).Where("user_id = ?", userID).Count(&count)
	return count
}

func (r *UserRepository) CountUserFavorites(userID int) int64 {
	var count int64
	r.db.Model(&patchModel.UserPatchFavoriteRelation{}).Where("user_id = ?", userID).Count(&count)
	return count
}

// ===== User Profile Lists =====

// All list helpers below split Count and Find onto independent statements via
// .Session(&gorm.Session{}). Reusing one chained *gorm.DB across Count then
// Find is the gorm v2 footgun that broke /message: Count leaves SELECT
// count(*) on the shared statement, so the follow-up Find returns the count
// row instead of the rows. See message/repository.go GetMessages.

func (r *UserRepository) GetUserPatches(userID, offset, limit int) ([]patchModel.Patch, int64, error) {
	var patches []patchModel.Patch
	var total int64
	base := r.db.Model(&patchModel.Patch{}).Where("user_id = ?", userID)
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).Order("created DESC").Offset(offset).Limit(limit).Find(&patches).Error
	return patches, total, err
}

func (r *UserRepository) GetUserResources(userID, offset, limit int) ([]patchModel.PatchResource, int64, error) {
	var resources []patchModel.PatchResource
	var total int64
	base := r.db.Model(&patchModel.PatchResource{}).Where("user_id = ?", userID)
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).Order("created DESC").Offset(offset).Limit(limit).Find(&resources).Error
	return resources, total, err
}

func (r *UserRepository) GetUserFavorites(userID, offset, limit int) ([]patchModel.Patch, int64, error) {
	var patches []patchModel.Patch
	var total int64
	subQuery := r.db.Table("user_patch_favorite_relation").Where("user_id = ?", userID).Select("galgame_id")
	base := r.db.Model(&patchModel.Patch{}).Where("id IN (?)", subQuery)
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).Order("created DESC").Offset(offset).Limit(limit).Find(&patches).Error
	return patches, total, err
}

func (r *UserRepository) GetUserComments(userID, offset, limit int) ([]patchModel.PatchComment, int64, error) {
	var comments []patchModel.PatchComment
	var total int64
	base := r.db.Model(&patchModel.PatchComment{}).Where("user_id = ?", userID)
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).Order("created DESC").Offset(offset).Limit(limit).Find(&comments).Error
	return comments, total, err
}

func (r *UserRepository) GetUserContributions(userID, offset, limit int) ([]patchModel.Patch, int64, error) {
	var patches []patchModel.Patch
	var total int64
	subQuery := r.db.Table("user_patch_contribute_relation").Where("user_id = ?", userID).Select("galgame_id")
	base := r.db.Model(&patchModel.Patch{}).Where("id IN (?)", subQuery)
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).Order("created DESC").Offset(offset).Limit(limit).Find(&patches).Error
	return patches, total, err
}

// ===== Follow =====

func (r *UserRepository) FindFollow(followerID, followingID int) (*model.UserFollowRelation, error) {
	var rel model.UserFollowRelation
	err := r.db.Where("follower_id = ? AND following_id = ?", followerID, followingID).First(&rel).Error
	return &rel, err
}

func (r *UserRepository) CreateFollow(rel *model.UserFollowRelation) error {
	return r.db.Create(rel).Error
}

func (r *UserRepository) DeleteFollow(followerID, followingID int) error {
	return r.db.Where("follower_id = ? AND following_id = ?", followerID, followingID).
		Delete(&model.UserFollowRelation{}).Error
}

func (r *UserRepository) UpdateFollowCounts(followerID, followingID, delta int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&authModel.User{}).Where("id = ?", followerID).
			UpdateColumn("following_count", gorm.Expr("GREATEST(following_count + ?, 0)", delta)).Error; err != nil {
			return err
		}
		return tx.Model(&authModel.User{}).Where("id = ?", followingID).
			UpdateColumn("follower_count", gorm.Expr("GREATEST(follower_count + ?, 0)", delta)).Error
	})
}

// GetFollowerIDs / GetFollowingIDs return only the user_ids; the handler layer
// resolves them to display briefs via OAuth /users/batch (pkg/userclient).

func (r *UserRepository) GetFollowerIDs(userID, offset, limit int) ([]int, int64, error) {
	var ids []int
	var total int64
	base := r.db.Table("user_follow_relation").Where("following_id = ?", userID)
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).Select("follower_id").Offset(offset).Limit(limit).Pluck("follower_id", &ids).Error
	return ids, total, err
}

func (r *UserRepository) GetFollowingIDs(userID, offset, limit int) ([]int, int64, error) {
	var ids []int
	var total int64
	base := r.db.Table("user_follow_relation").Where("follower_id = ?", userID)
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).Select("following_id").Offset(offset).Limit(limit).Pluck("following_id", &ids).Error
	return ids, total, err
}

// ===== Daily =====

func (r *UserRepository) CheckIn(userID int, points int) error {
	return r.db.Model(&authModel.User{}).Where("id = ?", userID).Updates(map[string]any{
		"daily_check_in": 1,
		"moemoepoint":    gorm.Expr("moemoepoint + ?", points),
	}).Error
}
