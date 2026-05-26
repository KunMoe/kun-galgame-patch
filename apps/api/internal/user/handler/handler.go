package handler

import (
	"io"
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

	"github.com/gofiber/fiber/v2"
)

// Read the bytes of a single image file from a form, with a 10 MB size cap.
func readImageFormFile(c *fiber.Ctx, field string) ([]byte, error) {
	f, err := c.FormFile(field)
	if err != nil || f == nil {
		return nil, errors.ErrBadRequest("缺少图片文件")
	}
	if f.Size > 10*1024*1024 {
		return nil, errors.ErrBadRequest("图片超过 10MB")
	}
	fh, err := f.Open()
	if err != nil {
		return nil, errors.ErrBadRequest("读取图片失败")
	}
	defer fh.Close()
	return io.ReadAll(fh)
}

type UserHandler struct {
	service *service.UserService
	wiki    *galgameClient.Client
	users   *userclient.Client
}

func New(svc *service.UserService, wiki *galgameClient.Client, users *userclient.Client) *UserHandler {
	return &UserHandler{service: svc, wiki: wiki, users: users}
}

func getUID(c *fiber.Ctx) (int, error) {
	userID, err := strconv.Atoi(c.Params("id"))
	if err != nil || userID < 1 {
		return 0, errors.ErrBadRequest("invalid user ID")
	}
	return userID, nil
}

// GetUserInfo GET /api/user/:id
func (h *UserHandler) GetUserInfo(c *fiber.Ctx) error {
	userID, err := getUID(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	currentUID := middleware.GetUserID(c)
	info, err := h.service.GetUserInfo(c.Context(), userID, currentUID)
	if err != nil {
		return response.Error(c, errors.ErrNotFound(err.Error()))
	}

	return response.OK(c, info)
}

// GetUserFloating GET /api/user/:id/floating
func (h *UserHandler) GetUserFloating(c *fiber.Ctx) error {
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

// GetUserPatches GET /api/user/:id/patch
func (h *UserHandler) GetUserPatches(c *fiber.Ctx) error {
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

	patches, total, err := h.service.GetUserPatches(userID, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, enricher.EnrichPatches(c.Context(), h.wiki, h.users, patches, utils.ContentLimitForListBrowse(c)), total)
}

// GetUserResources GET /api/user/:id/resource
func (h *UserHandler) GetUserResources(c *fiber.Ctx) error {
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
	// NSFW filter: each resource carries the owning patch summary via the
	// service's attachPatchSummaries — drop rows whose owning patch wiki
	// excludes under content_limit before they reach the response.
	data = enricher.FilterByGalgameContentLimit(c.Context(), h.wiki, data, func(r patchModel.PatchResource) int { return r.GalgameID }, utils.ContentLimitForListBrowse(c))
	return response.Paginated(c, data, total)
}

// GetUserFavorites GET /api/user/:id/favorite
func (h *UserHandler) GetUserFavorites(c *fiber.Ctx) error {
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

	patches, total, err := h.service.GetUserFavorites(userID, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, enricher.EnrichPatches(c.Context(), h.wiki, h.users, patches, utils.ContentLimitForListBrowse(c)), total)
}

// GetUserComments GET /api/user/:id/comment
func (h *UserHandler) GetUserComments(c *fiber.Ctx) error {
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
	data = enricher.FilterByGalgameContentLimit(c.Context(), h.wiki, data, func(m patchModel.PatchComment) int { return m.GalgameID }, utils.ContentLimitForListBrowse(c))
	return response.Paginated(c, data, total)
}

// GetUserContributions GET /api/user/:id/contribute
func (h *UserHandler) GetUserContributions(c *fiber.Ctx) error {
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

	patches, total, err := h.service.GetUserContributions(userID, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, enricher.EnrichPatches(c.Context(), h.wiki, h.users, patches, utils.ContentLimitForListBrowse(c)), total)
}

// Profile mutations (username / bio / password / email / avatar) live on
// OAuth: the frontend should call OAuth's PATCH /auth/me directly or be
// redirected to oauth.kungal.com/profile.

// Follow PUT /api/user/:id/follow
func (h *UserHandler) Follow(c *fiber.Ctx) error {
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

// Unfollow DELETE /api/user/:id/follow
func (h *UserHandler) Unfollow(c *fiber.Ctx) error {
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

// GetFollowers GET /api/user/:id/follower
func (h *UserHandler) GetFollowers(c *fiber.Ctx) error {
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

	// viewerID (0 = anonymous) lets the service stamp per-row is_followed
	// so the FE follow-list modal can render the correct button state.
	users, total, err := h.service.GetFollowers(c.Context(), userID, middleware.GetUserID(c), req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, users, total)
}

// GetFollowing GET /api/user/:id/following
func (h *UserHandler) GetFollowing(c *fiber.Ctx) error {
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

// CheckIn POST /api/user/check-in
func (h *UserHandler) CheckIn(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	points, err := h.service.CheckIn(user.ID)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	return response.OK(c, map[string]int{"moemoepoint": points})
}

// SearchUsers GET /api/user/search
func (h *UserHandler) SearchUsers(c *fiber.Ctx) error {
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

// UpdateAvatar was removed -- avatars are owned by OAuth/image_service.

// UploadImage POST /api/user/image
// Images used on the user's personal page. Rate-limited by daily_image_count.
func (h *UserHandler) UploadImage(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	raw, err := readImageFormFile(c, "image")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	url, err := h.service.UploadUserImage(c.Context(), user.ID, raw)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, map[string]string{"url": url})
}
