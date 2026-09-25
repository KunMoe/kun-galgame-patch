package repository

import (
	"log/slog"

	authModel "kun-galgame-patch-api/internal/auth/model"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/utils"

	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindByID(id int) (*authModel.User, error) {
	var user authModel.User
	err := r.db.First(&user, id).Error
	return &user, err
}

func (r *UserRepository) Exists(id int) (bool, error) {
	var n int64
	err := r.db.Model(&authModel.User{}).Where("id = ?", id).Count(&n).Error
	return n > 0, err
}

func countOrLog(q *gorm.DB, what string, userID int) int64 {
	var count int64
	if err := q.Count(&count).Error; err != nil {
		slog.Warn("user profile count failed", "what", what, "user_id", userID, "error", err)
	}
	return count
}

func (r *UserRepository) CountUserPatches(userID int) int64 {
	return countOrLog(r.db.Model(&patchModel.Patch{}).Where("user_id = ?", userID), "patches", userID)
}

func (r *UserRepository) CountUserResources(userID int) int64 {
	return countOrLog(r.db.Model(&patchModel.PatchResource{}).Where("user_id = ? AND status <> 2", userID), "resources", userID)
}

func (r *UserRepository) GetUserPatches(userID, offset, limit int, includeEmpty bool, contentLimit string) ([]patchModel.Patch, int64, error) {
	var patches []patchModel.Patch
	var total int64
	base := r.db.Model(&patchModel.Patch{}).Where("user_id = ?", userID)
	if !includeEmpty {
		base = base.Where("resource_count > 0")
	}
	base = utils.ScopePatchContentLimit(base, contentLimit)
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).Order("created DESC, id DESC").Offset(offset).Limit(limit).Find(&patches).Error
	return patches, total, err
}

func (r *UserRepository) GetUserResources(userID, offset, limit int) ([]patchModel.PatchResource, int64, error) {
	var resources []patchModel.PatchResource
	var total int64
	base := r.db.Model(&patchModel.PatchResource{}).Where("user_id = ? AND status <> 2", userID)
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).Order("created DESC, id DESC").Offset(offset).Limit(limit).Find(&resources).Error
	return resources, total, err
}

// GetUserFavoritesByIDs is the old GetUserFavorites with its subquery replaced
// by an id list. Favourites are catalog folder memberships since the cutover,
// so the set arrives over HTTP; everything after that — the empty-patch
// filter, the content-limit scope, the ordering and the paging — is the query
// this site always ran, and is left alone.
// A folder item names a catalog work and a page id IS that number since
// migration 037, so this narrows the shelf to the games that have a page here
// rather than translating between two id spaces.
func (r *UserRepository) ExistingPatchIDs(ids []int) (map[int]bool, error) {
	out := make(map[int]bool, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var found []int
	if err := r.db.Table("patch").Where("id IN ?", ids).Pluck("id", &found).Error; err != nil {
		return nil, err
	}
	for _, id := range found {
		out[id] = true
	}
	return out, nil
}

func (r *UserRepository) GetUserFavoritesByIDs(patchIDs []int, offset, limit int, includeEmpty bool, contentLimit string) ([]patchModel.Patch, int64, error) {
	if len(patchIDs) == 0 {
		return []patchModel.Patch{}, 0, nil
	}
	var patches []patchModel.Patch
	var total int64
	base := r.favoritesScope(patchIDs, includeEmpty, contentLimit)
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).Order("created DESC, id DESC").Offset(offset).Limit(limit).Find(&patches).Error
	return patches, total, err
}

// CountUserFavoritesByIDs is the profile header's number, taken off the very
// builder the tab's page is taken off. Counted any other way the header and
// the list disagree the moment the reader's NSFW gate hides a row.
func (r *UserRepository) CountUserFavoritesByIDs(patchIDs []int, includeEmpty bool, contentLimit string) (int64, error) {
	if len(patchIDs) == 0 {
		return 0, nil
	}
	var total int64
	err := r.favoritesScope(patchIDs, includeEmpty, contentLimit).Count(&total).Error
	return total, err
}

func (r *UserRepository) favoritesScope(patchIDs []int, includeEmpty bool, contentLimit string) *gorm.DB {
	base := r.db.Model(&patchModel.Patch{}).Where("id IN ?", patchIDs)
	if !includeEmpty {
		base = base.Where("resource_count > 0")
	}
	return utils.ScopePatchContentLimit(base, contentLimit)
}

func (r *UserRepository) GetUserContributions(userID, offset, limit int, includeEmpty bool, contentLimit string) ([]patchModel.Patch, int64, error) {
	var patches []patchModel.Patch
	var total int64
	subQuery := r.db.Table("user_patch_contribute_relation").Where("user_id = ?", userID).Select("galgame_id")
	base := r.db.Model(&patchModel.Patch{}).Where("id IN (?)", subQuery)
	if !includeEmpty {
		base = base.Where("resource_count > 0")
	}
	base = utils.ScopePatchContentLimit(base, contentLimit)
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).Order("created DESC, id DESC").Offset(offset).Limit(limit).Find(&patches).Error
	return patches, total, err
}

func (r *UserRepository) CheckIn(userID int) (int64, error) {
	res := r.db.Model(&authModel.User{}).
		Where("id = ? AND daily_check_in = 0", userID).
		Update("daily_check_in", 1)
	return res.RowsAffected, res.Error
}

func (r *UserRepository) CountPublishedPatchResources(userID int) int64 {
	return countOrLog(r.db.Model(&patchModel.PatchResource{}).Where("user_id = ? AND status = 0", userID), "published_resources", userID)
}
