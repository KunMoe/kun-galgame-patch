// Package model defines the blog table row and the /blog wire shapes.
//
// Blog is an independent, admin-managed, DB-backed feature (migration 015) —
// distinct from /about (about_post, seeded from .mdx). Images go through
// image_service: BannerImageHash is the content hash (the service derives the
// CDN URL), and inline images in Content are image_service CDN URLs.
package model

import (
	"time"

	"kun-galgame-patch-api/internal/infrastructure/markdown"
)

// Blog mirrors the blog table.
type Blog struct {
	ID              int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Title           string    `gorm:"size:255;not null;default:''" json:"title"`
	Summary         string    `gorm:"type:text;not null;default:''" json:"summary"`
	Content         string    `gorm:"type:text;not null;default:''" json:"content"`
	BannerImageHash string    `gorm:"column:banner_image_hash;type:char(64);not null;default:''" json:"banner_image_hash"`
	Status          int       `gorm:"not null;default:0" json:"status"` // 0=draft, 1=published
	Pin             bool      `gorm:"not null;default:false" json:"pin"`
	View            int       `gorm:"not null;default:0" json:"view"`
	UserID          int       `gorm:"not null;default:0" json:"user_id"`
	Created         time.Time `gorm:"autoCreateTime" json:"created"`
	Updated         time.Time `gorm:"autoUpdateTime" json:"updated"`
}

func (Blog) TableName() string { return "blog" }

// Status constants.
const (
	StatusDraft     = 0
	StatusPublished = 1
)

// BlogUser is the author brief (resolved from OAuth /users/batch).
type BlogUser struct {
	ID              int      `json:"id"`
	Name            string   `json:"name"`
	Avatar          string   `json:"avatar"`
	AvatarImageHash string   `json:"avatar_image_hash"`
	Roles           []string `json:"roles"`
}

// BlogCard is one entry in the /blog list (no body; banner is a derived URL).
type BlogCard struct {
	ID      int64     `json:"id"`
	Title   string    `json:"title"`
	Summary string    `json:"summary"`
	Banner  string    `json:"banner"` // derived image_service CDN URL ("" if none)
	Status  int       `json:"status"`
	Pin     bool      `json:"pin"`
	View    int       `json:"view"`
	User    *BlogUser `json:"user"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

// BlogDetail is the public GET /blog/:id shape (rendered HTML + TOC).
type BlogDetail struct {
	ID          int64              `json:"id"`
	Title       string             `json:"title"`
	Summary     string             `json:"summary"`
	ContentHTML string             `json:"content_html"`
	TOC         []markdown.TOCItem `json:"toc"`
	Banner      string             `json:"banner"`
	Status      int                `json:"status"`
	Pin         bool               `json:"pin"`
	View        int                `json:"view"`
	User        *BlogUser          `json:"user"`
	Created     time.Time          `json:"created"`
	Updated     time.Time          `json:"updated"`
}

// BlogEdit is the admin edit-load shape: the RAW markdown content + hash (so the
// editor can repopulate), plus the derived banner URL for preview.
type BlogEdit struct {
	ID              int64     `json:"id"`
	Title           string    `json:"title"`
	Summary         string    `json:"summary"`
	Content         string    `json:"content"`
	BannerImageHash string    `json:"banner_image_hash"`
	Banner          string    `json:"banner"`
	Status          int       `json:"status"`
	Pin             bool      `json:"pin"`
	View            int       `json:"view"`
	Created         time.Time `json:"created"`
	Updated         time.Time `json:"updated"`
}
