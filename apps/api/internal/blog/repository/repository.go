// Package repository is the DB layer for the blog feature (migration 015).
package repository

import (
	"kun-galgame-patch-api/internal/blog/model"

	"gorm.io/gorm"
)

type BlogRepository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *BlogRepository {
	return &BlogRepository{db: db}
}

// List returns a page of posts ordered pinned-first then newest. onlyPublished
// restricts to status=1 (the public feed); false returns drafts too (admin).
func (r *BlogRepository) List(onlyPublished bool, offset, limit int) ([]model.Blog, int64, error) {
	q := r.db.Model(&model.Blog{})
	if onlyPublished {
		q = q.Where("status = ?", model.StatusPublished)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var blogs []model.Blog
	err := q.Order("pin DESC, created DESC").Offset(offset).Limit(limit).Find(&blogs).Error
	return blogs, total, err
}

func (r *BlogRepository) GetByID(id int) (*model.Blog, error) {
	var blog model.Blog
	err := r.db.First(&blog, id).Error
	return &blog, err
}

func (r *BlogRepository) Create(blog *model.Blog) error {
	return r.db.Create(blog).Error
}

// Update applies the given column set to one post. Uses Updates(map) so only the
// provided fields change; gorm bumps `updated` via autoUpdateTime.
func (r *BlogRepository) Update(id int, fields map[string]any) error {
	return r.db.Model(&model.Blog{}).Where("id = ?", id).Updates(fields).Error
}

func (r *BlogRepository) Delete(id int) error {
	return r.db.Delete(&model.Blog{}, id).Error
}

// IncrementView bumps the view counter via UpdateColumn so it does NOT touch
// `updated` (a view is not an edit).
func (r *BlogRepository) IncrementView(id int) error {
	return r.db.Model(&model.Blog{}).Where("id = ?", id).
		UpdateColumn("view", gorm.Expr("view + 1")).Error
}
