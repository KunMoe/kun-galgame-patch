package handler

import (
	"strconv"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/galgame/enricher"
	"kun-galgame-patch-api/internal/middleware"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/internal/user/dto"
	"kun-galgame-patch-api/internal/user/service"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/userclient"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

type UserHandler struct {
	service *service.UserService
	galgame *galgameClient.Client
	users   *userclient.Client
}

func New(svc *service.UserService, galgame *galgameClient.Client, users *userclient.Client) *UserHandler {
	return &UserHandler{service: svc, galgame: galgame, users: users}
}

func getUID(c fiber.Ctx) (int, error) {
	userID, err := strconv.Atoi(c.Params("id"))
	if err != nil || userID < 1 {
		return 0, errors.ErrBadRequest("invalid user ID")
	}
	return userID, nil
}

func (h *UserHandler) GetUserInfo(c fiber.Ctx) error {
	userID, err := getUID(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	currentUID := middleware.GetUserID(c)
	info, err := h.service.GetUserInfo(c.Context(), userID, currentUID,
		middleware.GetAccessToken(c), utils.ContentLimitForListBrowse(c))
	if err != nil {
		return response.Error(c, errors.ErrNotFound(err.Error()))
	}

	return response.OK(c, info)
}

func (h *UserHandler) GetUserFloating(c fiber.Ctx) error {
	userID, err := getUID(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	info, err := h.service.GetUserFloating(c.Context(), userID)
	if err != nil {
		return response.Error(c, errors.ErrNotFound(err.Error()))
	}

	return response.OK(c, info)
}

func (h *UserHandler) GetUserPatches(c fiber.Ctx) error {
	userID, err := getUID(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.GetUserProfileRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 10
	}

	cl := utils.ContentLimitForListBrowse(c)
	patches, total, err := h.service.GetUserPatches(userID, req.Page, req.Limit, utils.IncludeEmptyGalgames(c), cl)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, enricher.EnrichPatchCards(c.Context(), h.galgame, h.users, patches, cl), total)
}

func (h *UserHandler) GetUserResources(c fiber.Ctx) error {
	userID, err := getUID(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.GetUserProfileRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 10
	}

	data, total, err := h.service.GetUserResources(c.Context(), userID, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	data = enricher.FilterByGalgameContentLimit(c.Context(), h.galgame, data, func(r patchModel.PatchResource) int { return r.GalgameID }, utils.ContentLimitForListBrowse(c))
	patchModel.StripResourceSecrets(data)
	return response.Paginated(c, data, total)
}

func (h *UserHandler) GetUserFavorites(c fiber.Ctx) error {
	userID, err := getUID(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.GetUserProfileRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 10
	}

	cl := utils.ContentLimitForListBrowse(c)
	viewer := middleware.GetUser(c)
	isOwner := viewer != nil && viewer.ID == userID
	patches, total, err := h.service.GetUserFavorites(c.Context(), userID,
		middleware.GetAccessToken(c), isOwner, req.Page, req.Limit, true, cl)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, enricher.EnrichPatchCards(c.Context(), h.galgame, h.users, patches, cl), total)
}

func (h *UserHandler) GetUserComments(c fiber.Ctx) error {
	userID, err := getUID(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.GetUserProfileRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 10
	}

	data, total, err := h.service.GetUserComments(c.Context(), userID, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	data = enricher.FilterByGalgameContentLimit(c.Context(), h.galgame, data, func(m patchModel.PatchComment) int { return m.GalgameID }, utils.ContentLimitForListBrowse(c))
	return response.Paginated(c, data, total)
}

func (h *UserHandler) GetUserContributions(c fiber.Ctx) error {
	userID, err := getUID(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.GetUserProfileRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 10
	}

	cl := utils.ContentLimitForListBrowse(c)
	patches, total, err := h.service.GetUserContributions(userID, req.Page, req.Limit, utils.IncludeEmptyGalgames(c), cl)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, enricher.EnrichPatchCards(c.Context(), h.galgame, h.users, patches, cl), total)
}

func (h *UserHandler) Follow(c fiber.Ctx) error {
	userID, err := getUID(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	user := middleware.MustGetUser(c)
	if err := h.service.Follow(user.ID, userID); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	return response.OKMessage(c, "Followed")
}

func (h *UserHandler) Unfollow(c fiber.Ctx) error {
	userID, err := getUID(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	user := middleware.MustGetUser(c)
	if err := h.service.Unfollow(user.ID, userID); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	return response.OKMessage(c, "Unfollowed")
}

func (h *UserHandler) GetFollowers(c fiber.Ctx) error {
	userID, err := getUID(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.GetUserProfileRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 20
	}

	users, total, err := h.service.GetFollowers(c.Context(), userID, middleware.GetUserID(c), req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, users, total)
}

func (h *UserHandler) GetFollowing(c fiber.Ctx) error {
	userID, err := getUID(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.GetUserProfileRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 20
	}

	users, total, err := h.service.GetFollowing(c.Context(), userID, middleware.GetUserID(c), req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, users, total)
}

func (h *UserHandler) CheckIn(c fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	points, err := h.service.CheckIn(user.ID)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	return response.OK(c, map[string]int{"moemoepoint": points})
}

func (h *UserHandler) GetMoemoepointLog(c fiber.Ctx) error {
	user := middleware.MustGetUser(c)

	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	beforeID, _ := strconv.ParseInt(c.Query("before_id"), 10, 64)

	items, hasMore, err := h.service.GetMoemoepointLog(c.Context(), user.ID, limit, beforeID, c.Query("reason"))
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.OK(c, fiber.Map{"items": items, "has_more": hasMore})
}

func (h *UserHandler) SearchUsers(c fiber.Ctx) error {
	var req dto.SearchUserRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	users, err := h.service.SearchUsers(c.Context(), req.Query, 50)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	return response.OK(c, users)
}
