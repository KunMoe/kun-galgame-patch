// Package service serves the /about pages from the about_post table
// (migration 014). Posts used to be on-disk .mdx files re-read per request;
// cmd/migrate-about-posts seeded them into the DB and the .mdx files remain the
// editable source. HTML + TOC are rendered from the stored markdown body on
// read so the output stays in lockstep with the markdown package.
package service

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"kun-galgame-patch-api/internal/about/model"
	"kun-galgame-patch-api/internal/infrastructure/markdown"
)

// directoryLabels mirrors the legacy aboutDirectoryLabelMap so the sidebar
// shows the same Chinese names as the Next.js project.
var directoryLabels = map[string]string{
	"about":   "关于我们",
	"dev":     "开发文档",
	"galgame": "Galgame",
	"kun":     "关于鲲",
	"notice":  "公告",
}

// PostStore is the DB dependency, implemented by about/repository.AboutRepository.
// An interface (not the concrete repo) so the service can be unit-tested with a
// fake, no DB required.
type PostStore interface {
	GetAll() ([]model.AboutPost, error)
}

// AboutService handles /about endpoints.
type AboutService struct {
	repo PostStore
}

// New constructs a service backed by the given post store.
func New(repo PostStore) *AboutService {
	return &AboutService{repo: repo}
}

// rawPost is the in-memory shape the list/tree/detail logic operates on.
type rawPost struct {
	slug        string
	directory   string
	frontmatter model.PostFrontmatter
	body        string
}

// readAll loads every post from the DB (already date-sorted newest-first by the
// repository) and maps it to the in-memory rawPost shape.
func (s *AboutService) readAll() ([]rawPost, error) {
	rows, err := s.repo.GetAll()
	if err != nil {
		return nil, err
	}
	posts := make([]rawPost, len(rows))
	for i := range rows {
		r := rows[i]
		posts[i] = rawPost{
			slug:      r.Slug,
			directory: r.Directory,
			frontmatter: model.PostFrontmatter{
				Title:          r.Title,
				Banner:         r.Banner,
				Description:    r.Description,
				Date:           r.Date,
				AuthorUID:      r.AuthorUID,
				AuthorName:     r.AuthorName,
				AuthorAvatar:   r.AuthorAvatar,
				AuthorHomepage: r.AuthorHomepage,
				Pin:            r.Pin,
			},
			body: r.Content,
		}
	}
	return posts, nil
}

// listMetadata maps the rawPost slice to the wire shape.
func listMetadata(posts []rawPost) []model.PostMetadata {
	items := make([]model.PostMetadata, len(posts))
	for i, p := range posts {
		// Match the legacy "N 字" estimation: body length minus a 300-char
		// fudge to account for code fences / images. Negative results clamp.
		count := len([]rune(p.body)) - 300
		if count < 0 {
			count = 0
		}
		items[i] = model.PostMetadata{
			Title:       p.frontmatter.Title,
			Banner:      p.frontmatter.Banner,
			Date:        p.frontmatter.Date,
			Description: p.frontmatter.Description,
			TextCount:   count,
			Slug:        p.slug,
			Path:        p.slug,
			Directory:   p.directory,
		}
	}
	return items
}

// buildTree groups posts by their top-level directory and returns a single
// "about" root node containing one child per directory.
func buildTree(posts []rawPost) model.TreeNode {
	type bucket struct {
		dir     string
		entries []rawPost
	}
	order := []string{}
	groups := map[string]*bucket{}
	for _, p := range posts {
		if p.directory == "" {
			continue
		}
		if _, ok := groups[p.directory]; !ok {
			groups[p.directory] = &bucket{dir: p.directory}
			order = append(order, p.directory)
		}
		groups[p.directory].entries = append(groups[p.directory].entries, p)
	}
	sort.Strings(order)

	root := model.TreeNode{
		Name:  "about",
		Label: directoryLabels["about"],
		Path:  "",
		Type:  "directory",
	}
	for _, dir := range order {
		dirNode := model.TreeNode{
			Name:  dir,
			Label: directoryLabel(dir),
			Path:  dir,
			Type:  "directory",
		}
		// Stable name order inside a directory.
		sort.Slice(groups[dir].entries, func(i, j int) bool {
			return groups[dir].entries[i].slug < groups[dir].entries[j].slug
		})
		for _, p := range groups[dir].entries {
			name := strings.TrimPrefix(p.slug, dir+"/")
			dirNode.Children = append(dirNode.Children, model.TreeNode{
				Name:  name,
				Label: p.frontmatter.Title,
				Path:  p.slug,
				Type:  "file",
			})
		}
		root.Children = append(root.Children, dirNode)
	}
	return root
}

func directoryLabel(dir string) string {
	if v, ok := directoryLabels[dir]; ok {
		return v
	}
	return dir
}

// List returns both the flat metadata list and the tree in a single read.
func (s *AboutService) List() (*model.PostsResponse, error) {
	posts, err := s.readAll()
	if err != nil {
		return nil, err
	}
	return &model.PostsResponse{
		Items: listMetadata(posts),
		Tree:  buildTree(posts),
	}, nil
}

// GetPost returns the rendered HTML, TOC, frontmatter and adjacent (prev/next)
// posts for the given slug.
func (s *AboutService) GetPost(slug string) (*model.PostDetail, error) {
	if slug == "" {
		return nil, fmt.Errorf("slug is required")
	}
	// A slug containing ".." is never a real post; return os.ErrNotExist (not a
	// generic error) so the handler maps it to a 404 rather than a 500.
	if strings.Contains(slug, "..") {
		return nil, os.ErrNotExist
	}

	posts, err := s.readAll()
	if err != nil {
		return nil, err
	}
	idx := -1
	for i, p := range posts {
		if p.slug == slug {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, os.ErrNotExist
	}

	html, toc, err := markdown.RenderWithTOC(posts[idx].body)
	if err != nil {
		return nil, fmt.Errorf("render markdown: %w", err)
	}

	// Posts are sorted newest-first; "prev" in reading order is index+1
	// (older), "next" is index-1 (newer). Mirrors getAdjacentPosts.
	var prev, next *model.PostMetadata
	all := listMetadata(posts)
	if idx+1 < len(all) {
		p := all[idx+1]
		prev = &p
	}
	if idx-1 >= 0 {
		n := all[idx-1]
		next = &n
	}

	return &model.PostDetail{
		Slug:        slug,
		HTML:        html,
		TOC:         toc,
		Frontmatter: posts[idx].frontmatter,
		Prev:        prev,
		Next:        next,
	}, nil
}
