package handler

import (
	"io"
	"strconv"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/galgame/enricher"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/internal/user/dto"
	"kun-galgame-patch-api/internal/user/service"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
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
}

func New(svc *service.UserService, wiki *galgameClient.Client) *UserHandler {
	return &UserHandler{service: svc, wiki: wiki}
}

func getUID(c *fiber.Ctx) (int, error) {
	uid, err := strconv.Atoi(c.Params("uid"))
	if err != nil || uid < 1 {
		return 0, errors.ErrBadRequest("invalid user ID")
	}
	return uid, nil
}

// GetUserInfo GET /api/user/:uid
func (h *UserHandler) GetUserInfo(c *fiber.Ctx) error {
	uid, err := getUID(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	currentUID := middleware.GetUID(c)
	info, err := h.service.GetUserInfo(uid, currentUID)
	if err != nil {
		return response.Error(c, errors.ErrNotFound(err.Error()))
	}

	return response.OK(c, info)
}

// GetUserFloating GET /api/user/:uid/floating
func (h *UserHandler) GetUserFloating(c *fiber.Ctx) error {
	uid, err := getUID(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	info, err := h.service.GetUserFloating(uid)
	if err != nil {
		return response.Error(c, errors.ErrNotFound(err.Error()))
	}

	return response.OK(c, info)
}

// GetUserPatches GET /api/user/:uid/patch
func (h *UserHandler) GetUserPatches(c *fiber.Ctx) error {
	uid, err := getUID(c)
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

	patches, total, err := h.service.GetUserPatches(uid, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, enricher.EnrichPatches(c.Context(), h.wiki, patches), total)
}

// GetUserResources GET /api/user/:uid/resource
func (h *UserHandler) GetUserResources(c *fiber.Ctx) error {
	uid, err := getUID(c)
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

	data, total, err := h.service.GetUserResources(uid, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, data, total)
}

// GetUserFavorites GET /api/user/:uid/favorite
func (h *UserHandler) GetUserFavorites(c *fiber.Ctx) error {
	uid, err := getUID(c)
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

	patches, total, err := h.service.GetUserFavorites(uid, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, enricher.EnrichPatches(c.Context(), h.wiki, patches), total)
}

// GetUserComments GET /api/user/:uid/comment
func (h *UserHandler) GetUserComments(c *fiber.Ctx) error {
	uid, err := getUID(c)
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

	data, total, err := h.service.GetUserComments(uid, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, data, total)
}

// GetUserContributions GET /api/user/:uid/contribute
func (h *UserHandler) GetUserContributions(c *fiber.Ctx) error {
	uid, err := getUID(c)
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

	patches, total, err := h.service.GetUserContributions(uid, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, enricher.EnrichPatches(c.Context(), h.wiki, patches), total)
}

// Profile mutations (username / bio / password / email / avatar) live on
// OAuth: the frontend should call OAuth's PATCH /auth/me directly or be
// redirected to oauth.kungal.com/profile.

// Follow PUT /api/user/:uid/follow
func (h *UserHandler) Follow(c *fiber.Ctx) error {
	uid, err := getUID(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	user := middleware.MustGetUser(c)
	if err := h.service.Follow(user.UID, uid); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	return response.OKMessage(c, "Followed")
}

// Unfollow DELETE /api/user/:uid/follow
func (h *UserHandler) Unfollow(c *fiber.Ctx) error {
	uid, err := getUID(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	user := middleware.MustGetUser(c)
	if err := h.service.Unfollow(user.UID, uid); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	return response.OKMessage(c, "Unfollowed")
}

// GetFollowers GET /api/user/:uid/follower
func (h *UserHandler) GetFollowers(c *fiber.Ctx) error {
	uid, err := getUID(c)
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

	users, total, err := h.service.GetFollowers(uid, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, users, total)
}

// GetFollowing GET /api/user/:uid/following
func (h *UserHandler) GetFollowing(c *fiber.Ctx) error {
	uid, err := getUID(c)
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

	users, total, err := h.service.GetFollowing(uid, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, users, total)
}

// CheckIn POST /api/user/check-in
func (h *UserHandler) CheckIn(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	points, err := h.service.CheckIn(user.UID)
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

	users, err := h.service.SearchUsers(req.Query)
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
	url, err := h.service.UploadUserImage(c.Context(), user.UID, raw)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, map[string]string{"url": url})
}
