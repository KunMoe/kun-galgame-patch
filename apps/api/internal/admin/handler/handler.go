package handler

import (
	"context"
	"strconv"
	"strings"

	"kun-galgame-patch-api/internal/admin/dto"
	adminModel "kun-galgame-patch-api/internal/admin/model"
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

	"github.com/gofiber/fiber/v3"
)

type AdminHandler struct {
	service *service.AdminService
	galgame *galgameClient.Client
	users   *userclient.Client
}

func New(svc *service.AdminService, galgame *galgameClient.Client, users *userclient.Client) *AdminHandler {
	return &AdminHandler{service: svc, galgame: galgame, users: users}
}

func (h *AdminHandler) attachCommentUsers(ctx context.Context, cs []patchModel.PatchComment) {
	uids := make([]int, 0, len(cs))
	for _, c := range cs {
		uids = append(uids, c.UserID)
	}
	briefs := userclient.BriefMapByInt(ctx, h.users, uids)
	for i := range cs {
		if b := briefs[cs[i].UserID]; b != nil {
			cs[i].User = &patchModel.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar, AvatarImageHash: b.AvatarImageHash, Roles: b.Roles, SiteRoles: b.SiteRoles}
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
			rs[i].User = &patchModel.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar, AvatarImageHash: b.AvatarImageHash, Roles: b.Roles, SiteRoles: b.SiteRoles}
		}
	}
}

func (h *AdminHandler) attachPatchSummaries(ctx context.Context, comments []patchModel.PatchComment, resources []patchModel.PatchResource) {
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
	summaries := enricher.BuildPatchSummaryMap(ctx, h.galgame, h.service, ids)
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

func (h *AdminHandler) attachLogUsers(ctx context.Context, ls []adminModel.AdminLog) {
	uids := make([]int, 0, len(ls))
	for _, l := range ls {
		uids = append(uids, l.UserID)
	}
	briefs := userclient.BriefMapByInt(ctx, h.users, uids)
	for i := range ls {
		if b := briefs[ls[i].UserID]; b != nil {
			ls[i].User = &patchModel.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar, AvatarImageHash: b.AvatarImageHash, Roles: b.Roles, SiteRoles: b.SiteRoles}
		}
	}
}

func getIDParam(c fiber.Ctx, name string) (int, error) {
	id, err := strconv.Atoi(c.Params(name))
	if err != nil || id < 1 {
		return 0, errors.ErrBadRequest("invalid ID")
	}
	return id, nil
}

func (h *AdminHandler) GetComments(c fiber.Ctx) error {
	var req dto.AdminPaginationRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	comments, total, err := h.service.GetComments(req.Search, req.Status, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	h.attachCommentUsers(c.Context(), comments)
	h.attachPatchSummaries(c.Context(), comments, nil)
	return response.Paginated(c, comments, total)
}

func (h *AdminHandler) UpdateComment(c fiber.Ctx) error {
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

func (h *AdminHandler) DeleteComment(c fiber.Ctx) error {
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

func (h *AdminHandler) GetResources(c fiber.Ctx) error {
	var req dto.AdminPaginationRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	resources, total, err := h.service.GetResources(req.Search, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	h.attachResourceUsers(c.Context(), resources)
	h.attachPatchSummaries(c.Context(), nil, resources)
	return response.Paginated(c, resources, total)
}

func (h *AdminHandler) UpdateResource(c fiber.Ctx) error {
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

func (h *AdminHandler) DeleteResource(c fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.Bind().Body(&body)
	reason := strings.TrimSpace(body.Reason)
	if rs := []rune(reason); len(rs) > 500 {
		reason = string(rs[:500])
	}

	admin := middleware.MustGetUser(c)
	if err := h.service.DeleteResource(id, admin.ID, reason); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OKMessage(c, "Resource deleted")
}

func (h *AdminHandler) GetUserPurgePreview(c fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	preview, perr := h.service.PurgeUserPreview(c.Context(), id,
		fiber.Query(c, "purge_owned_patches", false), middleware.GetAccessToken(c))
	if perr != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.OK(c, preview)
}

func (h *AdminHandler) PurgeUser(c fiber.Ctx) error {
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

func (h *AdminHandler) GetGalgame(c fiber.Ctx) error {
	var req dto.AdminPaginationRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	patches, total, err := h.service.GetAllPatches(req.Search, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	cards := enricher.EnrichPatches(c.Context(), h.galgame, h.users, patches, "all")
	return response.Paginated(c, cards, total)
}

func (h *AdminHandler) GetCommentVerify(c fiber.Ctx) error {
	return response.OK(c, map[string]bool{"enabled": h.service.GetSetting(settingService.KeyCommentVerify)})
}

func (h *AdminHandler) SetCommentVerify(c fiber.Ctx) error {
	var req dto.AdminSettingBoolRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if err := h.service.SetSetting(settingService.KeyCommentVerify, req.Enabled, middleware.MustGetUser(c).ID); err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.OKMessage(c, "Setting updated")
}

func (h *AdminHandler) GetCreatorOnly(c fiber.Ctx) error {
	return response.OK(c, map[string]bool{"enabled": h.service.GetSetting(settingService.KeyCreatorOnly)})
}

func (h *AdminHandler) SetCreatorOnly(c fiber.Ctx) error {
	var req dto.AdminSettingBoolRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if err := h.service.SetSetting(settingService.KeyCreatorOnly, req.Enabled, middleware.MustGetUser(c).ID); err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.OKMessage(c, "Setting updated")
}

func (h *AdminHandler) GetStats(c fiber.Ctx) error {
	var req dto.AdminStatsRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, h.service.GetStats(req.Days))
}

func (h *AdminHandler) GetStatsSum(c fiber.Ctx) error {
	return response.OK(c, h.service.GetStatsSum())
}

func (h *AdminHandler) GetResourceFileHistory(c fiber.Ctx) error {
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

func (h *AdminHandler) GetLogs(c fiber.Ctx) error {
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

func (h *AdminHandler) GetOrphanPatches(c fiber.Ctx) error {
	var req dto.AdminPaginationRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	candidateIDs, err := h.service.GetOrphanCandidateIDs()
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	existing := make([]int, 0, len(candidateIDs))
	for start := 0; start < len(candidateIDs); start += galgameClient.BatchMaxIDs {
		end := min(start+galgameClient.BatchMaxIDs, len(candidateIDs))
		briefs, bErr := h.galgame.GalgameBatch(c.Context(), candidateIDs[start:end], "")
		if bErr != nil {
			return response.Error(c, errors.ErrInternal("资料库校验失败"))
		}
		for _, b := range briefs {
			existing = append(existing, b.ID)
		}
	}

	items, total, err := h.service.GetOrphanPatches(req.Page, req.Limit, existing)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	pending, badVndb, cErr := h.service.CountOrphanPatches(existing)
	if cErr != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.OK(c, map[string]any{
		"items":          items,
		"total":          total,
		"pending_count":  pending,
		"bad_vndb_count": badVndb,
	})
}
