// Package model defines the unified `doc` table (migration 016, promoted from
// about_post) and the /doc + /admin/doc wire shapes.
//
// A doc keeps the about structure (category + "<category>/<name>" slug + markdown
// body + tree navigation) and adds admin/blog capabilities: publish status,
// image_service banner (stored as a content hash, URL derived on read), view
// counter, author user_id. The public wire shapes are kept byte-compatible with
// the legacy /about responses so the (renamed) frontend pages work unchanged.
package model

import (
	"time"

	"kun-galgame-patch-api/internal/infrastructure/markdown"
)

// Doc is the doc table row.
type Doc struct {
	ID              int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Slug            string    `gorm:"size:255;not null;uniqueIndex" json:"slug"` // "<category>/<name>"
	Category        string    `gorm:"size:64;not null;default:''" json:"category"`
	Title           string    `gorm:"size:255;not null;default:''" json:"title"`
	Banner          string    `gorm:"size:512;not null;default:''" json:"banner"` // legacy static URL fallback
	BannerImageHash string    `gorm:"column:banner_image_hash;type:char(64);not null;default:''" json:"banner_image_hash"`
	Description     string    `gorm:"type:text;not null;default:''" json:"description"`
	Date            string    `gorm:"size:32;not null;default:''" json:"date"`
	AuthorUID       int       `gorm:"column:author_uid;not null;default:0" json:"author_uid"`
	AuthorName      string    `gorm:"size:255;not null;default:''" json:"author_name"`
	AuthorAvatar    string    `gorm:"size:512;not null;default:''" json:"author_avatar"`
	AuthorHomepage  string    `gorm:"size:512;not null;default:''" json:"author_homepage"`
	Pin             bool      `gorm:"not null;default:false" json:"pin"`
	Content         string    `gorm:"type:text;not null;default:''" json:"content"`
	Status          int       `gorm:"not null;default:1" json:"status"` // 0=draft, 1=published
	View            int       `gorm:"not null;default:0" json:"view"`
	UserID          int       `gorm:"not null;default:0" json:"user_id"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Doc) TableName() string { return "doc" }

const (
	StatusDraft     = 0
	StatusPublished = 1
)

// Frontmatter is the author/meta block in PostDetail (about-compatible keys).
type Frontmatter struct {
	Title          string `json:"title"`
	Banner         string `json:"banner"`
	Description    string `json:"description"`
	Date           string `json:"date"`
	AuthorUID      int    `json:"author_uid,omitempty"`
	AuthorName     string `json:"author_name"`
	AuthorAvatar   string `json:"author_avatar"`
	AuthorHomepage string `json:"author_homepage,omitempty"`
	Pin            bool   `json:"pin,omitempty"`
}

// PostMetadata is one entry in the /doc/posts flat list (about-compatible;
// `directory` JSON key kept for FE compat == category). Banner is the effective
// URL (image_service-derived when a hash is set, else the legacy static URL).
type PostMetadata struct {
	Title       string `json:"title"`
	Banner      string `json:"banner"`
	Date        string `json:"date"`
	Description string `json:"description"`
	TextCount   int    `json:"text_count"`
	Slug        string `json:"slug"`
	Path        string `json:"path"`
	Directory   string `json:"directory"`
}

// TreeNode is one node in the directory tree for the sidebar.
type TreeNode struct {
	Name     string     `json:"name"`
	Label    string     `json:"label"`
	Path     string     `json:"path"`
	Type     string     `json:"type"` // "file" or "directory"
	Children []TreeNode `json:"children,omitempty"`
}

// PostsResponse bundles the flat list + the tree (one round-trip).
type PostsResponse struct {
	Items []PostMetadata `json:"items"`
	Tree  TreeNode       `json:"tree"`
}

// CarouselItem is a pinned, published doc surfaced on the home carousel.
// Banner is the effective (image_service hash → CDN, else static) URL; Slug
// drives the /doc/<slug> link. Author fields feed the carousel card byline.
type CarouselItem struct {
	Title        string `json:"title"`
	Banner       string `json:"banner"`
	Description  string `json:"description"`
	Date         string `json:"date"`
	Slug         string `json:"slug"`
	Category     string `json:"category"`
	AuthorName   string `json:"author_name"`
	AuthorAvatar string `json:"author_avatar"`
}

// PostDetail is GET /doc/post?slug=... (about-compatible).
type PostDetail struct {
	Slug        string             `json:"slug"`
	HTML        string             `json:"html"`
	TOC         []markdown.TOCItem `json:"toc"`
	Frontmatter Frontmatter        `json:"frontmatter"`
	Prev        *PostMetadata      `json:"prev"`
	Next        *PostMetadata      `json:"next"`
}

// AdminItem is one row in the admin doc-management list (incl. drafts).
type AdminItem struct {
	ID       int64  `json:"id"`
	Category string `json:"category"`
	Slug     string `json:"slug"`
	Name     string `json:"name"` // slug minus the "<category>/" prefix
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Pin      bool   `json:"pin"`
	View     int    `json:"view"`
	Date     string `json:"date"`
	Banner   string `json:"banner"`
}

// AdminDetail is the admin edit-load shape: raw markdown + the hash + the
// effective banner URL for preview.
type AdminDetail struct {
	ID              int64  `json:"id"`
	Category        string `json:"category"`
	Slug            string `json:"slug"` // full "<category>/<name>"
	Name            string `json:"name"` // within-category name
	Title           string `json:"title"`
	Description     string `json:"description"`
	Content         string `json:"content"`
	BannerImageHash string `json:"banner_image_hash"`
	Banner          string `json:"banner"`
	Date            string `json:"date"`
	Status          int    `json:"status"`
	Pin             bool   `json:"pin"`
	View            int    `json:"view"`
}
