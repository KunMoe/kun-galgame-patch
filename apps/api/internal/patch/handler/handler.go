package handler

import (
	"encoding/json"
	stderrors "errors"
	"io"
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
			out = append(out, model.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar, AvatarImageHash: b.AvatarImageHash, Roles: b.Roles})
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
