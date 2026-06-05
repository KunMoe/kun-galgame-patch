// Package model defines the on-the-wire shapes for the /about endpoints.
//
// Posts are static .mdx files under cfg.About.PostsDir (legacy `apps/web/posts`).
// Each file carries YAML frontmatter (title / banner / date / description /
// authorUid / authorName / authorAvatar / authorHomepage / pin) followed by
// markdown body. The directory layout is two levels deep at most:
//
//	posts/
//	  <directory>/   (e.g. dev / kun / galgame / notice)
//	    <slug>.mdx
//
// `slug` in API responses is always `<directory>/<filename-without-mdx>` with
// forward slashes.
package model

import (
	"time"

	"kun-galgame-patch-api/internal/infrastructure/markdown"
)

// AboutPost is the about_post table row (migration 014). The /about articles
// used to be on-disk .mdx files re-read per request; they now live here, with
// the .mdx files kept in the repo as the editable source (re-seed via
// cmd/migrate-about-posts). `Content` is the raw markdown body — HTML and TOC
// are rendered on read so output stays in lockstep with the markdown package.
// `Date` mirrors the frontmatter date string verbatim (ISO; lexical sort ==
// chronological). `Slug` is "<directory>/<name>" (forward slashes).
type AboutPost struct {
	ID             int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Slug           string    `gorm:"size:255;not null;uniqueIndex" json:"slug"`
	Directory      string    `gorm:"size:64;not null;default:''" json:"directory"`
	Title          string    `gorm:"size:255;not null;default:''" json:"title"`
	Banner         string    `gorm:"size:512;not null;default:''" json:"banner"`
	Description    string    `gorm:"type:text;not null;default:''" json:"description"`
	Date           string    `gorm:"size:32;not null;default:''" json:"date"`
	AuthorUID      int       `gorm:"column:author_uid;not null;default:0" json:"author_uid"`
	AuthorName     string    `gorm:"size:255;not null;default:''" json:"author_name"`
	AuthorAvatar   string    `gorm:"size:512;not null;default:''" json:"author_avatar"`
	AuthorHomepage string    `gorm:"size:512;not null;default:''" json:"author_homepage"`
	Pin            bool      `gorm:"not null;default:false" json:"pin"`
	Content        string    `gorm:"type:text;not null;default:''" json:"content"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (AboutPost) TableName() string { return "about_post" }

// PostFrontmatter mirrors the YAML block at the top of every .mdx file.
type PostFrontmatter struct {
	Title          string `yaml:"title" json:"title"`
	Banner         string `yaml:"banner" json:"banner"`
	Description    string `yaml:"description" json:"description"`
	Date           string `yaml:"date" json:"date"`
	AuthorUID      int    `yaml:"authorUid" json:"author_uid,omitempty"`
	AuthorName     string `yaml:"authorName" json:"author_name"`
	AuthorAvatar   string `yaml:"authorAvatar" json:"author_avatar"`
	AuthorHomepage string `yaml:"authorHomepage" json:"author_homepage,omitempty"`
	Pin            bool   `yaml:"pin" json:"pin,omitempty"`
}

// PostMetadata is the shape returned in the /about/posts list (one entry per
// .mdx file). text_count is post-body length minus a constant fudge so the
// "N 字" label on the cards matches the legacy behaviour.
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

// TreeNode is one node in the directory tree returned by /about/posts.
// Files have type=file and an empty Children slice; directories have
// type=directory and recursive Children.
type TreeNode struct {
	Name     string     `json:"name"`
	Label    string     `json:"label"`
	Path     string     `json:"path"`
	Type     string     `json:"type"` // "file" or "directory"
	Children []TreeNode `json:"children,omitempty"`
}

// PostsResponse bundles the flat metadata list (used for /about index card
// grid) and the tree (used for the doc-detail page sidebar). One round-trip
// covers both.
type PostsResponse struct {
	Items []PostMetadata `json:"items"`
	Tree  TreeNode       `json:"tree"`
}

// PostDetail is the shape returned by /about/post.
type PostDetail struct {
	Slug        string             `json:"slug"`
	HTML        string             `json:"html"`
	TOC         []markdown.TOCItem `json:"toc"`
	Frontmatter PostFrontmatter    `json:"frontmatter"`
	Prev        *PostMetadata      `json:"prev"`
	Next        *PostMetadata      `json:"next"`
}
