package common

import (
	"context"
	"fmt"
	"math/rand"
	"time"

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
// Returns "recent activity on this service" (latest 12 patches + 6 resources
// + 6 comments). NSFW filtering follows the wiki content_limit protocol
// (docs/galgame_wiki/00-handbook §16): the `content_limit` query parameter
// is forwarded to wiki via the enricher; missing / invalid query falls back
// to "sfw" — the home page is the single biggest SEO surface and must be
// safe-by-default for anonymous crawlers.
func (h *CommonHandler) GetHome(c *fiber.Ctx) error {
	cl := utils.ContentLimitForListBrowse(c)

	var patches []patchModel.Patch
	var resources []patchModel.PatchResource
	var comments []patchModel.PatchComment

	h.db.Model(&patchModel.Patch{}).Order("created DESC, id DESC").Limit(12).Find(&patches)
	// status = 0: don't promote disabled resources (pulled for virus etc.) in the home feed.
	h.db.Model(&patchModel.PatchResource{}).Where("status = 0").Order("created DESC, id DESC").Limit(6).Find(&resources)
	// status = 0: hide comments pending review (comment-verify) from the home feed.
	h.db.Model(&patchModel.PatchComment{}).Where("status = 0").Order("created DESC, id DESC").Limit(6).Find(&comments)

	// NSFW filter for the secondary slices BEFORE we render/attach anything —
	// no point rendering a resource note whose owning patch is about to be
	// hidden, and the attach step would otherwise leak the NSFW patch's name
	// into the response payload via comment.Patch / resource.Patch summaries.
	resources = enricher.FilterByGalgameContentLimit(c.Context(), h.wiki, resources, func(r patchModel.PatchResource) int { return r.GalgameID }, cl)
	comments = enricher.FilterByGalgameContentLimit(c.Context(), h.wiki, comments, func(m patchModel.PatchComment) int { return m.GalgameID }, cl)

	patchModel.RenderResourceNotes(resources)
	for i := range comments {
		comments[i].ContentHTML = markdown.MustRender(comments[i].Content)
	}
	h.attachResourceUsers(c.Context(), resources)
	h.attachCommentUsers(c.Context(), comments)
	h.attachPatchSummaries(c, comments, resources)
	// Home cards never render download links/secrets — strip them so this
	// public feed can't be scraped for download URLs / codes / passwords
	// (the rate-limited /patch/resource/:id/link is the only reveal surface).
	patchModel.StripResourceSecrets(resources)

	return response.OK(c, homeResponse{
		Galgames:  enricher.EnrichPatches(c.Context(), h.wiki, h.users, patches, cl),
		Resources: resources,
		Comments:  comments,
	})
}

// ===== Galgame List =====

type galgameListRequest struct {
	SelectedType string `query:"selected_type" validate:"required,min=1,max=107"`
	SortField    string `query:"sort_field" validate:"required,oneof=resource_update_time created view download release_date"`
	SortOrder    string `query:"sort_order" validate:"required,oneof=asc desc"`
	Page         int    `query:"page" validate:"required,min=1"`
	Limit        int    `query:"limit" validate:"required,min=1,max=24"`
	// 发售日期筛选 (YYYY / YYYY-MM)。格式不在 validator 里校验（YYYY vs
	// YYYY-MM 二选一不好用 oneof 表达）——交给 utils.ParseRelease*Bound，
	// 非法输入在 handler 里返回 400（对齐 wiki §17.1 的 loud-reject）。
	ReleasedFrom string `query:"released_from"`
	ReleasedTo   string `query:"released_to"`
	// 不连续月份集合 (CSV 1-12，如 "3,7,12")，叠加在年份区间上的 AND 过滤
	// (wiki §17.10)。本地 SQL: EXTRACT(MONTH FROM release_date) IN (...)。
	ReleasedMonths string `query:"released_months"`
}

// GetGalgameList GET /api/galgame
//
// Local-side filters: translation type + sort. NSFW filter follows the wiki
// content_limit protocol (docs/galgame_wiki/00-handbook §16): forwarded to
// wiki during enrichment, default "sfw" when unspecified (safe-by-default
// for anonymous browse). The reported `total` is the pre-filter local row
// count — filtering happens after the page slice is drawn, so the trailing
// pages can return a short slice when many rows in that range are NSFW.
// Callers should not rely on `total == sum(len(galgames))`.
func (h *CommonHandler) GetGalgameList(c *fiber.Ctx) error {
	var req galgameListRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	cl := utils.ContentLimitForListBrowse(c)

	// 发售日期边界 (YYYY / YYYY-MM → date)。malformed → 400 per wiki §17.1.
	lower, err := utils.ParseReleaseLowerBound(req.ReleasedFrom)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	upper, err := utils.ParseReleaseUpperBound(req.ReleasedTo)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	// 不连续月份集合 (CSV 1-12)。malformed → 400 per wiki §17.10.
	months, err := utils.ParseMonthSet(req.ReleasedMonths)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	// Independent statements for Count vs Find — see gorm v2 reuse footgun
	// documented in message/repository.go GetMessages.
	base := h.db.Model(&patchModel.Patch{})
	if req.SelectedType != "all" {
		base = base.Where("type @> ?", fmt.Sprintf(`["%s"]`, req.SelectedType))
	}
	// release_date filter. Setting either bound auto-excludes NULL rows (PG
	// >= / <= against NULL is UNKNOWN → dropped), which matches §17.4: "筛
	// 2024 年" means games with a *known* 2024 date. Both Count and Find see
	// these WHEREs because they're applied to `base` before the Session fork.
	if lower != nil {
		base = base.Where("release_date >= ?", *lower)
	}
	if upper != nil {
		base = base.Where("release_date <= ?", *upper)
	}
	// released_months: orthogonal AND filter on top of the year range (§17.10).
	// EXTRACT(MONTH FROM NULL) is NULL → not IN → NULL rows drop, same as the
	// range filter. Non-sargable but only re-checks the candidate set the
	// release_date btree range already narrowed.
	if len(months) > 0 {
		base = base.Where("EXTRACT(MONTH FROM release_date)::int IN ?", months)
	}

	var total int64
	base.Session(&gorm.Session{}).Count(&total)

	var patches []patchModel.Patch
	if err := base.Session(&gorm.Session{}).Order(fmt.Sprintf("%s %s, id DESC", req.SortField, req.SortOrder)).
		Offset((req.Page - 1) * req.Limit).Limit(req.Limit).
		Find(&patches).Error; err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	return response.OK(c, map[string]any{
		"galgames": enricher.EnrichPatches(c.Context(), h.wiki, h.users, patches, cl),
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
	cl := utils.ContentLimitForListBrowse(c)

	var comments []patchModel.PatchComment
	var total int64

	// status = 0: hide comments pending review (comment-verify) from the global list.
	base := h.db.Model(&patchModel.PatchComment{}).Where("status = 0")
	base.Session(&gorm.Session{}).Count(&total)

	err := base.Session(&gorm.Session{}).Order(fmt.Sprintf("%s %s, id DESC", req.SortField, req.SortOrder)).
		Offset((req.Page - 1) * req.Limit).Limit(req.Limit).
		Find(&comments).Error

	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	// Drop comments whose owning patch is NSFW (under the caller's
	// content_limit). attachPatchSummaries would otherwise leak the NSFW
	// patch's name/banner via the comment.Patch summary field.
	comments = enricher.FilterByGalgameContentLimit(c.Context(), h.wiki, comments, func(m patchModel.PatchComment) int { return m.GalgameID }, cl)

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
// NSFW filter follows the wiki content_limit protocol (default sfw). Filtered
// AFTER the page slice is drawn: `total` is the unfiltered local count, so a
// page can return fewer rows than the limit when many in that range belong to
// NSFW patches. Acceptable trade-off — alternative would be a per-page wiki
// pre-pass that doesn't scale.
func (h *CommonHandler) GetGlobalResources(c *fiber.Ctx) error {
	var req resourceListRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	cl := utils.ContentLimitForListBrowse(c)

	var resources []patchModel.PatchResource
	var total int64

	// status = 0: disabled resources are excluded from the global list.
	base := h.db.Model(&patchModel.PatchResource{}).Where("status = 0")
	base.Session(&gorm.Session{}).Count(&total)

	sortField := req.SortField
	if sortField == "like" {
		sortField = "like_count"
	}

	err := base.Session(&gorm.Session{}).Order(fmt.Sprintf("patch_resource.%s %s, patch_resource.id DESC", sortField, req.SortOrder)).
		Offset((req.Page - 1) * req.Limit).Limit(req.Limit).
		Find(&resources).Error

	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	resources = enricher.FilterByGalgameContentLimit(c.Context(), h.wiki, resources, func(r patchModel.PatchResource) int { return r.GalgameID }, cl)
	patchModel.RenderResourceNotes(resources)
	h.attachResourceUsers(c.Context(), resources)
	h.attachPatchSummaries(c, nil, resources)
	// Global resource feed cards never render the download payload — strip it
	// so this public, paginated (whole table) feed can't be bulk-scraped for
	// download URLs / codes / passwords, which would defeat the rate-limited
	// /patch/resource/:id/link reveal endpoint.
	patchModel.StripResourceSecrets(resources)
	return response.Paginated(c, resources, total)
}

// GetResourceDetail GET /api/resource/:id
//
// Returns the resource with its owning patch enriched via Wiki, plus up to 5
// recommended resources from the same patch. The frontend renders the patch
// header (name / banner / vndb_id) from the `patch` field.
//
// NSFW: forwards content_limit to wiki for the owning patch (default sfw).
// If the owning patch is filtered out → 404, so a NSFW resource never leaks
// out via this detail surface. Recommendations are filtered the same way:
// any rec whose galgame_id wiki doesn't return is dropped.
func (h *CommonHandler) GetResourceDetail(c *fiber.Ctx) error {
	cl := utils.ContentLimitForListBrowse(c)

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
		patchCard = enricher.EnrichPatch(c.Context(), h.wiki, h.users, &patch, cl)
	}
	if patchCard == nil {
		// Owning patch is missing / filtered (NSFW under a sfw caller). Don't
		// surface the resource on its own — it would mean "here's a NSFW
		// patch's resource link minus the cover image", which is still data
		// exfiltration. 404 mirrors what wiki itself does for a filtered :gid.
		return response.Error(c, errors.ErrNotFound("resource not found"))
	}

	// Recommendations — mirrors next-web /resource/detail:
	//   1. up to 5 other resources of the SAME patch (status=0, by download)
	//   2. if fewer than 5, top up with random popular resources from OTHER
	//      patches (status=0, download > 500), shuffled.
	const recTarget = 5
	var recs []patchModel.PatchResource
	h.db.Where("galgame_id = ? AND id != ? AND status = 0", resource.GalgameID, resource.ID).
		Order("download DESC, id DESC").Limit(recTarget).Find(&recs)

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

	// NSFW-filter the recommendations: the cross-patch top-up bucket can
	// include NSFW games unrelated to the resource's own patch. Same-patch
	// recs share resource.GalgameID which already passed the patchCard check.
	recs = enricher.FilterByGalgameContentLimit(c.Context(), h.wiki, recs, func(r patchModel.PatchResource) int { return r.GalgameID }, cl)

	resource.NoteHTML = markdown.MustRender(resource.Note)
	patchModel.RenderResourceNotes(recs)

	// Disabled resource (status != 0): withhold the download payload so the
	// link can't be fetched from the detail page either (mirrors the /link
	// endpoint's 403). The row is still returned — the frontend shows a 已禁用
	// notice instead of the (now empty) download links.
	if resource.Status != 0 {
		resource.Content = ""
		resource.S3Key = ""
		resource.Code = ""
		resource.Password = ""
	}

	// Attach publisher briefs to the main resource and the recommendations.
	one := []patchModel.PatchResource{resource}
	h.attachResourceUsers(c.Context(), one)
	resource = one[0]
	h.attachResourceUsers(c.Context(), recs)
	// Recommendation cards only show name/note/stats — strip their download
	// payload so the recs sidebar can't be walked to harvest links/secrets.
	// The main `resource` keeps them: it is the intended single-reveal surface
	// (the detail page renders its download links directly).
	patchModel.StripResourceSecrets(recs)

	// Viewer-specific state (if logged in): is_liked on the main resource +
	// recommendations, and is_favorite on the owning patch — so the
	// redesigned hero renders the like/favorite buttons in their real state.
	patchFavorited := false
	if u := middleware.GetUser(c); u != nil && u.ID > 0 {
		ids := make([]int, 0, len(recs)+1)
		ids = append(ids, resource.ID)
		for i := range recs {
			ids = append(ids, recs[i].ID)
		}
		var likedIDs []int
		h.db.Model(&patchModel.UserPatchResourceLikeRelation{}).
			Where("user_id = ? AND resource_id IN ?", u.ID, ids).
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
			Where("user_id = ? AND galgame_id = ?", u.ID, resource.GalgameID).
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

// The Hikari shapes mirror the LEGACY next-api/hikari contract so existing
// partner integrations keep working: the `{success, message, data}` envelope,
// `data` = the patch with a nested `resource` array, and the legacy field names
// (released / hash / patch_id …).
//
// The public `user` object ({id, name, avatar}) is carried on the patch + each
// resource, exactly as legacy did — partners render the uploader and link to
// /user/:id, so dropping it crashed their `patch.user.id` access.
//
// One deliberate departure, for safety: NO download secrets. `content` / `code`
// / `password` / `s3_key` are dropped (legacy already omitted `content`; we also
// drop code/password). The real download stays behind moyu's rate-limited
// reveal flow.
//
// Legacy patch fields that no longer exist locally (name / banner /
// introduction / engine — now wiki-sourced) are omitted; a partner that queries
// by vndb_id already holds those from VNDB.
type hikariEnvelope struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// hikariUser is the public uploader brief — exactly the legacy KunUser shape
// ({id, name, avatar}). Avatar is the full URL from OAuth (usable directly as an
// <img src>). Nothing more (no roles / bio / email / uuid) leaves the service.
type hikariUser struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

type hikariResource struct {
	ID         int                  `json:"id"`
	Storage    string               `json:"storage"`
	Name       string               `json:"name"`
	ModelName  string               `json:"model_name"`
	Size       string               `json:"size"`
	Note       string               `json:"note"`
	Hash       string               `json:"hash"`
	Type       patchModel.JSONArray `json:"type"`
	Language   patchModel.JSONArray `json:"language"`
	Platform   patchModel.JSONArray `json:"platform"`
	Download   int                  `json:"download"`
	Status     int                  `json:"status"`
	UpdateTime time.Time            `json:"update_time"`
	UserID     int                  `json:"user_id"`
	PatchID    int                  `json:"patch_id"`
	Created    time.Time            `json:"created"`
	User       hikariUser           `json:"user"`
}

type hikariPatch struct {
	ID                 int                  `json:"id"`
	VndbID             string               `json:"vndb_id"`
	Released           string               `json:"released"`
	Status             int                  `json:"status"`
	Download           int                  `json:"download"`
	View               int                  `json:"view"`
	ResourceUpdateTime time.Time            `json:"resource_update_time"`
	Type               patchModel.JSONArray `json:"type"`
	Language           patchModel.JSONArray `json:"language"`
	Platform           patchModel.JSONArray `json:"platform"`
	UserID             int                  `json:"user_id"`
	Created            time.Time            `json:"created"`
	Updated            time.Time            `json:"updated"`
	User               hikariUser           `json:"user"`
	Resource           []hikariResource     `json:"resource"`
}

// hikariFail writes the legacy error envelope ({success:false, data:null}).
func hikariFail(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(hikariEnvelope{Success: false, Message: message, Data: nil})
}

// GetHikari GET /api/hikari?vndb_id=...
//
// Public partner API — CORS allowlist + rate limit are applied at the route
// (see router.go). Response mirrors the legacy contract (see the type comments
// above) MINUS uploader identity + download secrets.
//
// NSFW: NOT gated. Like the legacy API, Hikari returns every patch by vndb_id
// regardless of the galgame's NSFW rating — partner sites (touchgal, shionlib,
// hikarinagi, …) are galgame sites themselves and need the full catalog. The
// content_limit / SEO gate that protects the public browse endpoints does NOT
// apply here, so this handler never calls the wiki.
func (h *CommonHandler) GetHikari(c *fiber.Ctx) error {
	vndbID := c.Query("vndb_id")
	if vndbID == "" {
		return hikariFail(c, fiber.StatusBadRequest, "Missing required parameter: vndb_id")
	}

	var patch patchModel.Patch
	if err := h.db.Where("vndb_id = ?", vndbID).First(&patch).Error; err != nil {
		return hikariFail(c, fiber.StatusNotFound, "No patch found for VNDB ID: "+vndbID)
	}

	// status = 0 only: never expose a disabled resource via the external API.
	var resources []patchModel.PatchResource
	h.db.Where("galgame_id = ? AND status = 0", patch.ID).Find(&resources)

	// Public uploader briefs (id/name/avatar) for the patch + every resource —
	// the legacy contract carried these and partners render/link the uploader.
	// Best-effort: on an OAuth miss the brief is nil and we fall back to just the
	// id (still the real, non-zero user id) so a partner's `user.id` never breaks.
	uids := make([]int, 0, len(resources)+1)
	uids = append(uids, patch.UserID)
	for i := range resources {
		uids = append(uids, resources[i].UserID)
	}
	briefs := userclient.BriefMapByInt(c.Context(), h.users, uids)
	toUser := func(uid int) hikariUser {
		if b := briefs[uid]; b != nil {
			return hikariUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar}
		}
		return hikariUser{ID: uid}
	}

	out := make([]hikariResource, 0, len(resources))
	for i := range resources {
		r := &resources[i]
		out = append(out, hikariResource{
			ID:         r.ID,
			Storage:    r.Storage,
			Name:       r.Name,
			ModelName:  r.ModelName,
			Size:       r.Size,
			Note:       r.Note,
			Hash:       r.Blake3,
			Type:       r.Type,
			Language:   r.Language,
			Platform:   r.Platform,
			Download:   r.Download,
			Status:     r.Status,
			UpdateTime: r.UpdateTime,
			UserID:     r.UserID,
			PatchID:    r.GalgameID,
			Created:    r.Created,
			User:       toUser(r.UserID),
		})
	}

	released := ""
	if patch.ReleaseDate != nil {
		released = patch.ReleaseDate.Format("2006-01-02")
	}

	return c.JSON(hikariEnvelope{
		Success: true,
		Message: "Patch found successfully",
		Data: hikariPatch{
			ID:                 patch.ID,
			VndbID:             patch.VndbID,
			Released:           released,
			Status:             patch.Status,
			Download:           patch.Download,
			View:               patch.View,
			ResourceUpdateTime: patch.ResourceUpdateTime,
			Type:               patch.Type,
			Language:           patch.Language,
			Platform:           patch.Platform,
			UserID:             patch.UserID,
			Created:            patch.Created,
			Updated:            patch.Updated,
			User:               toUser(patch.UserID),
			Resource:           out,
		},
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
		ID            int   `gorm:"column:id"`
		Moemoepoint   int   `gorm:"column:moemoepoint"`
		PatchCount    int64 `gorm:"column:patch_count"`
		ResourceCount int64 `gorm:"column:resource_count"`
		CommentCount  int64 `gorm:"column:comment_count"`
	}

	// id tiebreaker (u.id DESC) appended to every branch for stable ordering
	// when two users share the primary sort key (moemoepoint / count).
	orderBy := "u.moemoepoint DESC, u.id DESC"
	switch sortBy {
	case "patch", "patch_count":
		orderBy = "patch_count DESC, u.moemoepoint DESC, u.id DESC"
	case "resource", "resource_count":
		orderBy = "resource_count DESC, u.moemoepoint DESC, u.id DESC"
	case "comment", "comment_count":
		orderBy = "comment_count DESC, u.moemoepoint DESC, u.id DESC"
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
// elsewhere on the site. NSFW filter follows the wiki content_limit protocol
// (default sfw for SEO safety); the returned slice may have fewer than 60
// rows when NSFW games dominate the top of the ranking.
func (h *CommonHandler) GetPatchRanking(c *fiber.Ctx) error {
	cl := utils.ContentLimitForListBrowse(c)
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
		Order(fmt.Sprintf("%s DESC, id DESC", column)).
		Limit(60).
		Find(&patches).Error
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.OK(c, enricher.EnrichPatches(c.Context(), h.wiki, h.users, patches, cl))
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
