package handler

import (
	"strconv"

	"kun-galgame-patch-api/internal/admin/dto"
	"kun-galgame-patch-api/internal/admin/service"
	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/galgame/enricher"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

type AdminHandler struct {
	service *service.AdminService
	wiki    *galgameClient.Client
}

func New(svc *service.AdminService, wiki *galgameClient.Client) *AdminHandler {
	return &AdminHandler{service: svc, wiki: wiki}
}

func getIDParam(c *fiber.Ctx, name string) (int, error) {
	id, err := strconv.Atoi(c.Params(name))
	if err != nil || id < 1 {
		return 0, errors.ErrBadRequest("invalid ID")
	}
	return id, nil
}

// ===== Comments =====

// GetComments GET /api/admin/comment
func (h *AdminHandler) GetComments(c *fiber.Ctx) error {
	var req dto.AdminPaginationRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	comments, total, err := h.service.GetComments(req.Search, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, comments, total)
}

// UpdateComment PUT /api/admin/comment/:id
func (h *AdminHandler) UpdateComment(c *fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.AdminUpdateCommentRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	admin := middleware.MustGetUser(c)
	if err := h.service.UpdateComment(id, req.Content, admin.UID); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OKMessage(c, "Comment updated")
}

// DeleteComment DELETE /api/admin/comment/:id
func (h *AdminHandler) DeleteComment(c *fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	admin := middleware.MustGetUser(c)
	if err := h.service.DeleteComment(id, admin.UID); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OKMessage(c, "Comment deleted")
}

// ===== Resources =====

// GetResources GET /api/admin/resource
func (h *AdminHandler) GetResources(c *fiber.Ctx) error {
	var req dto.AdminPaginationRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	resources, total, err := h.service.GetResources(req.Search, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, resources, total)
}

// UpdateResource PUT /api/admin/resource/:id
func (h *AdminHandler) UpdateResource(c *fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.AdminUpdateResourceRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	admin := middleware.MustGetUser(c)
	if err := h.service.UpdateResource(id, req.Note, admin.UID); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OKMessage(c, "Resource updated")
}

// DeleteResource DELETE /api/admin/resource/:id
func (h *AdminHandler) DeleteResource(c *fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	admin := middleware.MustGetUser(c)
	if err := h.service.DeleteResource(id, admin.UID); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OKMessage(c, "Resource deleted")
}

// ===== Users =====

// GetUsers GET /api/admin/user
func (h *AdminHandler) GetUsers(c *fiber.Ctx) error {
	var req dto.AdminPaginationRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	users, total, err := h.service.GetUsers(req.Search, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, users, total)
}

// UpdateUser PUT /api/admin/user/:uid
func (h *AdminHandler) UpdateUser(c *fiber.Ctx) error {
	uid, err := getIDParam(c, "uid")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.AdminUpdateUserRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	admin := middleware.MustGetUser(c)
	fields := map[string]any{
		"name":              req.Name,
		"role":              req.Role,
		"status":            req.Status,
		"daily_image_count": req.DailyImageCount,
		"bio":               req.Bio,
	}

	if err := h.service.UpdateUser(uid, fields, admin.UID, admin.Role); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OKMessage(c, "User updated")
}

// DeleteUser DELETE /api/admin/user/:uid
func (h *AdminHandler) DeleteUser(c *fiber.Ctx) error {
	uid, err := getIDParam(c, "uid")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	admin := middleware.MustGetUser(c)
	if err := h.service.DeleteUser(uid, admin.UID); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OKMessage(c, "User deleted")
}

// ===== All Patches (admin browse) =====

// GetGalgame GET /api/admin/galgame
//
// Lists every patch with pagination. The optional `search` query is matched
// against vndb_id (game names live in Wiki and cannot be searched locally; the
// admin frontend pairs this list with the per-row "open detail" link to drill
// down). Each row is enriched via Wiki so the admin sees the same name/banner
// they see elsewhere on the site.
func (h *AdminHandler) GetGalgame(c *fiber.Ctx) error {
	var req dto.AdminPaginationRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	patches, total, err := h.service.GetAllPatches(req.Search, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	cards := enricher.EnrichPatches(c.Context(), h.wiki, patches)
	return response.Paginated(c, cards, total)
}

// ===== Creator Applications =====

// GetCreatorApplications GET /api/admin/creator
func (h *AdminHandler) GetCreatorApplications(c *fiber.Ctx) error {
	var req dto.AdminPaginationRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	apps, total, err := h.service.GetCreatorApplications(req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, apps, total)
}

// ApproveCreator PUT /api/admin/creator/:messageId/approve
func (h *AdminHandler) ApproveCreator(c *fiber.Ctx) error {
	messageID, err := getIDParam(c, "messageId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.ApproveCreatorRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	admin := middleware.MustGetUser(c)
	if err := h.service.ApproveCreator(messageID, req.UID, admin.UID); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OKMessage(c, "Creator approved")
}

// DeclineCreator PUT /api/admin/creator/:messageId/decline
func (h *AdminHandler) DeclineCreator(c *fiber.Ctx) error {
	messageID, err := getIDParam(c, "messageId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.DeclineCreatorRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	admin := middleware.MustGetUser(c)
	if err := h.service.DeclineCreator(messageID, admin.UID, req.Reason); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OKMessage(c, "Creator declined")
}

// ===== Settings =====

// GetCommentVerify GET /api/admin/setting/comment-verify
func (h *AdminHandler) GetCommentVerify(c *fiber.Ctx) error {
	return response.OK(c, map[string]bool{"enabled": h.service.GetSetting("admin:enable_comment_verify")})
}

// SetCommentVerify PUT /api/admin/setting/comment-verify
func (h *AdminHandler) SetCommentVerify(c *fiber.Ctx) error {
	var req dto.AdminSettingBoolRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	h.service.SetSetting("admin:enable_comment_verify", req.Enabled)
	return response.OKMessage(c, "Setting updated")
}

// GetCreatorOnly GET /api/admin/setting/creator-only
func (h *AdminHandler) GetCreatorOnly(c *fiber.Ctx) error {
	return response.OK(c, map[string]bool{"enabled": h.service.GetSetting("admin:enable_only_creator_create")})
}

// SetCreatorOnly PUT /api/admin/setting/creator-only
func (h *AdminHandler) SetCreatorOnly(c *fiber.Ctx) error {
	var req dto.AdminSettingBoolRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	h.service.SetSetting("admin:enable_only_creator_create", req.Enabled)
	return response.OKMessage(c, "Setting updated")
}

// GetRegisterDisabled GET /api/admin/setting/register
func (h *AdminHandler) GetRegisterDisabled(c *fiber.Ctx) error {
	return response.OK(c, map[string]bool{"disabled": h.service.GetSetting("admin:disable_register")})
}

// SetRegisterDisabled PUT /api/admin/setting/register
func (h *AdminHandler) SetRegisterDisabled(c *fiber.Ctx) error {
	var req dto.AdminSettingBoolRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	h.service.SetSetting("admin:disable_register", req.Enabled)
	return response.OKMessage(c, "Setting updated")
}

// ===== Stats =====

// GetStats GET /api/admin/stats
func (h *AdminHandler) GetStats(c *fiber.Ctx) error {
	var req dto.AdminStatsRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, h.service.GetStats(req.Days))
}

// GetStatsSum GET /api/admin/stats/sum
func (h *AdminHandler) GetStatsSum(c *fiber.Ctx) error {
	return response.OK(c, h.service.GetStatsSum())
}

// ===== Logs =====

// GetLogs GET /api/admin/log
func (h *AdminHandler) GetLogs(c *fiber.Ctx) error {
	var req dto.AdminPaginationRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	logs, total, err := h.service.GetLogs(req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, logs, total)
}

// ===== Orphan Patches (D12) =====

// GetOrphanPatches GET /api/admin/patch/orphans
//
// Lists all patches with galgame_id=0, i.e. "orphans" whose galgame cannot be found in Wiki.
// For each row, the admin can:
//   - Rebind the correct vndb_id via PUT /api/patch/:id (will re-verify with Wiki /galgame/check)
//   - Or DELETE /api/patch/:id to remove
//   - If vndb_id is real but not yet created in Wiki, create the galgame in Wiki first, then rebind
//
// Alongside `items`, the response also returns pending_count (vndb_id empty = pending-N)
// and bad_vndb_count (vndb_id format is valid but missing in Wiki).
func (h *AdminHandler) GetOrphanPatches(c *fiber.Ctx) error {
	var req dto.AdminPaginationRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	items, total, err := h.service.GetOrphanPatches(req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	pending, badVndb, _ := h.service.CountOrphanPatches()
	return response.OK(c, map[string]any{
		"items":           items,
		"total":           total,
		"pending_count":   pending,
		"bad_vndb_count":  badVndb,
	})
}
