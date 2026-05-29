// Package service is the file-system layer for the /about pages.
//
// Posts are static .mdx files. We re-read them from disk on every request —
// cheap because there are <30 files and Linux page cache makes this a
// nanosecond-grade operation. If profiling ever shows it being a hot spot we
// can add a memo with a mtime-keyed invalidation.
package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"kun-galgame-patch-api/internal/about/model"
	"kun-galgame-patch-api/internal/infrastructure/markdown"

	"gopkg.in/yaml.v3"
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

// AboutService handles /about endpoints.
type AboutService struct {
	postsDir string
}

// New constructs a service rooted at the given directory.
func New(postsDir string) *AboutService {
	return &AboutService{postsDir: postsDir}
}

// rawPost is the parsed contents of a single .mdx file.
type rawPost struct {
	slug        string
	directory   string
	frontmatter model.PostFrontmatter
	body        string
}

// readAll walks postsDir, parses every .mdx, and returns them sorted by date
// (newest first), matching the legacy listAllPosts behaviour.
func (s *AboutService) readAll() ([]rawPost, error) {
	if _, err := os.Stat(s.postsDir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("posts dir unavailable: %w", err)
	}

	var posts []rawPost
	err := filepath.Walk(s.postsDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".mdx") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fm, body, err := parseFrontmatter(data)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		rel, err := filepath.Rel(s.postsDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		slug := strings.TrimSuffix(rel, ".mdx")
		directory := ""
		if i := strings.IndexByte(slug, '/'); i > 0 {
			directory = slug[:i]
		}

		posts = append(posts, rawPost{
			slug:        slug,
			directory:   directory,
			frontmatter: fm,
			body:        body,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Newest-first by date string. Frontmatter writes ISO dates, lexical
	// ordering matches chronological ordering for that format.
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].frontmatter.Date > posts[j].frontmatter.Date
	})
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
		dir      string
		entries  []rawPost
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
	// Forbid path traversal — slugs are POSIX paths under postsDir. Return
	// os.ErrNotExist (not a generic error) so the handler maps it to 404:
	// the check fires before any file read, so a traversal probe should look
	// like an ordinary missing article rather than a 500.
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

// parseFrontmatter splits the YAML block delimited by `---` lines from the
// body. Returns frontmatter + body, or an error if the block is malformed.
// An empty/missing frontmatter yields a zero-valued PostFrontmatter.
func parseFrontmatter(data []byte) (model.PostFrontmatter, string, error) {
	const delim = "---"
	src := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(src, delim) {
		return model.PostFrontmatter{}, src, nil
	}
	rest := src[len(delim):]
	rest = strings.TrimLeft(rest, "\n")
	end := strings.Index(rest, "\n"+delim)
	if end < 0 {
		return model.PostFrontmatter{}, "", fmt.Errorf("frontmatter is not terminated")
	}
	yamlSrc := rest[:end]
	body := rest[end+len("\n"+delim):]
	body = strings.TrimLeft(body, "\n")

	var fm model.PostFrontmatter
	if err := yaml.Unmarshal([]byte(yamlSrc), &fm); err != nil {
		return model.PostFrontmatter{}, "", fmt.Errorf("parse frontmatter: %w", err)
	}
	return fm, body, nil
}
