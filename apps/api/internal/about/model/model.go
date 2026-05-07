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
	"kun-galgame-patch-api/internal/infrastructure/markdown"
)

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
