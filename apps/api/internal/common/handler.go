package common

import (
	"fmt"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/galgame/enricher"
	"kun-galgame-patch-api/internal/infrastructure/markdown"
	"kun-galgame-patch-api/internal/middleware"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	userModel "kun-galgame-patch-api/internal/user/model"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type CommonHandler struct {
	db   *gorm.DB
	wiki *galgameClient.Client
}

func NewHandler(db *gorm.DB, wiki *galgameClient.Client) *CommonHandler {
	return &CommonHandler{db: db, wiki: wiki}
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

	h.db.Model(&patchModel.Patch{}).Order("created DESC").Limit(12).
		Preload("User", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "avatar")
		}).Find(&patches)

	h.db.Model(&patchModel.PatchResource{}).Order("created DESC").Limit(6).
		Preload("User", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "avatar")
		}).Find(&resources)

	h.db.Model(&patchModel.PatchComment{}).Order("created DESC").Limit(6).
		Preload("User", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "avatar")
		}).Find(&comments)

	patchModel.RenderResourceNotes(resources)
	for i := range comments {
		comments[i].ContentHTML = markdown.MustRender(comments[i].Content)
	}
	h.attachPatchSummaries(c, comments, resources)

	return response.OK(c, homeResponse{
		Galgames:  enricher.EnrichPatches(c.Context(), h.wiki, patches),
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

	query := h.db.Model(&patchModel.Patch{})
	if req.SelectedType != "all" {
		query = query.Where("type @> ?", fmt.Sprintf(`["%s"]`, req.SelectedType))
	}

	var total int64
	query.Count(&total)

	var patches []patchModel.Patch
	err := query.Order(fmt.Sprintf("%s %s", req.SortField, req.SortOrder)).
		Offset((req.Page - 1) * req.Limit).Limit(req.Limit).
		Preload("User", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "avatar")
		}).Find(&patches).Error
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	return response.OK(c, map[string]any{
		"galgames": enricher.EnrichPatches(c.Context(), h.wiki, patches),
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

	query := h.db.Model(&patchModel.PatchComment{})
	query.Count(&total)

	err := query.Order(fmt.Sprintf("%s %s", req.SortField, req.SortOrder)).
		Offset((req.Page - 1) * req.Limit).Limit(req.Limit).
		Preload("User", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "avatar")
		}).Find(&comments).Error

	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	for i := range comments {
		comments[i].ContentHTML = markdown.MustRender(comments[i].Content)
	}
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

	query := h.db.Model(&patchModel.PatchResource{})
	query.Count(&total)

	sortField := req.SortField
	if sortField == "like" {
		sortField = "like_count"
	}

	err := query.Order(fmt.Sprintf("patch_resource.%s %s", sortField, req.SortOrder)).
		Offset((req.Page - 1) * req.Limit).Limit(req.Limit).
		Preload("User", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "avatar")
		}).Find(&resources).Error

	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	patchModel.RenderResourceNotes(resources)
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
	if dbErr := h.db.Preload("User", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "name", "avatar")
	}).First(&resource, resourceID).Error; dbErr != nil {
		return response.Error(c, errors.ErrNotFound("resource not found"))
	}

	// Fetch the owning patch and enrich it via Wiki so the frontend has
	// name / banner / vndb_id without making a separate call.
	var patch patchModel.Patch
	var patchCard *enricher.GalgameCard
	if err := h.db.Preload("User", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "name", "avatar")
	}).First(&patch, resource.GalgameID).Error; err == nil {
		card := enricher.EnrichPatch(c.Context(), h.wiki, &patch)
		patchCard = &card
	}

	// Get up to 5 recommendations from the same patch
	var recs []patchModel.PatchResource
	h.db.Where("galgame_id = ? AND id != ?", resource.GalgameID, resource.ID).
		Limit(5).Order("like_count DESC").Find(&recs)

	resource.NoteHTML = markdown.MustRender(resource.Note)
	patchModel.RenderResourceNotes(recs)

	return response.OK(c, map[string]any{
		"resource":        resource,
		"patch":           patchCard,
		"recommendations": recs,
	})
}

// ===== Creator Application =====

// Apply POST /api/apply
func (h *CommonHandler) Apply(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)

	// Check minimum resource count
	var resourceCount int64
	h.db.Model(&patchModel.PatchResource{}).Where("user_id = ?", user.UID).Count(&resourceCount)
	if resourceCount < 3 {
		return response.Error(c, errors.ErrBadRequest("need at least 3 published resources"))
	}

	// Check for pending application
	var pendingCount int64
	h.db.Model(&userModel.UserMessage{}).
		Where("type = 'apply' AND sender_id = ? AND status = 0", user.UID).
		Count(&pendingCount)
	if pendingCount > 0 {
		return response.Error(c, errors.ErrBadRequest("you already have a pending application"))
	}

	msg := &userModel.UserMessage{
		Type:     "apply",
		Content:  fmt.Sprintf("Creator application from user %d", user.UID),
		Status:   0,
		SenderID: &user.UID,
	}
	h.db.Create(msg)
	return response.OKMessage(c, "Application submitted")
}

// GetApplyStatus GET /api/apply/status
func (h *CommonHandler) GetApplyStatus(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)

	var resourceCount int64
	h.db.Model(&patchModel.PatchResource{}).Where("user_id = ?", user.UID).Count(&resourceCount)

	return response.OK(c, map[string]any{
		"resource_count": resourceCount,
		"role":           user.Role,
	})
}

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
	type row struct {
		ID            int    `gorm:"column:id"`
		Name          string `gorm:"column:name"`
		Avatar        string `gorm:"column:avatar"`
		Moemoepoint   int    `gorm:"column:moemoepoint"`
		PatchCount    int64  `gorm:"column:patch_count"`
		ResourceCount int64  `gorm:"column:resource_count"`
		CommentCount  int64  `gorm:"column:comment_count"`
	}

	orderBy := "moemoepoint DESC"
	switch sortBy {
	case "patch", "patch_count":
		orderBy = "patch_count DESC, u.moemoepoint DESC"
	case "resource", "resource_count":
		orderBy = "resource_count DESC, u.moemoepoint DESC"
	case "comment", "comment_count":
		orderBy = "comment_count DESC, u.moemoepoint DESC"
	default:
		orderBy = "u.moemoepoint DESC"
	}

	var rows []row
	err := h.db.Table(`"user" u`).
		Select(`u.id, u.name, u.avatar, u.moemoepoint,
			COALESCE((SELECT COUNT(*) FROM patch p WHERE p.user_id = u.id), 0) AS patch_count,
			COALESCE((SELECT COUNT(*) FROM patch_resource pr WHERE pr.user_id = u.id), 0) AS resource_count,
			COALESCE((SELECT COUNT(*) FROM patch_comment pc WHERE pc.user_id = u.id), 0) AS comment_count`).
		Where("u.status = 0").
		Order(orderBy).
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	out := make([]rankingUser, len(rows))
	for i, r := range rows {
		out[i] = rankingUser(r)
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
		Preload("User", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "avatar")
		}).Find(&patches).Error
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.OK(c, enricher.EnrichPatches(c.Context(), h.wiki, patches))
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
