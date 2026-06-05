// Package service holds the blog business logic: status gating, markdown
// rendering, image_service banner-URL derivation, and author-brief attach.
package service

import (
	"context"
	"fmt"
	"regexp"

	"kun-galgame-patch-api/internal/blog/dto"
	"kun-galgame-patch-api/internal/blog/model"
	"kun-galgame-patch-api/internal/blog/repository"
	"kun-galgame-patch-api/internal/infrastructure/markdown"
	"kun-galgame-patch-api/pkg/imageclient"
	"kun-galgame-patch-api/pkg/userclient"

	"gorm.io/gorm"
)

var hash64 = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

type BlogService struct {
	repo  *repository.BlogRepository
	img   *imageclient.Client
	users *userclient.Client
}

func New(repo *repository.BlogRepository, img *imageclient.Client, users *userclient.Client) *BlogService {
	return &BlogService{repo: repo, img: img, users: users}
}

// banner derives the image_service CDN URL from a hash ("" stays "").
func (s *BlogService) banner(hash string) string {
	if hash == "" {
		return ""
	}
	return s.img.MainURL(hash)
}

// ListPublic returns published posts; ListAdmin returns all (incl. drafts).
func (s *BlogService) ListPublic(ctx context.Context, page, limit int) ([]model.BlogCard, int64, error) {
	return s.list(ctx, true, page, limit)
}

func (s *BlogService) ListAdmin(ctx context.Context, page, limit int) ([]model.BlogCard, int64, error) {
	return s.list(ctx, false, page, limit)
}

func (s *BlogService) list(ctx context.Context, onlyPublished bool, page, limit int) ([]model.BlogCard, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 12
	}
	blogs, total, err := s.repo.List(onlyPublished, (page-1)*limit, limit)
	if err != nil {
		return nil, 0, err
	}

	cards := make([]model.BlogCard, len(blogs))
	uids := make([]int, 0, len(blogs))
	for i, b := range blogs {
		cards[i] = model.BlogCard{
			ID:      b.ID,
			Title:   b.Title,
			Summary: b.Summary,
			Banner:  s.banner(b.BannerImageHash),
			Status:  b.Status,
			Pin:     b.Pin,
			View:    b.View,
			Created: b.Created,
			Updated: b.Updated,
		}
		uids = append(uids, b.UserID)
	}
	briefs := userclient.BriefMapByInt(ctx, s.users, uids)
	for i := range cards {
		cards[i].User = toBlogUser(briefs[blogs[i].UserID])
	}
	return cards, total, nil
}

// GetPublic renders one published post. A missing post OR a draft returns
// gorm.ErrRecordNotFound so the handler maps both to a 404 (a draft must not be
// distinguishable from a non-existent post on the public surface).
func (s *BlogService) GetPublic(ctx context.Context, id int) (*model.BlogDetail, error) {
	blog, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if blog.Status != model.StatusPublished {
		return nil, gorm.ErrRecordNotFound
	}

	html, toc, err := markdown.RenderWithTOC(blog.Content)
	if err != nil {
		return nil, fmt.Errorf("render markdown: %w", err)
	}

	d := &model.BlogDetail{
		ID:          blog.ID,
		Title:       blog.Title,
		Summary:     blog.Summary,
		ContentHTML: html,
		TOC:         toc,
		Banner:      s.banner(blog.BannerImageHash),
		Status:      blog.Status,
		Pin:         blog.Pin,
		View:        blog.View,
		Created:     blog.Created,
		Updated:     blog.Updated,
	}
	briefs := userclient.BriefMapByInt(ctx, s.users, []int{blog.UserID})
	d.User = toBlogUser(briefs[blog.UserID])
	return d, nil
}

// GetForEdit returns the raw post (any status) for the admin editor.
func (s *BlogService) GetForEdit(id int) (*model.BlogEdit, error) {
	blog, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	return &model.BlogEdit{
		ID:              blog.ID,
		Title:           blog.Title,
		Summary:         blog.Summary,
		Content:         blog.Content,
		BannerImageHash: blog.BannerImageHash,
		Banner:          s.banner(blog.BannerImageHash),
		Status:          blog.Status,
		Pin:             blog.Pin,
		View:            blog.View,
		Created:         blog.Created,
		Updated:         blog.Updated,
	}, nil
}

func (s *BlogService) Create(userID int, req dto.BlogCreateRequest) (*model.Blog, error) {
	if req.BannerImageHash != "" && !hash64.MatchString(req.BannerImageHash) {
		return nil, fmt.Errorf("banner_image_hash 必须是 64 位十六进制 image_service hash")
	}
	blog := &model.Blog{
		Title:           req.Title,
		Summary:         req.Summary,
		Content:         req.Content,
		BannerImageHash: req.BannerImageHash,
		UserID:          userID,
	}
	if req.Status != nil {
		blog.Status = *req.Status
	}
	if req.Pin != nil {
		blog.Pin = *req.Pin
	}
	if err := s.repo.Create(blog); err != nil {
		return nil, err
	}
	return blog, nil
}

func (s *BlogService) Update(id int, req dto.BlogUpdateRequest) (*model.Blog, error) {
	if _, err := s.repo.GetByID(id); err != nil {
		return nil, err
	}

	fields := map[string]any{}
	if req.Title != nil {
		fields["title"] = *req.Title
	}
	if req.Summary != nil {
		fields["summary"] = *req.Summary
	}
	if req.Content != nil {
		fields["content"] = *req.Content
	}
	if req.BannerImageHash != nil {
		// "" clears the banner; otherwise it must be a valid hash.
		if *req.BannerImageHash != "" && !hash64.MatchString(*req.BannerImageHash) {
			return nil, fmt.Errorf("banner_image_hash 必须是 64 位十六进制 image_service hash")
		}
		fields["banner_image_hash"] = *req.BannerImageHash
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

func (s *BlogService) Delete(id int) error          { return s.repo.Delete(id) }
func (s *BlogService) IncrementView(id int) error   { return s.repo.IncrementView(id) }

func toBlogUser(b *userclient.Brief) *model.BlogUser {
	if b == nil {
		return nil
	}
	return &model.BlogUser{
		ID:              int(b.ID),
		Name:            b.Name,
		Avatar:          b.Avatar,
		AvatarImageHash: b.AvatarImageHash,
		Roles:           b.Roles,
	}
}
