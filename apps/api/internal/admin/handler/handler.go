package handler

import (
	"context"
	"strconv"

	adminModel "kun-galgame-patch-api/internal/admin/model"
	"kun-galgame-patch-api/internal/admin/dto"
	"kun-galgame-patch-api/internal/admin/service"
	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/galgame/enricher"
	"kun-galgame-patch-api/internal/middleware"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	settingService "kun-galgame-patch-api/internal/setting/service"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/userclient"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

type AdminHandler struct {
	service *service.AdminService
	wiki    *galgameClient.Client
	users   *userclient.Client
}

func New(svc *service.AdminService, wiki *galgameClient.Client, users *userclient.Client) *AdminHandler {
	return &AdminHandler{service: svc, wiki: wiki, users: users}
}

// attachUserBriefs is a tiny helper used by every admin list endpoint that
// previously relied on Preload("User") -- fetches publisher briefs from OAuth
// /users/batch in one call and stamps the User field on each row.
func (h *AdminHandler) attachCommentUsers(ctx context.Context, cs []patchModel.PatchComment) {
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

func (h *AdminHandler) attachResourceUsers(ctx context.Context, rs []patchModel.PatchResource) {
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

func (h *AdminHandler) attachLogUsers(ctx context.Context, ls []adminModel.AdminLog) {
	uids := make([]int, 0, len(ls))
	for _, l := range ls {
		uids = append(uids, l.UserID)
	}
	briefs := userclient.BriefMapByInt(ctx, h.users, uids)
	for i := range ls {
		if b := briefs[ls[i].UserID]; b != nil {
			ls[i].User = &patchModel.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar, AvatarImageHash: b.AvatarImageHash, Roles: b.Roles}
		}
	}
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

	comments, total, err := h.service.GetComments(req.Search, req.Status, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	h.attachCommentUsers(c.Context(), comments)
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
	if err := h.service.UpdateComment(id, req.Content, admin.ID); err != nil {
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
	if err := h.service.DeleteComment(id, admin.ID); err != nil {
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
	h.attachResourceUsers(c.Context(), resources)
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
	if err := h.service.UpdateResource(id, req.Note, admin.ID); err != nil {
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
	if err := h.service.DeleteResource(id, admin.ID); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OKMessage(c, "Resource deleted")
}

// ===== User purge (anti-spam, admin-only) =====

// GetUserPurgePreview GET /api/admin/user/:id/purge-preview?purge_owned_patches=
//
// Dry run: returns the count breakdown of everything a purge would remove. The
// purge_owned_patches query flag mirrors the execute-time force option so the
// owned-patch collateral counts reflect the same choice.
func (h *AdminHandler) GetUserPurgePreview(c *fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	preview, perr := h.service.PurgeUserPreview(id, c.QueryBool("purge_owned_patches", false))
	if perr != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.OK(c, preview)
}

// PurgeUser POST /api/admin/user/:id/purge   body: { purge_owned_patches: bool }
//
// Irreversibly removes every moyu-side trace of the user (comments, resources +
// their S3 files, likes/favorites/contributes, follows, chat, private messages)
// and the local user row, fixing denormalized counters on surviving content.
// Out of scope: OAuth identity, Wiki, kungal, image_service. Admin-only.
func (h *AdminHandler) PurgeUser(c *fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	var req dto.PurgeUserRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	admin := middleware.MustGetUser(c)
	res, perr := h.service.PurgeUser(id, req.PurgeOwnedPatches, admin.ID)
	if perr != nil {
		if appErr, ok := perr.(*errors.AppError); ok {
			return response.Error(c, appErr)
		}
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.OK(c, res)
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
	// Admin view sees everything by default — pass "all" so NSFW patches show
	// up in the admin list regardless of any content_limit query param the
	// admin's browser session happened to carry over from another page. The
	// admin console is the canonical "manage every row" surface; filtering
	// here would hide rows admins need to moderate.
	cards := enricher.EnrichPatches(c.Context(), h.wiki, h.users, patches, "all")
	return response.Paginated(c, cards, total)
}

// ===== Settings =====

// GetCommentVerify GET /api/admin/setting/comment-verify
func (h *AdminHandler) GetCommentVerify(c *fiber.Ctx) error {
	return response.OK(c, map[string]bool{"enabled": h.service.GetSetting(settingService.KeyCommentVerify)})
}

// SetCommentVerify PUT /api/admin/setting/comment-verify
func (h *AdminHandler) SetCommentVerify(c *fiber.Ctx) error {
	var req dto.AdminSettingBoolRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if err := h.service.SetSetting(settingService.KeyCommentVerify, req.Enabled, middleware.MustGetUser(c).ID); err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.OKMessage(c, "Setting updated")
}

// GetCreatorOnly GET /api/admin/setting/creator-only
func (h *AdminHandler) GetCreatorOnly(c *fiber.Ctx) error {
	return response.OK(c, map[string]bool{"enabled": h.service.GetSetting(settingService.KeyCreatorOnly)})
}

// SetCreatorOnly PUT /api/admin/setting/creator-only
//
// When on, only moderators / admins (role > 2) may publish a galgame — enforced
// in the patch publish handlers (CreatePatch / ClaimGalgame / SubmitGalgame).
func (h *AdminHandler) SetCreatorOnly(c *fiber.Ctx) error {
	var req dto.AdminSettingBoolRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if err := h.service.SetSetting(settingService.KeyCreatorOnly, req.Enabled, middleware.MustGetUser(c).ID); err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.OKMessage(c, "Setting updated")
}

// The "禁止注册" (disable-register) setting was removed — registration is
// unified on the OAuth server (the local register flow no longer exists), so
// the toggle is being reimplemented there rather than in this admin panel.

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

// GetResourceFileHistory GET /api/admin/resource/:id/history
//
// Returns the append-only file-replacement audit trail for one patch_resource
// (MOYU-PR5 / M3). Admin/moderator only (route gated by moderatorAuth). Rows
// are paginated newest-first; each row carries the snapshot of the resource's
// old file pointer (storage / s3_key / blake3 / size / content), the
// operator-supplied reason, and the actor id+role snapshot.
//
// Use case: when a user reports "this download is broken", an admin can pull
// up the resource's history and see exactly when the file was swapped, by
// whom, and why.
func (h *AdminHandler) GetResourceFileHistory(c *fiber.Ctx) error {
	resourceID, perr := strconv.Atoi(c.Params("id"))
	if perr != nil || resourceID <= 0 {
		return response.Error(c, errors.ErrBadRequest("invalid resource id"))
	}
	var req dto.AdminPaginationRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	items, total, err := h.service.GetResourceFileHistory(resourceID, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, items, total)
}

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
	h.attachLogUsers(c.Context(), logs)
	return response.Paginated(c, logs, total)
}

// ===== Orphan Patches (D12) =====

// GetOrphanPatches GET /api/admin/patch/orphans
//
// Lists "orphan" patches — those whose vndb_id is not a well-formed VNDB id
// (`vN`), so no matching Wiki galgame can exist (see repository.orphanCond;
// the old galgame_id=0 sentinel was dropped in D13). For each row, the admin can:
//   - Rebind the correct vndb_id via PUT /api/patch/:id (will re-verify with Wiki /galgame/check)
//   - Or DELETE /api/patch/:id to remove
//   - If vndb_id is real but not yet created in Wiki, create the galgame in Wiki first, then rebind
//
// Alongside `items`, the response returns pending_count (vndb_id = pending-N)
// and bad_vndb_count (vndb_id malformed — not vN and not pending-).
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
