package handler

import (
	"encoding/json"
	stderrors "errors"
	"io"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/galgame/enricher"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/internal/patch/dto"
	"kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/internal/patch/service"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/userclient"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Non-creators need a vndb_id; in all cases we require a well-formed vndb_id.
var vndbIDRegex = regexp.MustCompile(`^v\d+$`)

type PatchHandler struct {
	service *service.PatchService
	wiki    *galgameClient.Client
	users   *userclient.Client
}

func New(svc *service.PatchService, wiki *galgameClient.Client, users *userclient.Client) *PatchHandler {
	return &PatchHandler{service: svc, wiki: wiki, users: users}
}

func getIDParam(c *fiber.Ctx, name string) (int, error) {
	id, err := strconv.Atoi(c.Params(name))
	if err != nil || id < 1 {
		return 0, errors.ErrBadRequest("invalid ID")
	}
	return id, nil
}

// gatePatchByContentLimit checks whether `patchID`'s owning galgame passes the
// caller's content_limit filter. Returns true → handler may serve the data;
// false → handler MUST respond 404 (mirrors how /api/patch/:id itself handles
// the NSFW miss).
//
// Used by sub-endpoints (/comment, /resource, /contributor, comment/:id/markdown)
// that don't go through the main enricher but still expose patch-coupled data
// — comment text, resource notes, contributor list, etc. A direct call to e.g.
// /api/patch/<nsfw-id>/comment must not list comments from anonymous (sfw)
// callers, even though the parent detail endpoint already 404s for them.
//
// Defaults to sfw via ContentLimitForListBrowse: an anonymous crawler with no
// content_limit query gets the SEO-safe path. Wiki transient failure fails
// closed (return false) — same SEO-safety reasoning as enricher / FilterBy.
func (h *PatchHandler) gatePatchByContentLimit(c *fiber.Ctx, patchID int) bool {
	cl := utils.ContentLimitForListBrowse(c)
	if cl == "" || h.wiki == nil {
		return true
	}
	briefs, err := h.wiki.GalgameBatch(c.Context(), []int{patchID}, cl)
	if err != nil {
		return false
	}
	return len(briefs) > 0
}

// ===== Patch CRUD =====

// ensureCanPublishGalgame enforces the admin "仅创作者(role>2)可发布 Galgame"
// toggle: when on, only moderators / admins may publish. Returns a 403
// AppError to block, or nil to allow. Applied to every publish entry point
// (CreatePatch / ClaimGalgame / SubmitGalgame) so the gate can't be bypassed
// by hitting a different publish route.
func (h *PatchHandler) ensureCanPublishGalgame(c *fiber.Ctx) *errors.AppError {
	if h.service.IsCreatorOnlyEnabled() && !middleware.HasAnyRole(c, "admin", "moderator") {
		return errors.New(40300, "本站当前仅允许版主 / 管理员发布 Galgame", fiber.StatusForbidden)
	}
	return nil
}

// CreatePatch POST /api/patch
//
// D12 (2026-04-21): the request body is simplified to JSON { "vndb_id": "vXXX" }.
// The server calls Wiki /galgame/check to verify and fetch the galgame_id to persist locally.
//
// Publish gate: by default any logged-in user may create a patch (the legacy
// hard-coded "creator" role gate was retired with that role in the OAuth
// migration). The admin "仅创作者(role>2)可发布" toggle can re-enable a gate —
// see ensureCanPublishGalgame, applied to every publish entry point.
func (h *PatchHandler) CreatePatch(c *fiber.Ctx) error {
	if appErr := h.ensureCanPublishGalgame(c); appErr != nil {
		return response.Error(c, appErr)
	}
	user := middleware.MustGetUser(c)

	var req dto.PatchCreateRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if !vndbIDRegex.MatchString(req.VndbID) {
		return response.Error(c, errors.ErrBadRequest("vndb_id 格式不合法（应为 vXXX）"))
	}

	id, err := h.service.CreatePatch(c.Context(), user.ID, req.VndbID)
	if err != nil {
		// Distinct error code so the frontend can render a "前往 Wiki 创建"
		// CTA when the vndb_id is missing on Wiki, vs the generic toast for
		// any other failure (e.g. duplicate vndb_id locally).
		if stderrors.Is(err, service.ErrWikiGalgameMissing) {
			return response.Error(c, errors.ErrWikiGalgameNotFound(""))
		}
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, map[string]int{"id": id})
}

// headerCard flattens GalgameCard + is_favorite to match the frontend PatchHeader shape.
type headerCard struct {
	enricher.GalgameCard
	IsFavorite bool `json:"is_favorite"`
}

// GetPatch GET /api/patch/:id
//
// D12: return the flat GalgameCard structure directly (no longer wrapped in patch / is_favorite layers).
// Frontend PatchHeader = GalgameCard + isFavorite.
//
// NSFW: forwards content_limit to wiki (default sfw — moyu is stricter than
// wiki's "detail default = no filter" because *moyu's* detail surface is what
// the search engine indexes). When wiki filters this id out, the enricher
// returns nil and we 404 — the same shape as a missing patch.
func (h *PatchHandler) GetPatch(c *fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	patch, err := h.service.GetPatch(c.Context(), id)
	if err != nil {
		return response.Error(c, errors.ErrNotFound("patch not found"))
	}

	enriched := enricher.EnrichPatch(c.Context(), h.wiki, h.users, patch, utils.ContentLimitForListBrowse(c))
	if enriched == nil {
		return response.Error(c, errors.ErrNotFound("patch not found"))
	}

	card := headerCard{GalgameCard: *enriched}
	if user := middleware.GetUser(c); user != nil {
		card.IsFavorite = h.service.IsFavorited(user.ID, id)
	}
	return response.OK(c, card)
}

// GetPatchDetail GET /api/patch/:id/detail
//
// D12: detail enrichment goes through Wiki /galgame/:gid to additionally fetch intro / tag_ids / official_ids.
//
// NSFW: same gating as GetPatch — content_limit forwarded to wiki, default
// sfw, nil from enricher → 404. The introduction_html / tags / officials this
// endpoint emits are the biggest single NSFW surface in moyu's SSR output
// (Google indexes the full intro text), so 404'ing on a filter miss matters
// even more here than on GetPatch.
func (h *PatchHandler) GetPatchDetail(c *fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	patch, err := h.service.GetPatchDetail(c.Context(), id)
	if err != nil {
		return response.Error(c, errors.ErrNotFound("patch not found"))
	}
	enriched := enricher.EnrichPatchDetail(c.Context(), h.wiki, h.users, patch, utils.ContentLimitForListBrowse(c))
	if enriched == nil {
		return response.Error(c, errors.ErrNotFound("patch not found"))
	}
	return response.OK(c, enriched)
}

// UpdatePatch PUT /api/patch/:id
//
// After D12 this only permits "rebinding vndb_id" (creator or role >= 3 only).
// Game name/introduction/banner etc. all live in Wiki; this endpoint no longer accepts them.
func (h *PatchHandler) UpdatePatch(c *fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.PatchUpdateRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if !vndbIDRegex.MatchString(req.VndbID) {
		return response.Error(c, errors.ErrBadRequest("vndb_id 格式不合法"))
	}

	user := middleware.MustGetUser(c)
	isPrivileged := middleware.HasAnyRole(c, "admin", "moderator")
	if err := h.service.UpdatePatch(c.Context(), id, user.ID, isPrivileged, req.VndbID); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OKMessage(c, "Patch updated")
}

// DeletePatch DELETE /api/patch/:id
func (h *PatchHandler) DeletePatch(c *fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	user := middleware.MustGetUser(c)
	isAdmin := middleware.HasRole(c, "admin")
	if err := h.service.DeletePatch(id, user.ID, isAdmin); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	return response.OKMessage(c, "Patch deleted")
}

// CheckDuplicate GET /api/patch/duplicate
func (h *PatchHandler) CheckDuplicate(c *fiber.Ctx) error {
	var req dto.DuplicateCheckRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	exists, err := h.service.CheckDuplicate(req.VndbID)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	return response.OK(c, map[string]bool{"exists": exists})
}

// IncrementView PUT /api/patch/:id/view
func (h *PatchHandler) IncrementView(c *fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	h.service.IncrementView(id)
	return response.OKMessage(c, "OK")
}

// ===== Comments =====

// GetComments GET /api/patch/:id/comment
//
// NSFW gate: same shape as GetPatch — anonymous (sfw) callers see 404 on a
// NSFW patch's comment list, so direct hits to /patch/<nsfw>/comment can't
// bypass the SEO filter that already protects the parent detail endpoint.
func (h *PatchHandler) GetComments(c *fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	if !h.gatePatchByContentLimit(c, id) {
		return response.Error(c, errors.ErrNotFound("patch not found"))
	}

	var req dto.GetPatchCommentRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	currentUID := middleware.GetUserID(c)
	comments, total, err := h.service.GetComments(c.Context(), id, currentUID, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	return response.Paginated(c, comments, total)
}

// CreateComment POST /api/patch/:id/comment
func (h *PatchHandler) CreateComment(c *fiber.Ctx) error {
	patchID, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.PatchCommentCreateRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	req.GalgameID = patchID

	user := middleware.MustGetUser(c)
	comment, err := h.service.CreateComment(patchID, user.ID, req.Content, req.ParentID)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	// Background notifications — only for immediately-visible comments. When
	// the comment is pending review (comment-verify), notifications are
	// deferred to ApproveComment so we don't ping mentioned users / the patch
	// owner about a comment that may be rejected.
	if comment.Status == 0 {
		go func() {
			h.service.CreateMentionMessages(user.ID, patchID, comment.ID, req.Content)
			h.service.CreateCommentNotification(user.ID, comment)
		}()
	}

	return response.OK(c, comment)
}

// ApproveComment PUT /api/admin/comment/:id/approve
//
// Flips a pending comment (comment-verify) to approved, applying the deferred
// visible-comment side effects (comment_count, owner moemoepoint, contributor)
// and firing the deferred mention / comment notifications. Idempotent.
// Registered under the moderator-gated /admin group.
func (h *PatchHandler) ApproveComment(c *fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	comment, aerr := h.service.ApproveComment(id)
	if aerr != nil {
		return response.Error(c, errors.ErrBadRequest(aerr.Error()))
	}
	go func() {
		h.service.CreateMentionMessages(comment.UserID, comment.GalgameID, comment.ID, comment.Content)
		h.service.CreateCommentNotification(comment.UserID, comment)
	}()
	return response.OK(c, comment)
}

// UpdateComment PUT /api/patch/comment/:commentId
func (h *PatchHandler) UpdateComment(c *fiber.Ctx) error {
	commentID, err := getIDParam(c, "commentId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.PatchCommentUpdateRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	user := middleware.MustGetUser(c)
	comment, err := h.service.UpdateComment(commentID, user.ID, req.Content)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	return response.OK(c, comment)
}

// DeleteComment DELETE /api/patch/comment/:commentId
func (h *PatchHandler) DeleteComment(c *fiber.Ctx) error {
	commentID, err := getIDParam(c, "commentId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	user := middleware.MustGetUser(c)
	isPrivileged := middleware.HasAnyRole(c, "admin", "moderator")
	if err := h.service.DeleteComment(commentID, user.ID, isPrivileged); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	return response.OKMessage(c, "Comment deleted")
}

// ToggleCommentLike PUT /api/patch/comment/:commentId/like
func (h *PatchHandler) ToggleCommentLike(c *fiber.Ctx) error {
	commentID, err := getIDParam(c, "commentId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	user := middleware.MustGetUser(c)
	liked, err := h.service.ToggleCommentLike(commentID, user.ID)
	if err != nil {
		// The only error this returns is "comment not found" → 404, not 400
		// (audit F034).
		return response.Error(c, errors.ErrNotFound(err.Error()))
	}

	return response.OK(c, map[string]bool{"liked": liked})
}

// GetCommentMarkdown GET /api/patch/comment/:commentId/markdown
//
// NSFW gate: look up the comment's owning patch and apply the same
// content_limit check the parent /patch/:id/comment list applies. Without
// this an anonymous caller who knows a NSFW comment id could fetch its raw
// markdown — same exfiltration surface as GetComments itself.
func (h *PatchHandler) GetCommentMarkdown(c *fiber.Ctx) error {
	commentID, err := getIDParam(c, "commentId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	patchID, pErr := h.service.GetCommentPatchID(commentID)
	if pErr != nil {
		return response.Error(c, errors.ErrNotFound("comment not found"))
	}
	if !h.gatePatchByContentLimit(c, patchID) {
		return response.Error(c, errors.ErrNotFound("comment not found"))
	}

	md, err := h.service.GetCommentMarkdown(commentID)
	if err != nil {
		return response.Error(c, errors.ErrNotFound("comment not found"))
	}

	return response.OK(c, map[string]string{"markdown": md})
}

// LocateComment GET /api/patch/comment/:commentId/locate?limit=N
//
// Resolves a comment id to {page, root_id, is_reply, galgame_id} in the
// paginated root-comment list so a deep-link can jump straight to it.
// NSFW-gated like GetCommentMarkdown so a NSFW comment's location can't be
// probed anonymously.
func (h *PatchHandler) LocateComment(c *fiber.Ctx) error {
	commentID, err := getIDParam(c, "commentId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	patchID, pErr := h.service.GetCommentPatchID(commentID)
	if pErr != nil {
		return response.Error(c, errors.ErrNotFound("comment not found"))
	}
	if !h.gatePatchByContentLimit(c, patchID) {
		return response.Error(c, errors.ErrNotFound("comment not found"))
	}

	limit, _ := strconv.Atoi(c.Query("limit", "30"))
	res, lErr := h.service.LocateComment(commentID, limit)
	if lErr != nil {
		return response.Error(c, errors.ErrNotFound("comment not found"))
	}
	return response.OK(c, res)
}

// ===== Resources =====

// GetResources GET /api/patch/:id/resource
//
// NSFW gate: same as GetComments. Resource notes / titles may describe NSFW
// content explicitly, so listing them under a NSFW patch must 404 for sfw
// callers — even though the resource rows themselves don't carry
// content_limit (the field lives on the owning patch via wiki).
func (h *PatchHandler) GetResources(c *fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	if !h.gatePatchByContentLimit(c, id) {
		return response.Error(c, errors.ErrNotFound("patch not found"))
	}

	currentUID := middleware.GetUserID(c)
	resources, err := h.service.GetResources(c.Context(), id, currentUID)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	return response.OK(c, resources)
}

// CreateResource POST /api/patch/:id/resource
func (h *PatchHandler) CreateResource(c *fiber.Ctx) error {
	patchID, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.PatchResourceCreateRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	user := middleware.MustGetUser(c)
	resource := &model.PatchResource{
		GalgameID: patchID,
		Storage:   req.Storage,
		Name:      req.Name,
		ModelName: req.ModelName,
		S3Key:     req.S3Key,
		Content:   req.Content,
		Size:      req.Size,
		Code:      req.Code,
		Password:  req.Password,
		Note:      req.Note,
		Type:      model.JSONArray(req.Type),
		Language:  model.JSONArray(req.Language),
		Platform:  model.JSONArray(req.Platform),
	}

	if err := h.service.CreateResource(c.Context(), resource, user.ID); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	return response.OK(c, resource)
}

// UpdateResource PUT /api/patch/resource/:resourceId
func (h *PatchHandler) UpdateResource(c *fiber.Ctx) error {
	resourceID, err := getIDParam(c, "resourceId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.PatchResourceUpdateRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	user := middleware.MustGetUser(c)
	update := &model.PatchResource{
		Storage:   req.Storage,
		Name:      req.Name,
		ModelName: req.ModelName,
		S3Key:     req.S3Key,
		Content:   req.Content,
		Size:      req.Size,
		Code:      req.Code,
		Password:  req.Password,
		Note:      req.Note,
		Type:      model.JSONArray(req.Type),
		Language:  model.JSONArray(req.Language),
		Platform:  model.JSONArray(req.Platform),
	}

	// Snapshot the actor's privilege so the file-history row records who+role
	// at time of edit (MOYU-PR5 / M3). Mirrors the Wiki revision convention
	// (3=admin / 2=mod / 1=user / 0=unknown).
	actorRole := 1
	if middleware.HasAnyRole(c, "admin") {
		actorRole = 3
	} else if middleware.HasAnyRole(c, "moderator") {
		actorRole = 2
	}

	updated, err := h.service.UpdateResource(c.Context(), resourceID, user.ID, update, req.Reason, actorRole)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	// Return the fully-rendered row (with new note_html, update_time, user
	// brief) so the frontend can replace its local list entry directly
	// instead of patching together a partial merge — that path used to keep
	// the old note_html and confused the user ("note 改了但简介没变").
	return response.OK(c, updated)
}

// DeleteResource DELETE /api/patch/resource/:resourceId
func (h *PatchHandler) DeleteResource(c *fiber.Ctx) error {
	resourceID, err := getIDParam(c, "resourceId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	user := middleware.MustGetUser(c)
	// Option B: privileged users (moderator / admin) can delete any resource
	// from the public page; non-privileged callers fall through to the
	// owner check inside the service.
	isPrivileged := middleware.HasAnyRole(c, "admin", "moderator")
	if err := h.service.DeleteResource(resourceID, user.ID, isPrivileged); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	return response.OKMessage(c, "Resource deleted")
}

// ToggleResourceDisable PUT /api/patch/resource/:resourceId/disable
func (h *PatchHandler) ToggleResourceDisable(c *fiber.Ctx) error {
	resourceID, err := getIDParam(c, "resourceId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	user := middleware.MustGetUser(c)
	isPrivileged := middleware.HasAnyRole(c, "admin", "moderator")
	status, err := h.service.ToggleResourceDisable(resourceID, user.ID, isPrivileged)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	return response.OK(c, fiber.Map{"status": status})
}

// IncrementResourceDownload PUT /api/patch/resource/:resourceId/download
// GetResourceDownloadInfo GET /api/patch/resource/:resourceId/link
//
// Minimal payload for the "获取资源链接" reveal on the patch resource list:
// only the storage type + download links + secrets. No Wiki enrichment, no
// recommendations, no blake3 (the card already shows the hash).
func (h *PatchHandler) GetResourceDownloadInfo(c *fiber.Ctx) error {
	resourceID, err := getIDParam(c, "resourceId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	r, gErr := h.service.GetResourceDownloadInfo(resourceID)
	if gErr != nil {
		return response.Error(c, errors.ErrNotFound("resource not found"))
	}
	// NSFW gate: matches the resource-list endpoint's gate so an anonymous
	// caller can't hop directly to the download link of a NSFW patch's
	// resource by knowing its id. The download URL itself (B2 / user link)
	// would otherwise be leaked even though no patch metadata is.
	if !h.gatePatchByContentLimit(c, r.GalgameID) {
		return response.Error(c, errors.ErrNotFound("resource not found"))
	}
	// Disabled resources (status != 0) have their download link withheld — the
	// owner/admin pulled it (e.g. virus). The row stays visible (marked 已禁用)
	// but the link can't be fetched. Distinct 403 code so the frontend can show
	// a clear "已禁用" message instead of a generic failure.
	if r.Status != 0 {
		return response.Error(c, errors.New(40310, "该资源已被禁用，暂时无法下载", fiber.StatusForbidden))
	}
	return response.OK(c, fiber.Map{
		"storage":  r.Storage,
		"content":  r.Content,
		"code":     r.Code,
		"password": r.Password,
	})
}

func (h *PatchHandler) IncrementResourceDownload(c *fiber.Ctx) error {
	resourceID, err := getIDParam(c, "resourceId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	if err := h.service.IncrementResourceDownload(resourceID); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	return response.OKMessage(c, "OK")
}

// ToggleResourceLike PUT /api/patch/resource/:resourceId/like
func (h *PatchHandler) ToggleResourceLike(c *fiber.Ctx) error {
	resourceID, err := getIDParam(c, "resourceId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	user := middleware.MustGetUser(c)
	liked, err := h.service.ToggleResourceLike(resourceID, user.ID)
	if err != nil {
		// Only "resource not found" → 404, not 400 (audit F034).
		return response.Error(c, errors.ErrNotFound(err.Error()))
	}

	return response.OK(c, map[string]bool{"liked": liked})
}

// ToggleResourceFavorite PUT /api/patch/resource/:resourceId/favorite
//
// Per-resource SUBSCRIPTION — distinct from the resource LIKE (appreciation) and
// the galgame FAVORITE (notified on new resources). A subscriber gets a
// patchResourceUpdate notification when this resource's file/link changes.
func (h *PatchHandler) ToggleResourceFavorite(c *fiber.Ctx) error {
	resourceID, err := getIDParam(c, "resourceId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	user := middleware.MustGetUser(c)
	favorited, err := h.service.ToggleResourceFavorite(resourceID, user.ID)
	if err != nil {
		// Generic message — never leak the raw DB error (e.g. a missing-table
		// "relation does not exist" when migration 017 hasn't run) to the client.
		slog.Error("ToggleResourceFavorite failed", "resourceID", resourceID, "error", err)
		return response.Error(c, errors.ErrInternal("收藏失败，请稍后重试"))
	}

	return response.OK(c, map[string]bool{"favorited": favorited})
}

// ===== Favorites =====

// ToggleFavorite PUT /api/patch/:id/favorite
func (h *PatchHandler) ToggleFavorite(c *fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	user := middleware.MustGetUser(c)
	favorited, err := h.service.ToggleFavorite(id, user.ID)
	if err != nil {
		// Generic message — don't leak the raw DB error to the client.
		slog.Error("ToggleFavorite failed", "patchID", id, "error", err)
		return response.Error(c, errors.ErrInternal("收藏失败，请稍后重试"))
	}

	return response.OK(c, map[string]bool{"favorited": favorited})
}

// ===== Contributors =====

// GetContributors GET /api/patch/:id/contributor
//
// Returns publisher briefs (id/name/avatar) batch-resolved from OAuth
// /users/batch. The local DB only stores the contributor user_ids.
func (h *PatchHandler) GetContributors(c *fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	if !h.gatePatchByContentLimit(c, id) {
		return response.Error(c, errors.ErrNotFound("patch not found"))
	}

	ids, err := h.service.GetContributorIDs(id)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	briefs := userclient.BriefMapByInt(c.Context(), h.users, ids)
	out := make([]model.PatchUser, 0, len(ids))
	for _, userID := range ids {
		if b := briefs[userID]; b != nil {
			out = append(out, model.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar, AvatarImageHash: b.AvatarImageHash, Roles: b.Roles})
		}
	}
	return response.OK(c, out)
}

// GetRandomPatch GET /api/home/random
//
// NSFW: forwards content_limit so the random landing page can't dump a NSFW
// patch into an anonymous (sfw-default) browser session. Service drains a
// 60-row random sample through wiki batch and picks from the survivors.
func (h *PatchHandler) GetRandomPatch(c *fiber.Ctx) error {
	id, err := h.service.GetRandomPatchID(c.Context(), utils.ContentLimitForListBrowse(c), utils.IncludeEmptyGalgames(c))
	if err != nil {
		// "no candidate passes the content_limit filter" is a not-found, not a
		// server fault — return 404 instead of a 500 that trips alerting (audit
		// F083). The NSFW fail-closed logic is preserved.
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return response.Error(c, errors.ErrNotFound("no patch available"))
		}
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.OK(c, map[string]int{"id": id})
}

// UpdateGalgame PUT /api/v1/galgame/:gid
//
// Thin proxy over the Wiki Service's PUT /galgame/:gid. Two modes:
//
//   - application/json  -> forwards the JSON body unchanged.
//   - multipart/form-data with `data` (JSON) and optional `file` (banner)
//     -> forwards as multipart so Wiki/image_service can attach the banner
//     in the same revision (no orphan files; see docs/galgame_wiki/01-galgame.md
//     §Banner 上传).
//
// We do not enforce authorization locally: Wiki itself permits only the
// creator or an admin. The user's OAuth access_token is forwarded verbatim
// (carries the JWT roles claim Wiki validates).
func (h *PatchHandler) UpdateGalgame(c *fiber.Ctx) error {
	gid, err := getIDParam(c, "gid")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	accessToken := middleware.GetAccessToken(c)
	if accessToken == "" {
		return response.Error(c, errors.ErrUnauthorized())
	}

	ctype := string(c.Request().Header.ContentType())
	if strings.HasPrefix(ctype, "multipart/form-data") {
		return h.updateGalgameMultipart(c, gid, accessToken)
	}
	return h.updateGalgameJSON(c, gid, accessToken)
}

// updateGalgameJSON is the plain JSON path. Decoding into the client's
// pointer-fielded shape filters out unsupported keys (e.g. vndb_id, which
// Wiki rejects on update anyway).
func (h *PatchHandler) updateGalgameJSON(c *fiber.Ctx, gid int, accessToken string) error {
	var req galgameClient.UpdateGalgameRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, errors.ErrBadRequest("无法解析请求体"))
	}
	data, err := h.wiki.UpdateGalgame(c.Context(), accessToken, gid, &req)
	if err == nil {
		// Editing galgame info is a content update → bump moyu's 最近更新 sort key.
		h.service.TouchResourceUpdateTime(gid)
	}
	return writeWikiResult(c, data, err)
}

// updateGalgameMultipart reads `data` + optional `file` from the incoming
// multipart body and forwards them through the wiki client. Size cap is
// 10 MB (consistent with other image-upload paths in this project).
func (h *PatchHandler) updateGalgameMultipart(c *fiber.Ctx, gid int, accessToken string) error {
	form, err := c.MultipartForm()
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("multipart 表单解析失败"))
	}

	// data field: JSON string mirroring UpdateGalgameRequest. Re-decode into
	// the typed shape to strip unsupported keys before forwarding.
	dataStrs, _ := form.Value["data"]
	if len(dataStrs) == 0 {
		return response.Error(c, errors.ErrBadRequest("缺少 data 字段"))
	}
	var req galgameClient.UpdateGalgameRequest
	if err := json.Unmarshal([]byte(dataStrs[0]), &req); err != nil {
		return response.Error(c, errors.ErrBadRequest("data 字段不是合法 JSON"))
	}

	// file field is optional -- if absent, fall back to JSON mode so we don't
	// invent an empty multipart that Wiki could misinterpret.
	fileHeaders, _ := form.File["file"]
	if len(fileHeaders) == 0 {
		data, err := h.wiki.UpdateGalgame(c.Context(), accessToken, gid, &req)
		if err == nil {
			h.service.TouchResourceUpdateTime(gid)
		}
		return writeWikiResult(c, data, err)
	}

	fh := fileHeaders[0]
	if fh.Size > 10*1024*1024 {
		return response.Error(c, errors.ErrBadRequest("banner 超过 10MB 上限"))
	}
	f, err := fh.Open()
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无法读取上传文件"))
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("读取上传文件失败"))
	}
	mime := fh.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}

	data, err := h.wiki.UpdateGalgameMultipart(
		c.Context(), accessToken, gid, &req, fh.Filename, raw, mime,
	)
	if err == nil {
		h.service.TouchResourceUpdateTime(gid)
	}
	return writeWikiResult(c, data, err)
}

// writeWikiResult is the shared result -> response mapping for both modes.
// Wiki business errors (e.g. 80008 image quota, 60002 review rejected,
// 40300 forbidden) flow through as-is via WikiError so the frontend can
// render specific messages.
func writeWikiResult(c *fiber.Ctx, data json.RawMessage, err error) error {
	if err != nil {
		if werr, ok := err.(*galgameClient.WikiError); ok {
			return response.Error(c, errors.New(werr.Code, werr.Message, fiber.StatusBadRequest))
		}
		return response.Error(c, errors.ErrInternal("调用 Galgame Wiki 失败"))
	}
	return c.JSON(response.Response{Code: 0, Message: "OK", Data: data})
}

// ===== Wiki submission proxies (docs/galgame_wiki/07-submission.md) =====
//
// Each endpoint is a thin pass-through to Wiki: extract the user's
// access_token from the session, forward verbatim, surface Wiki's business
// errors as-is. The site backend does not re-implement authorization —
// Wiki decodes the JWT and enforces submitter / status rules itself.

// SubmitGalgame POST /api/v1/galgame/submit
//
// Two content types accepted (same shape as UpdateGalgame):
//   - application/json
//   - multipart/form-data with `data` + optional `file` for banner upload
func (h *PatchHandler) SubmitGalgame(c *fiber.Ctx) error {
	if appErr := h.ensureCanPublishGalgame(c); appErr != nil {
		return response.Error(c, appErr)
	}
	accessToken := middleware.GetAccessToken(c)
	if accessToken == "" {
		return response.Error(c, errors.ErrUnauthorized())
	}
	ctype := string(c.Request().Header.ContentType())
	if strings.HasPrefix(ctype, "multipart/form-data") {
		return h.submitGalgameMultipart(c, accessToken)
	}
	return h.submitGalgameJSON(c, accessToken)
}

func (h *PatchHandler) submitGalgameJSON(c *fiber.Ctx, accessToken string) error {
	var req galgameClient.SubmitGalgameRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, errors.ErrBadRequest("无法解析请求体"))
	}
	data, err := h.wiki.SubmitGalgame(c.Context(), accessToken, &req)
	return writeWikiResult(c, data, err)
}

func (h *PatchHandler) submitGalgameMultipart(c *fiber.Ctx, accessToken string) error {
	req, fileName, fileBytes, fileMime, err := parseGalgameMultipart(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	if len(fileBytes) == 0 {
		data, callErr := h.wiki.SubmitGalgame(c.Context(), accessToken, &req)
		return writeWikiResult(c, data, callErr)
	}
	data, callErr := h.wiki.SubmitGalgameMultipart(
		c.Context(), accessToken, &req, fileName, fileBytes, fileMime,
	)
	return writeWikiResult(c, data, callErr)
}

// ClaimGalgame POST /api/v1/galgame/:gid/claim
//
// Flip a VNDB draft (status=2) to published (status=0) on Wiki, then register
// the local patch row + award +3 moemoepoint atomically (handbook §9). The
// frontend must NOT additionally POST /patch — that produced a double +3.
//
// Response payload: { "id": <local patch id == galgame id> } so the frontend
// can navigate straight to /patch/:id without a second round-trip.
func (h *PatchHandler) ClaimGalgame(c *fiber.Ctx) error {
	if appErr := h.ensureCanPublishGalgame(c); appErr != nil {
		return response.Error(c, appErr)
	}
	gid, idErr := getIDParam(c, "gid")
	if idErr != nil {
		return response.Error(c, idErr.(*errors.AppError))
	}
	accessToken := middleware.GetAccessToken(c)
	if accessToken == "" {
		return response.Error(c, errors.ErrUnauthorized())
	}

	data, err := h.wiki.ClaimGalgame(c.Context(), accessToken, gid)
	if err != nil {
		if werr, ok := err.(*galgameClient.WikiError); ok {
			// Forward Wiki business code/message verbatim (e.g. 20006 草稿不可
			// 认领 when the Meilisearch row was stale). HTTP 400.
			return response.Error(c, errors.New(werr.Code, werr.Message, fiber.StatusBadRequest))
		}
		return response.Error(c, errors.ErrInternal("调用 Galgame Wiki 失败"))
	}

	// Wiki returned the published galgame (status=0). Pull the id/vndb_id so
	// we can create the local patch row keyed by galgame_id (== patch.id, D13).
	var claimed struct {
		ID     int    `json:"id"`
		VndbID string `json:"vndb_id"`
	}
	if jerr := json.Unmarshal(data, &claimed); jerr != nil || claimed.ID == 0 {
		// Wiki succeeded but we couldn't parse — fall back to the path param.
		claimed.ID = gid
	}

	userID := middleware.MustGetUser(c).ID
	patchID, regErr := h.service.RegisterClaimedGalgame(userID, claimed.ID, claimed.VndbID)
	if regErr != nil {
		// The Wiki-side claim already succeeded and cannot be rolled back; the
		// galgame is published and owned by the user. Surface a soft error so
		// the user can retry the (idempotent) local registration via the
		// detail page's first interaction, but don't pretend it failed wholesale.
		return response.Error(c, errors.ErrInternal("认领成功，但本站登记失败，请稍后重试"))
	}

	return c.JSON(response.Response{
		Code:    0,
		Message: "OK",
		Data:    fiber.Map{"id": patchID},
	})
}

// PatchGalgameDraft PATCH /api/v1/galgame/:gid
//
// Edit one's own pending/declined draft. Wiki auto-flips status=4 back to 3.
// Same dual-content-type as Submit/Update.
func (h *PatchHandler) PatchGalgameDraft(c *fiber.Ctx) error {
	gid, idErr := getIDParam(c, "gid")
	if idErr != nil {
		return response.Error(c, idErr.(*errors.AppError))
	}
	accessToken := middleware.GetAccessToken(c)
	if accessToken == "" {
		return response.Error(c, errors.ErrUnauthorized())
	}
	ctype := string(c.Request().Header.ContentType())
	if strings.HasPrefix(ctype, "multipart/form-data") {
		req, fileName, fileBytes, fileMime, err := parseGalgameMultipart(c)
		if err != nil {
			return response.Error(c, err.(*errors.AppError))
		}
		if len(fileBytes) == 0 {
			data, callErr := h.wiki.PatchGalgameDraft(c.Context(), accessToken, gid, &req)
			return writeWikiResult(c, data, callErr)
		}
		data, callErr := h.wiki.PatchGalgameDraftMultipart(
			c.Context(), accessToken, gid, &req, fileName, fileBytes, fileMime,
		)
		return writeWikiResult(c, data, callErr)
	}
	var req galgameClient.SubmitGalgameRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, errors.ErrBadRequest("无法解析请求体"))
	}
	data, err := h.wiki.PatchGalgameDraft(c.Context(), accessToken, gid, &req)
	return writeWikiResult(c, data, err)
}

// DeleteGalgameDraft DELETE /api/v1/galgame/:gid (draft)
//
// Hard-delete one's own pending/declined draft (Wiki enforces status ∈ {3,4}
// + submitter check). NOTE: this conflicts at the path level with the local
// patch DELETE /api/v1/patch/:id; we expose it under /galgame/:gid which is
// what Wiki uses, so the verb is unambiguous.
func (h *PatchHandler) DeleteGalgameDraft(c *fiber.Ctx) error {
	gid, idErr := getIDParam(c, "gid")
	if idErr != nil {
		return response.Error(c, idErr.(*errors.AppError))
	}
	accessToken := middleware.GetAccessToken(c)
	if accessToken == "" {
		return response.Error(c, errors.ErrUnauthorized())
	}
	if err := h.wiki.DeleteGalgameDraft(c.Context(), accessToken, gid); err != nil {
		if werr, ok := err.(*galgameClient.WikiError); ok {
			return response.Error(c, errors.New(werr.Code, werr.Message, fiber.StatusBadRequest))
		}
		return response.Error(c, errors.ErrInternal("调用 Galgame Wiki 失败"))
	}
	return response.OKMessage(c, "OK")
}

// ListMyGalgames GET /api/v1/galgame/mine
func (h *PatchHandler) ListMyGalgames(c *fiber.Ctx) error {
	accessToken := middleware.GetAccessToken(c)
	if accessToken == "" {
		return response.Error(c, errors.ErrUnauthorized())
	}
	status := c.Query("status", "")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if limit < 1 || limit > 50 {
		limit = 20
	}
	if page < 1 {
		page = 1
	}
	out, err := h.wiki.ListMyGalgames(c.Context(), accessToken, status, page, limit)
	if err != nil {
		if werr, ok := err.(*galgameClient.WikiError); ok {
			return response.Error(c, errors.New(werr.Code, werr.Message, fiber.StatusBadRequest))
		}
		return response.Error(c, errors.ErrInternal("调用 Galgame Wiki 失败"))
	}
	return response.OK(c, out)
}

// SearchGalgameForPublish GET /api/v1/galgame/search/publish
//
// Used by the publish wizard. Forwards the user's Bearer + include_pending=true
// so the response surfaces both public results and the caller's own
// pending/declined drafts.
func (h *PatchHandler) SearchGalgameForPublish(c *fiber.Ctx) error {
	accessToken := middleware.GetAccessToken(c)
	if accessToken == "" {
		return response.Error(c, errors.ErrUnauthorized())
	}
	q := c.Query("q", "")
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	if limit < 1 || limit > 24 {
		limit = 10
	}
	out, err := h.wiki.SearchGalgameForPublish(c.Context(), accessToken, q, limit)
	if err != nil {
		if werr, ok := err.(*galgameClient.WikiError); ok {
			return response.Error(c, errors.New(werr.Code, werr.Message, fiber.StatusBadRequest))
		}
		return response.Error(c, errors.ErrInternal("调用 Galgame Wiki 失败"))
	}
	return response.OK(c, out)
}

// GetWikiMessagesReadState GET /api/v1/galgame/messages/read-state
//
// Returns the caller's last-read marker for wiki notifications. Used by the
// notification-center to compute the unread badge (count of wiki messages
// with id > last_read_message_id). State lives locally — wiki doesn't
// maintain per-user read flags (see docs/galgame_wiki/08-messages.md §已读状态).
func (h *PatchHandler) GetWikiMessagesReadState(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	var lastRead int64
	row := h.service.DB().Raw(
		`SELECT last_read_message_id FROM wiki_message_read_state WHERE user_id = ?`,
		user.ID,
	).Row()
	_ = row.Scan(&lastRead)
	return response.OK(c, map[string]any{"last_read_message_id": lastRead})
}

// UpdateWikiMessagesReadState PUT /api/v1/galgame/messages/read-state
//
// Body: { "last_read_message_id": int64 }. We only move forward — submitting
// a smaller id is a no-op.
func (h *PatchHandler) UpdateWikiMessagesReadState(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	var body struct {
		LastReadMessageID int64 `json:"last_read_message_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.Error(c, errors.ErrBadRequest("无法解析请求体"))
	}
	if body.LastReadMessageID < 0 {
		return response.Error(c, errors.ErrBadRequest("last_read_message_id 不能为负"))
	}
	if err := h.service.DB().Exec(`
		INSERT INTO wiki_message_read_state(user_id, last_read_message_id, updated_at)
		VALUES (?, ?, NOW())
		ON CONFLICT(user_id) DO UPDATE
		SET last_read_message_id = GREATEST(wiki_message_read_state.last_read_message_id, EXCLUDED.last_read_message_id),
		    updated_at = NOW()
	`, user.ID, body.LastReadMessageID).Error; err != nil {
		return response.Error(c, errors.ErrInternal("保存已读状态失败"))
	}
	return response.OKMessage(c, "OK")
}

// GetMyWikiMessages GET /api/v1/galgame/messages/mine
func (h *PatchHandler) GetMyWikiMessages(c *fiber.Ctx) error {
	accessToken := middleware.GetAccessToken(c)
	if accessToken == "" {
		return response.Error(c, errors.ErrUnauthorized())
	}
	sinceID, _ := strconv.ParseInt(c.Query("since_id", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	out, err := h.wiki.GetMyWikiMessages(c.Context(), accessToken, sinceID, limit)
	if err != nil {
		if werr, ok := err.(*galgameClient.WikiError); ok {
			return response.Error(c, errors.New(werr.Code, werr.Message, fiber.StatusBadRequest))
		}
		return response.Error(c, errors.ErrInternal("调用 Galgame Wiki 失败"))
	}
	return response.OK(c, out)
}

// parseGalgameMultipart is shared between SubmitGalgame and PatchGalgameDraft.
// Returns the JSON body, the file name / bytes / mime if present, or an
// AppError describing what failed.
func parseGalgameMultipart(c *fiber.Ctx) (galgameClient.SubmitGalgameRequest, string, []byte, string, error) {
	var req galgameClient.SubmitGalgameRequest
	form, err := c.MultipartForm()
	if err != nil {
		return req, "", nil, "", errors.ErrBadRequest("multipart 表单解析失败")
	}
	dataStrs := form.Value["data"]
	if len(dataStrs) == 0 {
		return req, "", nil, "", errors.ErrBadRequest("缺少 data 字段")
	}
	if err := json.Unmarshal([]byte(dataStrs[0]), &req); err != nil {
		return req, "", nil, "", errors.ErrBadRequest("data 字段不是合法 JSON")
	}
	fileHeaders := form.File["file"]
	if len(fileHeaders) == 0 {
		return req, "", nil, "", nil
	}
	fh := fileHeaders[0]
	if fh.Size > 10*1024*1024 {
		return req, "", nil, "", errors.ErrBadRequest("banner 超过 10MB 上限")
	}
	f, err := fh.Open()
	if err != nil {
		return req, "", nil, "", errors.ErrBadRequest("无法读取上传文件")
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		return req, "", nil, "", errors.ErrBadRequest("读取上传文件失败")
	}
	mime := fh.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}
	return req, fh.Filename, raw, mime, nil
}

// GetResourceFileHistory GET /api/patch/resource/:resourceId/history
//
// Public, privacy-safe view of one resource's file-replacement audit
// (when / who-role / why / old size + hash). Deliberately omits the old
// download links + s3 key — those stay behind the rate-limited /link endpoint.
// Lets any visitor (incl. anonymous) see a resource's change history.
func (h *PatchHandler) GetResourceFileHistory(c *fiber.Ctx) error {
	resourceID, err := getIDParam(c, "resourceId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	var req dto.ResourceFileHistoryRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	items, total, gErr := h.service.GetResourceFileHistory(resourceID, req.Page, req.Limit)
	if gErr != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, items, total)
}

// GetResourceRevisions GET /api/patch/resource/:resourceId/revisions
//
// Public per-field edit history (diff) for one resource: each row is one edit
// with a list of {field, before, after}. Secret-free (see service). Lets any
// visitor see "language changed from X to Y", etc.
func (h *PatchHandler) GetResourceRevisions(c *fiber.Ctx) error {
	resourceID, err := getIDParam(c, "resourceId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	var req dto.ResourceFileHistoryRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	items, total, gErr := h.service.GetResourceRevisions(resourceID, req.Page, req.Limit)
	if gErr != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, items, total)
}
