package common

import (
	"context"
	"fmt"
	"math/rand"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/galgame/enricher"
	"kun-galgame-patch-api/internal/infrastructure/markdown"
	"kun-galgame-patch-api/internal/middleware"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/userclient"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type CommonHandler struct {
	db    *gorm.DB
	wiki  *galgameClient.Client
	users *userclient.Client
}

func NewHandler(db *gorm.DB, wiki *galgameClient.Client, users *userclient.Client) *CommonHandler {
	return &CommonHandler{db: db, wiki: wiki, users: users}
}

// attachResourceUsers / attachCommentUsers do the same id-collect → batch →
// stamp dance for the various list endpoints below. Best-effort: on OAuth
// error rows are returned with User=nil and the frontend falls back to the
// anonymous-avatar path.
func (h *CommonHandler) attachResourceUsers(ctx context.Context, rs []patchModel.PatchResource) {
	if len(rs) == 0 {
		return
	}
	uids := make([]int, 0, len(rs))
	for _, r := range rs {
		uids = append(uids, r.UserID)
	}
	briefs := userclient.BriefMapByInt(ctx, h.users, uids)
	for i := range rs {
		if b := briefs[rs[i].UserID]; b != nil {
			rs[i].User = &patchModel.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar, AvatarImageHash: b.AvatarImageHash, Roles: b.Roles}
		}
	}
}

func (h *CommonHandler) attachCommentUsers(ctx context.Context, cs []patchModel.PatchComment) {
	if len(cs) == 0 {
		return
	}
	uids := make([]int, 0, len(cs))
	for _, c := range cs {
		uids = append(uids, c.UserID)
	}
	briefs := userclient.BriefMapByInt(ctx, h.users, uids)
	for i := range cs {
		if b := briefs[cs[i].UserID]; b != nil {
			cs[i].User = &patchModel.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar, AvatarImageHash: b.AvatarImageHash, Roles: b.Roles}
		}
	}
}

// patchSummaryFinder adapts *gorm.DB to enricher.patchSummaryDB so the
// enricher can fetch the minimal patch projection without depending on gorm.
type patchSummaryFinder struct{ db *gorm.DB }

func (p patchSummaryFinder) LookupPatchesByIDs(ids []int) ([]patchModel.Patch, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []patchModel.Patch
	// D13: patch.id is the Wiki galgame_id, no separate column.
	err := p.db.Select("id", "vndb_id").
		Where("id IN ?", ids).Find(&rows).Error
	return rows, err
}

// ===== Home =====

type homeResponse struct {
	Galgames  []enricher.GalgameCard     `json:"galgames"`
	Resources []patchModel.PatchResource `json:"resources"`
	Comments  []patchModel.PatchComment  `json:"comments"`
}

// GetHome GET /api/home
//
// D12: NSFW filtering has moved to Wiki (via /api/search). This endpoint only
// shows "recent patches on this service"; the enriched galgame objects carry
// a content_limit field for the frontend to filter on the client.
func (h *CommonHandler) GetHome(c *fiber.Ctx) error {
	var patches []patchModel.Patch
	var resources []patchModel.PatchResource
	var comments []patchModel.PatchComment

	h.db.Model(&patchModel.Patch{}).Order("created DESC").Limit(12).Find(&patches)
	h.db.Model(&patchModel.PatchResource{}).Order("created DESC").Limit(6).Find(&resources)
	h.db.Model(&patchModel.PatchComment{}).Order("created DESC").Limit(6).Find(&comments)

	patchModel.RenderResourceNotes(resources)
	for i := range comments {
		comments[i].ContentHTML = markdown.MustRender(comments[i].Content)
	}
	h.attachResourceUsers(c.Context(), resources)
	h.attachCommentUsers(c.Context(), comments)
	h.attachPatchSummaries(c, comments, resources)

	return response.OK(c, homeResponse{
		Galgames:  enricher.EnrichPatches(c.Context(), h.wiki, h.users, patches),
		Resources: resources,
		Comments:  comments,
	})
}

// ===== Galgame List =====

type galgameListRequest struct {
	SelectedType string `query:"selected_type" validate:"required,min=1,max=107"`
	SortField    string `query:"sort_field" validate:"required,oneof=resource_update_time created view download"`
	SortOrder    string `query:"sort_order" validate:"required,oneof=asc desc"`
	Page         int    `query:"page" validate:"required,min=1"`
	Limit        int    `query:"limit" validate:"required,min=1,max=24"`
}

// GetGalgameList GET /api/galgame
//
// D12: Filtering by release date/NSFW has moved to Wiki (via /api/search). This
// endpoint only filters by patch-local fields (translation type) and sorts,
// then enriches the result via Wiki before returning.
func (h *CommonHandler) GetGalgameList(c *fiber.Ctx) error {
	var req galgameListRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	// Independent statements for Count vs Find — see gorm v2 reuse footgun
	// documented in message/repository.go GetMessages.
	base := h.db.Model(&patchModel.Patch{})
	if req.SelectedType != "all" {
		base = base.Where("type @> ?", fmt.Sprintf(`["%s"]`, req.SelectedType))
	}

	var total int64
	base.Session(&gorm.Session{}).Count(&total)

	var patches []patchModel.Patch
	err := base.Session(&gorm.Session{}).Order(fmt.Sprintf("%s %s", req.SortField, req.SortOrder)).
		Offset((req.Page - 1) * req.Limit).Limit(req.Limit).
		Find(&patches).Error
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	return response.OK(c, map[string]any{
		"galgames": enricher.EnrichPatches(c.Context(), h.wiki, h.users, patches),
		"total":    total,
	})
}

// ===== Global Comments =====

type commentListRequest struct {
	SortField string `query:"sort_field" validate:"required,oneof=created like_count"`
	SortOrder string `query:"sort_order" validate:"required,oneof=asc desc"`
	Page      int    `query:"page" validate:"required,min=1"`
	Limit     int    `query:"limit" validate:"required,min=1,max=50"`
}

// GetGlobalComments GET /api/comment
func (h *CommonHandler) GetGlobalComments(c *fiber.Ctx) error {
	var req commentListRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	var comments []patchModel.PatchComment
	var total int64

	base := h.db.Model(&patchModel.PatchComment{})
	base.Session(&gorm.Session{}).Count(&total)

	err := base.Session(&gorm.Session{}).Order(fmt.Sprintf("%s %s", req.SortField, req.SortOrder)).
		Offset((req.Page - 1) * req.Limit).Limit(req.Limit).
		Find(&comments).Error

	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	for i := range comments {
		comments[i].ContentHTML = markdown.MustRender(comments[i].Content)
	}
	h.attachCommentUsers(c.Context(), comments)
	h.attachPatchSummaries(c, comments, nil)
	return response.Paginated(c, comments, total)
}

// attachPatchSummaries fills the `Patch` field on every comment / resource row
// in one Wiki batch call, avoiding an N+1 over the page. Either slice may be
// nil when the corresponding endpoint does not need it.
func (h *CommonHandler) attachPatchSummaries(c *fiber.Ctx, comments []patchModel.PatchComment, resources []patchModel.PatchResource) {
	if len(comments) == 0 && len(resources) == 0 {
		return
	}

	idSet := make(map[int]struct{}, len(comments)+len(resources))
	for _, m := range comments {
		idSet[m.GalgameID] = struct{}{}
	}
	for _, r := range resources {
		idSet[r.GalgameID] = struct{}{}
	}
	if len(idSet) == 0 {
		return
	}
	ids := make([]int, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}

	summaries := enricher.BuildPatchSummaryMap(c.Context(), h.wiki, patchSummaryFinder{db: h.db}, ids)
	for i := range comments {
		if s, ok := summaries[comments[i].GalgameID]; ok {
			summary := s
			comments[i].Patch = &summary
		}
	}
	for i := range resources {
		if s, ok := summaries[resources[i].GalgameID]; ok {
			summary := s
			resources[i].Patch = &summary
		}
	}
}

// ===== Global Resources =====

type resourceListRequest struct {
	SortField string `query:"sort_field" validate:"required,oneof=update_time created download like_count"`
	SortOrder string `query:"sort_order" validate:"required,oneof=asc desc"`
	Page      int    `query:"page" validate:"required,min=1"`
	Limit     int    `query:"limit" validate:"required,min=1,max=50"`
}

// GetGlobalResources GET /api/resource
//
// D12: patch.content_limit has been removed; NSFW filtering is provided by Wiki. This endpoint no longer does local NSFW filtering.
func (h *CommonHandler) GetGlobalResources(c *fiber.Ctx) error {
	var req resourceListRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	var resources []patchModel.PatchResource
	var total int64

	base := h.db.Model(&patchModel.PatchResource{})
	base.Session(&gorm.Session{}).Count(&total)

	sortField := req.SortField
	if sortField == "like" {
		sortField = "like_count"
	}

	err := base.Session(&gorm.Session{}).Order(fmt.Sprintf("patch_resource.%s %s", sortField, req.SortOrder)).
		Offset((req.Page - 1) * req.Limit).Limit(req.Limit).
		Find(&resources).Error

	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	patchModel.RenderResourceNotes(resources)
	h.attachResourceUsers(c.Context(), resources)
	h.attachPatchSummaries(c, nil, resources)
	return response.Paginated(c, resources, total)
}

// GetResourceDetail GET /api/resource/:id
//
// Returns the resource with its owning patch enriched via Wiki, plus up to 5
// recommended resources from the same patch. The frontend renders the patch
// header (name / banner / vndb_id) from the `patch` field.
func (h *CommonHandler) GetResourceDetail(c *fiber.Ctx) error {
	resourceID := c.Params("id")
	var resource patchModel.PatchResource
	if dbErr := h.db.First(&resource, resourceID).Error; dbErr != nil {
		return response.Error(c, errors.ErrNotFound("resource not found"))
	}

	// Fetch the owning patch and enrich it via Wiki so the frontend has
	// name / banner / vndb_id without making a separate call.
	var patch patchModel.Patch
	var patchCard *enricher.GalgameCard
	if err := h.db.First(&patch, resource.GalgameID).Error; err == nil {
		card := enricher.EnrichPatch(c.Context(), h.wiki, h.users, &patch)
		patchCard = &card
	}

	// Recommendations — mirrors next-web /resource/detail:
	//   1. up to 5 other resources of the SAME patch (status=0, by download)
	//   2. if fewer than 5, top up with random popular resources from OTHER
	//      patches (status=0, download > 500), shuffled.
	const recTarget = 5
	var recs []patchModel.PatchResource
	h.db.Where("galgame_id = ? AND id != ? AND status = 0", resource.GalgameID, resource.ID).
		Order("download DESC").Limit(recTarget).Find(&recs)

	if len(recs) < recTarget {
		var pool []patchModel.PatchResource
		h.db.Where("id != ? AND galgame_id != ? AND status = 0 AND download > ?",
			resource.ID, resource.GalgameID, 500).
			Limit(20).Find(&pool)
		seen := make(map[int]bool, len(recs))
		for _, r := range recs {
			seen[r.ID] = true
		}
		extras := pool[:0]
		for _, r := range pool {
			if !seen[r.ID] {
				extras = append(extras, r)
			}
		}
		rand.Shuffle(len(extras), func(i, j int) {
			extras[i], extras[j] = extras[j], extras[i]
		})
		if need := recTarget - len(recs); need > 0 && len(extras) > 0 {
			if need > len(extras) {
				need = len(extras)
			}
			recs = append(recs, extras[:need]...)
		}
	}

	resource.NoteHTML = markdown.MustRender(resource.Note)
	patchModel.RenderResourceNotes(recs)

	// Attach publisher briefs to the main resource and the recommendations.
	one := []patchModel.PatchResource{resource}
	h.attachResourceUsers(c.Context(), one)
	resource = one[0]
	h.attachResourceUsers(c.Context(), recs)

	// Viewer-specific state (if logged in): is_liked on the main resource +
	// recommendations, and is_favorite on the owning patch — so the
	// redesigned hero renders the like/favorite buttons in their real state.
	patchFavorited := false
	if u := middleware.GetUser(c); u != nil && u.UID > 0 {
		ids := make([]int, 0, len(recs)+1)
		ids = append(ids, resource.ID)
		for i := range recs {
			ids = append(ids, recs[i].ID)
		}
		var likedIDs []int
		h.db.Model(&patchModel.UserPatchResourceLikeRelation{}).
			Where("user_id = ? AND resource_id IN ?", u.UID, ids).
			Pluck("resource_id", &likedIDs)
		likedSet := make(map[int]bool, len(likedIDs))
		for _, id := range likedIDs {
			likedSet[id] = true
		}
		resource.IsLiked = likedSet[resource.ID]
		for i := range recs {
			recs[i].IsLiked = likedSet[recs[i].ID]
		}

		var favCount int64
		h.db.Model(&patchModel.UserPatchFavoriteRelation{}).
			Where("user_id = ? AND galgame_id = ?", u.UID, resource.GalgameID).
			Count(&favCount)
		patchFavorited = favCount > 0
	}

	return response.OK(c, map[string]any{
		"resource":          resource,
		"patch":             patchCard,
		"recommendations":   recs,
		"patch_is_favorite": patchFavorited,
	})
}

// Creator-application endpoints (Apply / GetApplyStatus) were removed alongside
// the creator role itself in the OAuth migration.

// ===== Hikari External API =====

// GetHikari GET /api/hikari
func (h *CommonHandler) GetHikari(c *fiber.Ctx) error {
	vndbID := c.Query("vndb_id")
	if vndbID == "" {
		return response.Error(c, errors.ErrBadRequest("vndb_id is required"))
	}

	var patch patchModel.Patch
	if err := h.db.Where("vndb_id = ?", vndbID).First(&patch).Error; err != nil {
		return response.Error(c, errors.ErrNotFound("patch not found"))
	}

	// Get resources but strip S3 download content
	var resources []patchModel.PatchResource
	h.db.Where("galgame_id = ?", patch.ID).Find(&resources)

	for i := range resources {
		if resources[i].Storage == "s3" {
			resources[i].Content = ""
		}
	}
	patchModel.RenderResourceNotes(resources)

	return response.OK(c, map[string]any{
		"patch":     patch,
		"resources": resources,
	})
}

// ===== Ranking =====

// rankingUser is the public-safe shape of a row on the user ranking page.
type rankingUser struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Avatar        string `json:"avatar"`
	Moemoepoint   int    `json:"moemoepoint"`
	PatchCount    int64  `json:"patch_count"`
	ResourceCount int64  `json:"resource_count"`
	CommentCount  int64  `json:"comment_count"`
}

// GetUserRanking GET /api/ranking/user
//
// Top 60 users sorted by one of:
//   - moemoepoint (default)
//   - patch        — count of patches the user owns
//   - resource     — count of resources the user owns
//   - comment      — count of comments the user authored
//
// timeRange is accepted for API parity with the legacy frontend but currently
// ignored ("all" is the only behavior). Aggregate counts are computed in one
// query so we do not pay an N+1 over 60 users.
func (h *CommonHandler) GetUserRanking(c *fiber.Ctx) error {
	sortBy := c.Query("sort_by", c.Query("sortBy", "moemoepoint"))

	const limit = 60
	// row holds only the local-side aggregates; name/avatar are filled later
	// from OAuth /users/batch since they are no longer in the local user table.
	type row struct {
		ID            int    `gorm:"column:id"`
		Moemoepoint   int    `gorm:"column:moemoepoint"`
		PatchCount    int64  `gorm:"column:patch_count"`
		ResourceCount int64  `gorm:"column:resource_count"`
		CommentCount  int64  `gorm:"column:comment_count"`
	}

	orderBy := "u.moemoepoint DESC"
	switch sortBy {
	case "patch", "patch_count":
		orderBy = "patch_count DESC, u.moemoepoint DESC"
	case "resource", "resource_count":
		orderBy = "resource_count DESC, u.moemoepoint DESC"
	case "comment", "comment_count":
		orderBy = "comment_count DESC, u.moemoepoint DESC"
	}

	var rows []row
	err := h.db.Table(`"user" u`).
		Select(`u.id, u.moemoepoint,
			COALESCE((SELECT COUNT(*) FROM patch p WHERE p.user_id = u.id), 0) AS patch_count,
			COALESCE((SELECT COUNT(*) FROM patch_resource pr WHERE pr.user_id = u.id), 0) AS resource_count,
			COALESCE((SELECT COUNT(*) FROM patch_comment pc WHERE pc.user_id = u.id), 0) AS comment_count`).
		Order(orderBy).
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	uids := make([]int, 0, len(rows))
	for _, r := range rows {
		uids = append(uids, r.ID)
	}
	briefs := userclient.BriefMapByInt(c.Context(), h.users, uids)

	out := make([]rankingUser, 0, len(rows))
	for _, r := range rows {
		ru := rankingUser{
			ID:            r.ID,
			Moemoepoint:   r.Moemoepoint,
			PatchCount:    r.PatchCount,
			ResourceCount: r.ResourceCount,
			CommentCount:  r.CommentCount,
		}
		if b := briefs[r.ID]; b != nil {
			// Skip banned users (status != 0); the local "u.status = 0" filter
			// has been removed since status now lives on OAuth.
			if b.Status != 0 {
				continue
			}
			ru.Name = b.Name
			ru.Avatar = b.Avatar
		}
		out = append(out, ru)
	}
	return response.OK(c, out)
}

// GetPatchRanking GET /api/ranking/patch
//
// Top 60 patches sorted by view / download / favorite. Results are passed
// through the enricher so each row carries the same shape the frontend uses
// elsewhere on the site.
func (h *CommonHandler) GetPatchRanking(c *fiber.Ctx) error {
	sortBy := c.Query("sort_by", c.Query("sortBy", "view"))

	column := "view"
	switch sortBy {
	case "download":
		column = "download"
	case "favorite", "favorite_by", "favorite_count":
		column = "favorite_count"
	}

	var patches []patchModel.Patch
	err := h.db.Model(&patchModel.Patch{}).
		Where("status = 0").
		Order(fmt.Sprintf("%s DESC", column)).
		Limit(60).
		Find(&patches).Error
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.OK(c, enricher.EnrichPatches(c.Context(), h.wiki, h.users, patches))
}

// GetMoyuHasPatch GET /api/moyu/patch/has-patch
func (h *CommonHandler) GetMoyuHasPatch(c *fiber.Ctx) error {
	var vndbIDs []string
	h.db.Model(&patchModel.Patch{}).
		Joins("JOIN patch_resource ON patch_resource.galgame_id = patch.id").
		Where("patch.vndb_id IS NOT NULL").
		Distinct("patch.vndb_id").
		Pluck("patch.vndb_id", &vndbIDs)

	return response.OK(c, vndbIDs)
}
