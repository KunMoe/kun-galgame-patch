package handler

import (
	"regexp"
	"strconv"

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

// ===== Patch CRUD =====

// CreatePatch POST /api/patch
//
// D12 (2026-04-21): the request body is simplified to JSON { "vndb_id": "vXXX" }.
// The server calls Wiki /galgame/check to verify and fetch the galgame_id to persist locally.
//
// The legacy "creator-only" gate (role >= 2 / role == "creator") was removed
// when the creator role was retired in the OAuth migration; any logged-in
// user may now create a patch.
func (h *PatchHandler) CreatePatch(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)

	var req dto.PatchCreateRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if !vndbIDRegex.MatchString(req.VndbID) {
		return response.Error(c, errors.ErrBadRequest("vndb_id 格式不合法（应为 vXXX）"))
	}

	id, err := h.service.CreatePatch(c.Context(), user.UID, req.VndbID)
	if err != nil {
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
func (h *PatchHandler) GetPatch(c *fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	patch, err := h.service.GetPatch(id)
	if err != nil {
		return response.Error(c, errors.ErrNotFound("patch not found"))
	}

	card := headerCard{GalgameCard: enricher.EnrichPatch(c.Context(), h.wiki, h.users, patch)}
	if user := middleware.GetUser(c); user != nil {
		card.IsFavorite = h.service.IsFavorited(user.UID, id)
	}
	return response.OK(c, card)
}

// GetPatchDetail GET /api/patch/:id/detail
//
// D12: detail enrichment goes through Wiki /galgame/:gid to additionally fetch intro / tag_ids / official_ids.
func (h *PatchHandler) GetPatchDetail(c *fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	patch, err := h.service.GetPatchDetail(id)
	if err != nil {
		return response.Error(c, errors.ErrNotFound("patch not found"))
	}
	return response.OK(c, enricher.EnrichPatchDetail(c.Context(), h.wiki, h.users, patch))
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
	if err := h.service.UpdatePatch(c.Context(), id, user.UID, isPrivileged, req.VndbID); err != nil {
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
	if err := h.service.DeletePatch(id, user.UID, isAdmin); err != nil {
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
func (h *PatchHandler) GetComments(c *fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.GetPatchCommentRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	currentUID := middleware.GetUID(c)
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
	comment, err := h.service.CreateComment(patchID, user.UID, req.Content, req.ParentID)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	// Background notifications
	go func() {
		h.service.CreateMentionMessages(user.UID, patchID, req.Content)
		h.service.CreateCommentNotification(user.UID, comment)
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
	if err := h.service.UpdateComment(commentID, user.UID, req.Content); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	return response.OKMessage(c, "Comment updated")
}

// DeleteComment DELETE /api/patch/comment/:commentId
func (h *PatchHandler) DeleteComment(c *fiber.Ctx) error {
	commentID, err := getIDParam(c, "commentId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	user := middleware.MustGetUser(c)
	isPrivileged := middleware.HasAnyRole(c, "admin", "moderator")
	if err := h.service.DeleteComment(commentID, user.UID, isPrivileged); err != nil {
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
	liked, err := h.service.ToggleCommentLike(commentID, user.UID)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	return response.OK(c, map[string]bool{"liked": liked})
}

// GetCommentMarkdown GET /api/patch/comment/:commentId/markdown
func (h *PatchHandler) GetCommentMarkdown(c *fiber.Ctx) error {
	commentID, err := getIDParam(c, "commentId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	md, err := h.service.GetCommentMarkdown(commentID)
	if err != nil {
		return response.Error(c, errors.ErrNotFound("comment not found"))
	}

	return response.OK(c, map[string]string{"markdown": md})
}

// ===== Resources =====

// GetResources GET /api/patch/:id/resource
func (h *PatchHandler) GetResources(c *fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	currentUID := middleware.GetUID(c)
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

	if err := h.service.CreateResource(resource, user.UID); err != nil {
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

	var req dto.PatchResourceCreateRequest
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

	if err := h.service.UpdateResource(resourceID, user.UID, update); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	return response.OKMessage(c, "Resource updated")
}

// DeleteResource DELETE /api/patch/resource/:resourceId
func (h *PatchHandler) DeleteResource(c *fiber.Ctx) error {
	resourceID, err := getIDParam(c, "resourceId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	user := middleware.MustGetUser(c)
	if err := h.service.DeleteResource(resourceID, user.UID); err != nil {
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
	if err := h.service.ToggleResourceDisable(resourceID, user.UID, isPrivileged); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	return response.OKMessage(c, "OK")
}

// IncrementResourceDownload PUT /api/patch/resource/:resourceId/download
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
	liked, err := h.service.ToggleResourceLike(resourceID, user.UID)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	return response.OK(c, map[string]bool{"liked": liked})
}

// ===== Favorites =====

// ToggleFavorite PUT /api/patch/:id/favorite
func (h *PatchHandler) ToggleFavorite(c *fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	user := middleware.MustGetUser(c)
	favorited, err := h.service.ToggleFavorite(id, user.UID)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
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

	ids, err := h.service.GetContributorIDs(id)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	briefs := userclient.BriefMapByInt(c.Context(), h.users, ids)
	out := make([]model.PatchUser, 0, len(ids))
	for _, uid := range ids {
		if b := briefs[uid]; b != nil {
			out = append(out, model.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar})
		}
	}
	return response.OK(c, out)
}

// GetRandomPatch GET /api/home/random
func (h *PatchHandler) GetRandomPatch(c *fiber.Ctx) error {
	id, err := h.service.GetRandomPatchID()
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.OK(c, map[string]int{"id": id})
}

// UpdateGalgame PUT /api/v1/galgame/:gid
//
// Thin proxy over the Wiki Service's PUT /galgame/:gid. The Wiki Service owns
// all galgame metadata (D12); editing it from this site means forwarding the
// user's request — together with their OAuth access_token — and propagating
// Wiki's response code verbatim back to the frontend.
//
// We do not enforce authorization locally: Wiki itself permits only the
// creator or an admin. Local side effects (e.g. moemoepoint rewards on edit)
// are intentionally not added here to keep this a pure proxy; if we want
// them later they go on success after the Wiki call returns.
func (h *PatchHandler) UpdateGalgame(c *fiber.Ctx) error {
	gid, err := getIDParam(c, "gid")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	accessToken := middleware.GetAccessToken(c)
	if accessToken == "" {
		return response.Error(c, errors.ErrUnauthorized())
	}

	// Decode body into the Wiki client's request shape so callers cannot smuggle
	// unsupported keys (e.g. vndb_id which Wiki blocks on update anyway).
	var req galgameClient.UpdateGalgameRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, errors.ErrBadRequest("无法解析请求体"))
	}

	data, err := h.wiki.UpdateGalgame(c.Context(), accessToken, gid, &req)
	if err != nil {
		if werr, ok := err.(*galgameClient.WikiError); ok {
			// Forward Wiki's business code (4xxxx / 6xxxx) without remapping.
			return response.Error(c, errors.New(werr.Code, werr.Message, fiber.StatusBadRequest))
		}
		return response.Error(c, errors.ErrInternal("调用 Galgame Wiki 失败"))
	}
	return c.JSON(response.Response{Code: 0, Message: "OK", Data: data})
}
