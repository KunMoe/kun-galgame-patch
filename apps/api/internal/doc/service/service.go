// Package service holds the unified doc logic: about-style list/tree/detail
// (markdown render, prev/next, category tree) plus blog-style admin CRUD,
// publish-status gating and image_service banner derivation.
package service

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"kun-galgame-patch-api/internal/doc/dto"
	"kun-galgame-patch-api/internal/doc/model"
	"kun-galgame-patch-api/internal/doc/repository"
	"kun-galgame-patch-api/internal/infrastructure/markdown"
	"kun-galgame-patch-api/pkg/imageclient"
	"kun-galgame-patch-api/pkg/userclient"

	"gorm.io/gorm"
)

var hash64 = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

// directoryLabels mirrors the legacy about category labels; unknown categories
// fall back to the raw name.
var directoryLabels = map[string]string{
	"about":   "关于我们",
	"dev":     "开发文档",
	"galgame": "Galgame",
	"kun":     "关于鲲",
	"notice":  "公告",
}

type DocService struct {
	repo  *repository.DocRepository
	img   *imageclient.Client
	users *userclient.Client
}

func New(repo *repository.DocRepository, img *imageclient.Client, users *userclient.Client) *DocService {
	return &DocService{repo: repo, img: img, users: users}
}

// effectiveBanner prefers the image_service-derived URL (from the hash) and
// falls back to the legacy static banner URL.
func (s *DocService) effectiveBanner(d model.Doc) string {
	if d.BannerImageHash != "" {
		if u := s.img.MainURL(d.BannerImageHash); u != "" {
			return u
		}
	}
	return d.Banner
}

// ===== Public =====

// List returns the published flat list + the category tree.
func (s *DocService) List() (*model.PostsResponse, error) {
	docs, err := s.repo.GetAll(true)
	if err != nil {
		return nil, err
	}
	return &model.PostsResponse{
		Items: s.listMetadata(docs),
		Tree:  buildTree(docs),
	}, nil
}

func (s *DocService) listMetadata(docs []model.Doc) []model.PostMetadata {
	items := make([]model.PostMetadata, len(docs))
	for i, d := range docs {
		count := len([]rune(d.Content)) - 300
		if count < 0 {
			count = 0
		}
		items[i] = model.PostMetadata{
			Title:       d.Title,
			Banner:      s.effectiveBanner(d),
			Date:        d.Date,
			Description: d.Description,
			TextCount:   count,
			Slug:        d.Slug,
			Path:        d.Slug,
			Directory:   d.Category,
		}
	}
	return items
}

func buildTree(docs []model.Doc) model.TreeNode {
	order := []string{}
	groups := map[string][]model.Doc{}
	for _, d := range docs {
		if d.Category == "" {
			continue
		}
		if _, ok := groups[d.Category]; !ok {
			order = append(order, d.Category)
		}
		groups[d.Category] = append(groups[d.Category], d)
	}
	sort.Strings(order)

	root := model.TreeNode{Name: "doc", Label: directoryLabels["about"], Path: "", Type: "directory"}
	for _, cat := range order {
		catNode := model.TreeNode{Name: cat, Label: directoryLabel(cat), Path: cat, Type: "directory"}
		entries := groups[cat]
		sort.Slice(entries, func(i, j int) bool { return entries[i].Slug < entries[j].Slug })
		for _, d := range entries {
			catNode.Children = append(catNode.Children, model.TreeNode{
				Name:  strings.TrimPrefix(d.Slug, cat+"/"),
				Label: d.Title,
				Path:  d.Slug,
				Type:  "file",
			})
		}
		root.Children = append(root.Children, catNode)
	}
	return root
}

func directoryLabel(cat string) string {
	if v, ok := directoryLabels[cat]; ok {
		return v
	}
	return cat
}

// GetPost renders one published doc by slug. Missing OR draft → ErrRecordNotFound
// so the handler maps both to a 404.
func (s *DocService) GetPost(slug string) (*model.PostDetail, error) {
	if slug == "" || strings.Contains(slug, "..") {
		return nil, gorm.ErrRecordNotFound
	}
	docs, err := s.repo.GetAll(true)
	if err != nil {
		return nil, err
	}
	idx := -1
	for i := range docs {
		if docs[i].Slug == slug {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, gorm.ErrRecordNotFound
	}

	html, toc, err := markdown.RenderWithTOC(docs[idx].Content)
	if err != nil {
		return nil, fmt.Errorf("render markdown: %w", err)
	}

	all := s.listMetadata(docs)
	var prev, next *model.PostMetadata
	if idx+1 < len(all) {
		p := all[idx+1]
		prev = &p
	}
	if idx-1 >= 0 {
		n := all[idx-1]
		next = &n
	}

	d := docs[idx]
	return &model.PostDetail{
		Slug: slug,
		HTML: html,
		TOC:  toc,
		Frontmatter: model.Frontmatter{
			Title:          d.Title,
			Banner:         s.effectiveBanner(d),
			Description:    d.Description,
			Date:           d.Date,
			AuthorUID:      d.AuthorUID,
			AuthorName:     d.AuthorName,
			AuthorAvatar:   d.AuthorAvatar,
			AuthorHomepage: d.AuthorHomepage,
			Pin:            d.Pin,
		},
		Prev: prev,
		Next: next,
	}, nil
}

// IncrementViewBySlug bumps the view counter for a published doc (best-effort).
func (s *DocService) IncrementViewBySlug(slug string) {
	d, err := s.repo.GetBySlug(slug)
	if err == nil && d.Status == model.StatusPublished {
		_ = s.repo.IncrementView(int(d.ID))
	}
}

// ===== Admin =====

func (s *DocService) ListAdmin() ([]model.AdminItem, error) {
	docs, err := s.repo.GetAll(false)
	if err != nil {
		return nil, err
	}
	items := make([]model.AdminItem, len(docs))
	for i, d := range docs {
		items[i] = model.AdminItem{
			ID:       d.ID,
			Category: d.Category,
			Slug:     d.Slug,
			Name:     nameOf(d.Slug, d.Category),
			Title:    d.Title,
			Status:   d.Status,
			Pin:      d.Pin,
			View:     d.View,
			Date:     d.Date,
			Banner:   s.effectiveBanner(d),
		}
	}
	return items, nil
}

func (s *DocService) GetForEdit(id int) (*model.AdminDetail, error) {
	d, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	return &model.AdminDetail{
		ID:              d.ID,
		Category:        d.Category,
		Slug:            d.Slug,
		Name:            nameOf(d.Slug, d.Category),
		Title:           d.Title,
		Description:     d.Description,
		Content:         d.Content,
		BannerImageHash: d.BannerImageHash,
		Banner:          s.effectiveBanner(*d),
		Date:            d.Date,
		Status:          d.Status,
		Pin:             d.Pin,
		View:            d.View,
	}, nil
}

func (s *DocService) Create(ctx context.Context, userID int, req dto.DocCreateRequest) (*model.Doc, error) {
	slug := composeSlug(req.Category, req.Name)
	if _, err := s.repo.GetBySlug(slug); err == nil {
		return nil, fmt.Errorf("该分类下已存在同名文档: %s", slug)
	}
	date := req.Date
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}
	doc := &model.Doc{
		Slug:            slug,
		Category:        req.Category,
		Title:           req.Title,
		Description:     req.Description,
		Content:         req.Content,
		BannerImageHash: req.BannerImageHash,
		Date:            date,
		Status:          model.StatusPublished,
		UserID:          userID,
		AuthorUID:       userID,
	}
	if req.Status != nil {
		doc.Status = *req.Status
	}
	if req.Pin != nil {
		doc.Pin = *req.Pin
	}
	// Snapshot the author name/avatar from OAuth so the frontmatter renders it
	// without a per-request lookup (mirrors how about stored author fields).
	if b := userclient.BriefMapByInt(ctx, s.users, []int{userID})[userID]; b != nil {
		doc.AuthorName = b.Name
		doc.AuthorAvatar = b.Avatar
	}
	if err := s.repo.Create(doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (s *DocService) Update(id int, req dto.DocUpdateRequest) (*model.Doc, error) {
	cur, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	fields := map[string]any{}
	newCat := cur.Category
	newName := nameOf(cur.Slug, cur.Category)
	slugTouched := false
	if req.Category != nil {
		newCat = *req.Category
		slugTouched = true
	}
	if req.Name != nil {
		newName = *req.Name
		slugTouched = true
	}
	if slugTouched {
		newSlug := composeSlug(newCat, newName)
		fields["category"] = newCat
		if newSlug != cur.Slug {
			if existing, e := s.repo.GetBySlug(newSlug); e == nil && existing.ID != cur.ID {
				return nil, fmt.Errorf("该分类下已存在同名文档: %s", newSlug)
			}
			fields["slug"] = newSlug
		}
	}
	if req.Title != nil {
		fields["title"] = *req.Title
	}
	if req.Description != nil {
		fields["description"] = *req.Description
	}
	if req.Content != nil {
		fields["content"] = *req.Content
	}
	if req.BannerImageHash != nil {
		if *req.BannerImageHash != "" && !hash64.MatchString(*req.BannerImageHash) {
			return nil, fmt.Errorf("banner_image_hash 必须是 64 位十六进制 hash")
		}
		fields["banner_image_hash"] = *req.BannerImageHash
	}
	if req.Date != nil {
		fields["date"] = *req.Date
	}
	if req.Status != nil {
		fields["status"] = *req.Status
	}
	if req.Pin != nil {
		fields["pin"] = *req.Pin
	}

	if len(fields) > 0 {
		if err := s.repo.Update(id, fields); err != nil {
			return nil, err
		}
	}
	return s.repo.GetByID(id)
}

func (s *DocService) Delete(id int) error { return s.repo.Delete(id) }

// composeSlug builds the stored "<category>/<name>" slug from its parts.
func composeSlug(category, name string) string {
	return strings.Trim(category, "/") + "/" + strings.Trim(name, "/")
}

// nameOf strips the "<category>/" prefix to recover the within-category name.
func nameOf(slug, category string) string {
	return strings.TrimPrefix(slug, category+"/")
}
