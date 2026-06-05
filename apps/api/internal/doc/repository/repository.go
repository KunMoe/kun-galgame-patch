// Package repository is the DB layer for the unified doc feature (migration 016).
package repository

import (
	"kun-galgame-patch-api/internal/doc/model"

	"gorm.io/gorm"
)

type DocRepository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *DocRepository {
	return &DocRepository{db: db}
}

// GetAll returns docs newest-first by the frontmatter date string (lexical ==
// chronological for ISO), slug ASC to break ties. onlyPublished restricts to
// status=1 (public surface); false includes drafts (admin).
func (r *DocRepository) GetAll(onlyPublished bool) ([]model.Doc, error) {
	q := r.db.Model(&model.Doc{})
	if onlyPublished {
		q = q.Where("status = ?", model.StatusPublished)
	}
	var docs []model.Doc
	err := q.Order("date DESC, slug ASC").Find(&docs).Error
	return docs, err
}

func (r *DocRepository) GetBySlug(slug string) (*model.Doc, error) {
	var doc model.Doc
	err := r.db.Where("slug = ?", slug).First(&doc).Error
	return &doc, err
}

func (r *DocRepository) GetByID(id int) (*model.Doc, error) {
	var doc model.Doc
	err := r.db.First(&doc, id).Error
	return &doc, err
}

func (r *DocRepository) Create(doc *model.Doc) error {
	return r.db.Create(doc).Error
}

func (r *DocRepository) Update(id int, fields map[string]any) error {
	return r.db.Model(&model.Doc{}).Where("id = ?", id).Updates(fields).Error
}

func (r *DocRepository) Delete(id int) error {
	return r.db.Delete(&model.Doc{}, id).Error
}

// IncrementView bumps the counter without touching updated_at (UpdateColumn).
func (r *DocRepository) IncrementView(id int) error {
	return r.db.Model(&model.Doc{}).Where("id = ?", id).
		UpdateColumn("view", gorm.Expr("view + 1")).Error
}
