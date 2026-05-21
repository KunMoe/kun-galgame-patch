package repository

import (
	"encoding/json"
	"time"

	adminModel "kun-galgame-patch-api/internal/admin/model"
	authModel "kun-galgame-patch-api/internal/auth/model"
	patchModel "kun-galgame-patch-api/internal/patch/model"

	"gorm.io/gorm"
)

type AdminRepository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

// ===== Comments =====

func (r *AdminRepository) GetComments(search string, offset, limit int) ([]patchModel.PatchComment, int64, error) {
	var comments []patchModel.PatchComment
	var total int64

	// Independent statements for Count vs Find — see gorm v2 reuse footgun
	// documented in message/repository.go GetMessages.
	base := r.db.Model(&patchModel.PatchComment{})
	if search != "" {
		base = base.Where("content ILIKE ?", "%"+search+"%")
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

func (r *AdminRepository) DeleteComment(commentID int) error {
	return r.db.Delete(&patchModel.PatchComment{}, commentID).Error
}

// ===== Resources =====

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
	return r.db.Delete(&patchModel.PatchResource{}, resourceID).Error
}

// User management & creator-application repo methods are gone with the
// migration: identity is owned by OAuth, and the creator role was retired.

// ===== Stats =====

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

// ===== Resource file history (MOYU-PR5 / M3) =====

// GetResourceFileHistory returns the audit trail for one resource, newest
// first. Page-based; default limit comes from the caller. Admin-only — exposed
// at GET /api/v1/admin/resource/:id/history.
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

// ===== Admin Logs =====

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

// ===== All Patches (admin browse) =====

// GetAllPatches lists every patch with pagination, optionally filtering by
// substring of vndb_id (game names are owned by Wiki and cannot be searched
// locally; the admin frontend pairs this listing with the patch_id-based
// patch detail link to navigate further).
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

// ===== Orphan Patches (D12 cleanup) =====

// GetOrphanPatches returns a paginated list of patches with galgame_id=0
// (no matching galgame found in Wiki). Ordered by resource count descending so
// admins can prioritize "important" orphans that already have resources.
//
// Two categories:
//   - vndb_id LIKE 'pending-%': vndb_id was not filled at creation time
//   - vndb_id looks like vN but Wiki lookup fails: typo or deleted in Wiki
func (r *AdminRepository) GetOrphanPatches(offset, limit int) ([]patchModel.Patch, int64, error) {
	var patches []patchModel.Patch
	var total int64
	base := r.db.Model(&patchModel.Patch{}).Where("galgame_id = 0")
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).
		Order("resource_count DESC, comment_count DESC, favorite_count DESC, id ASC").
		Offset(offset).Limit(limit).
		Find(&patches).Error
	return patches, total, err
}

// CountOrphanPatches returns totals for galgame_id=0 split into pending vs. valid-format-but-missing-in-Wiki.
func (r *AdminRepository) CountOrphanPatches() (pendingCount, badVndbCount int64, err error) {
	if err := r.db.Model(&patchModel.Patch{}).
		Where("galgame_id = 0 AND vndb_id LIKE 'pending-%'").
		Count(&pendingCount).Error; err != nil {
		return 0, 0, err
	}
	err = r.db.Model(&patchModel.Patch{}).
		Where("galgame_id = 0 AND vndb_id NOT LIKE 'pending-%'").
		Count(&badVndbCount).Error
	return
}
