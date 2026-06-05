// Package repository is the DB layer for /about posts (about_post, migration
// 014). The service reads via GetAll; the one-time seeder
// (cmd/migrate-about-posts) writes via Upsert.
package repository

import (
	"kun-galgame-patch-api/internal/about/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AboutRepository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *AboutRepository {
	return &AboutRepository{db: db}
}

// GetAll returns every post newest-first (date DESC), slug ASC to break ties
// deterministically. The service derives the flat list, the tree and prev/next
// from this single ordered read (post count is tiny).
func (r *AboutRepository) GetAll() ([]model.AboutPost, error) {
	var posts []model.AboutPost
	err := r.db.Order("date DESC, slug ASC").Find(&posts).Error
	return posts, err
}

// Upsert inserts a post or updates it in place, keyed by the unique slug. Used
// by the cmd/migrate-about-posts seeder, so it is safe to re-run after editing
// the .mdx sources. created_at is preserved on update (not in the update set).
func (r *AboutRepository) Upsert(p *model.AboutPost) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "slug"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"directory", "title", "banner", "description", "date",
			"author_uid", "author_name", "author_avatar", "author_homepage",
			"pin", "content", "updated_at",
		}),
	}).Create(p).Error
}
