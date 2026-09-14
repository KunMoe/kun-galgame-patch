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

func (s *DocService) effectiveBanner(d model.Doc) string {
	if d.BannerImageHash != "" {
		if u := s.img.MainURL(d.BannerImageHash); u != "" {
			return u
		}
	}
	return d.Banner
}

func (s *DocService) effectiveAuthorAvatar(b *userclient.Brief) string {
	if b == nil {
		return ""
	}
	if b.AvatarImageHash != "" {
		if u := s.img.VariantURL(b.AvatarImageHash, "100"); u != "" {
			return u
		}
	}
	return b.Avatar
}

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

func (s *DocService) ListPinned() ([]model.CarouselItem, error) {
	docs, err := s.repo.GetAll(true)
	if err != nil {
		return nil, err
	}
	pinned := make([]model.Doc, 0)
	for _, d := range docs {
		if d.Pin {
			pinned = append(pinned, d)
		}
	}
	sort.SliceStable(pinned, func(i, j int) bool {
		return parseDocDate(pinned[i].Date).After(parseDocDate(pinned[j].Date))
	})
	uids := make([]int, 0, len(pinned))
	for _, d := range pinned {
		if d.AuthorUID > 0 {
			uids = append(uids, d.AuthorUID)
		}
	}
	briefs := userclient.BriefMapByInt(context.Background(), s.users, uids)

	items := make([]model.CarouselItem, 0, len(pinned))
	for _, d := range pinned {
		avatar := s.effectiveAuthorAvatar(briefs[d.AuthorUID])
		if avatar == "" {
			avatar = d.AuthorAvatar
		}
		items = append(items, model.CarouselItem{
			Title:        d.Title,
			Banner:       s.effectiveBanner(d),
			Description:  d.Description,
			Date:         d.Date,
			Slug:         d.Slug,
			Category:     d.Category,
			AuthorName:   d.AuthorName,
			AuthorAvatar: avatar,
		})
	}
	return items, nil
}

func parseDocDate(s string) time.Time {
	s = strings.TrimSpace(s)
	for _, layout := range []string{
		"2006-1-2", "2006-01-02", "2006/1/2", "2006/01/02", time.RFC3339,
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
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
	authorAvatar := d.AuthorAvatar
	if d.AuthorUID > 0 {
		if b := userclient.BriefMapByInt(context.Background(), s.users, []int{d.AuthorUID})[d.AuthorUID]; b != nil {
			if av := s.effectiveAuthorAvatar(b); av != "" {
				authorAvatar = av
			}
		}
	}
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
			AuthorAvatar:   authorAvatar,
			AuthorHomepage: d.AuthorHomepage,
			Pin:            d.Pin,
		},
		Prev: prev,
		Next: next,
	}, nil
}

func (s *DocService) IncrementViewBySlug(slug string) {
	d, err := s.repo.GetBySlug(slug)
	if err == nil && d.Status == model.StatusPublished {
		_ = s.repo.IncrementView(int(d.ID))
	}
}

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
	if b := userclient.BriefMapByInt(ctx, s.users, []int{userID})[userID]; b != nil {
		doc.AuthorName = b.Name
		// Stored, unlike the two read-time uses above: this row is the fallback
		// for when the OAuth brief cannot be read, and a CDN URL in it welds the
		// image host into the row -- the same shape that killed the sticker URLs.
		doc.AuthorAvatar = markdown.NormalizeContentImageURLs(s.effectiveAuthorAvatar(b))
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

func composeSlug(category, name string) string {
	return strings.Trim(category, "/") + "/" + strings.Trim(name, "/")
}

func nameOf(slug, category string) string {
	return strings.TrimPrefix(slug, category+"/")
}
