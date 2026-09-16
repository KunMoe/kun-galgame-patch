package repository

import (
	"log/slog"

	"kun-galgame-patch-api/internal/comment/model"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) DB() *gorm.DB { return r.db }

// FindMapByLegacyID resolves a pre-cutover comment id. A miss is not an error:
// links to comments posted after the cutover already carry a post id.
func (r *Repository) FindMapByLegacyID(legacyID int) (*model.PatchCommentCommunityMap, error) {
	var row model.PatchCommentCommunityMap
	err := r.db.Where("old_comment_id = ?", legacyID).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// Bookkeeping the comment walls still own on moyu's own side. patch.comment_count
// is a cached display counter: nothing can recompute it from SQL any more, so
// the write path is the only thing that keeps it true.

func (r *Repository) BumpCommentCount(patchID, delta int) {
	if patchID <= 0 || delta == 0 {
		return
	}
	expr := "comment_count + ?"
	if delta < 0 {
		expr = "GREATEST(comment_count + ?, 0)"
	}
	if err := r.db.Table("patch").Where("id = ?", patchID).
		Update("comment_count", gorm.Expr(expr, delta)).Error; err != nil {
		slog.Warn("comment: comment_count adjust failed (best-effort)",
			"patch_id", patchID, "delta", delta, "error", err)
	}
}

func (r *Repository) EnsureContributor(userID, patchID int) {
	if userID <= 0 || patchID <= 0 {
		return
	}
	result := r.db.Exec(`
		INSERT INTO user_patch_contribute_relation (user_id, galgame_id, created, updated)
		VALUES (?, ?, now(), now())
		ON CONFLICT DO NOTHING`, userID, patchID)
	if result.Error != nil {
		slog.Warn("comment: contributor upsert failed (best-effort)",
			"patch_id", patchID, "user_id", userID, "error", result.Error)
		return
	}
	if result.RowsAffected > 0 {
		r.db.Table("patch").Where("id = ?", patchID).
			Update("contribute_count", gorm.Expr("contribute_count + 1"))
	}
}

func (r *Repository) PatchOwner(patchID int) int {
	var uid int
	r.db.Table("patch").Select("user_id").Where("id = ?", patchID).Scan(&uid)
	return uid
}

// ResourceRef is what a resource comment wall needs to know about its resource:
// who published it (the notification recipient) and which game it hangs off.
type ResourceRef struct {
	OwnerID   int `gorm:"column:user_id"`
	GalgameID int `gorm:"column:galgame_id"`
	Status    int `gorm:"column:status"`
}

func (r *Repository) ResourceRef(resourceID int) (*ResourceRef, error) {
	var row ResourceRef
	res := r.db.Table("patch_resource").
		Select("user_id, galgame_id, status").
		Where("id = ?", resourceID).Limit(1).Find(&row)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &row, nil
}

// ResourcePatchIDs answers a whole page of resource walls at once. A feed can
// carry fifty of them, and the resolver used to ask for each one on its own.
func (r *Repository) ResourcePatchIDs(resourceIDs []int) map[int]int {
	out := make(map[int]int, len(resourceIDs))
	if len(resourceIDs) == 0 {
		return out
	}
	var rows []struct {
		ID        int `gorm:"column:id"`
		GalgameID int `gorm:"column:galgame_id"`
	}
	if err := r.db.Table("patch_resource").
		Select("id, galgame_id").
		Where("id IN ?", resourceIDs).
		Scan(&rows).Error; err != nil {
		slog.Warn("comment: resource anchor lookup failed (best-effort)", "error", err)
		return out
	}
	for _, row := range rows {
		out[row.ID] = row.GalgameID
	}
	return out
}

func (r *Repository) MapByLegacyIDs(ids []int) map[int]model.PatchCommentCommunityMap {
	out := make(map[int]model.PatchCommentCommunityMap, len(ids))
	if len(ids) == 0 {
		return out
	}
	var rows []model.PatchCommentCommunityMap
	if err := r.db.Where("old_comment_id IN ?", ids).Find(&rows).Error; err != nil {
		slog.Warn("comment: legacy map lookup failed (best-effort)", "error", err)
		return out
	}
	for _, row := range rows {
		out[row.OldCommentID] = row
	}
	return out
}
