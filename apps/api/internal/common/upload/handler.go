package upload

import (
	stderrors "errors"
	"io"

	"kun-galgame-patch-api/internal/constants"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/imageclient"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

type Handler struct {
	svc *Service
	img *imageclient.Client
}

func NewHandler(svc *Service, img *imageclient.Client) *Handler {
	return &Handler{svc: svc, img: img}
}

func uploadTier(c fiber.Ctx) constants.UploadTier {
	switch {
	case middleware.IsAdmin(c):
		return constants.AdminUploadTier
	case middleware.HasRole(c, "moderator"):
		return constants.ModeratorUploadTier
	case middleware.HasRole(c, "creator"):
		return constants.CreatorUploadTier
	default:
		return constants.UserUploadTier
	}
}

func uploadError(c fiber.Ctx, err error) error {
	switch {
	case stderrors.Is(err, errNotUploadOwner):
		return response.Error(c, errors.New(40300, err.Error(), fiber.StatusForbidden))
	case stderrors.Is(err, errArtifactInUse):
		return response.Error(c, errors.ErrConflict(err.Error()))
	}
	return response.Error(c, errors.ErrBadRequest(err.Error()))
}

func (h *Handler) Init(c fiber.Ctx) error {
	var req InitRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	user := middleware.MustGetUser(c)

	resp, err := h.svc.Init(c.Context(), user.ID, uploadTier(c), req)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, resp)
}

func (h *Handler) Complete(c fiber.Ctx) error {
	var req CompleteRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	user := middleware.MustGetUser(c)

	resp, err := h.svc.Complete(c.Context(), user.ID, uploadTier(c), req)
	if err != nil {
		return uploadError(c, err)
	}
	return response.OK(c, resp)
}

func (h *Handler) Resume(c fiber.Ctx) error {
	var req ResumeRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	user := middleware.MustGetUser(c)

	resp, err := h.svc.Resume(c.Context(), user.ID, req)
	if err != nil {
		return uploadError(c, err)
	}
	return response.OK(c, resp)
}

func (h *Handler) UploadImageService(c fiber.Ctx) error {
	user := middleware.MustGetUser(c)

	preset := c.FormValue("preset")
	if preset == "" {
		return response.Error(c, errors.ErrBadRequest("缺少 preset 字段"))
	}

	fh, ferr := c.FormFile("file")
	if ferr != nil {
		return response.Error(c, errors.ErrBadRequest("缺少 file 字段"))
	}
	if fh.Size > 10*1024*1024 {
		return response.Error(c, errors.ErrBadRequest("文件超过 10MB 上限"))
	}
	f, oerr := fh.Open()
	if oerr != nil {
		return response.Error(c, errors.ErrBadRequest("无法读取上传文件"))
	}
	defer f.Close()
	mime := fh.Header.Get("Content-Type")

	if h.img == nil {
		return response.Error(c, errors.ErrInternal("image_service 客户端未配置"))
	}

	if qErr := h.svc.CheckDailyImageQuota(user.ID); qErr != nil {
		if stderrors.Is(qErr, errDailyImageLimit) {
			return response.Error(c, errors.New(80008, qErr.Error(), fiber.StatusTooManyRequests))
		}
		return response.Error(c, errors.ErrInternal("查询上传配额失败"))
	}

	result, err := h.img.Upload(c.Context(), io.Reader(f), fh.Filename, mime, preset)
	if err != nil {
		switch {
		case stderrors.Is(err, imageclient.ErrQuotaExceeded):
			return response.Error(c, errors.New(80008, err.Error(), fiber.StatusTooManyRequests))
		case stderrors.Is(err, imageclient.ErrModerationRejected):
			return response.Error(c, errors.New(60002, err.Error(), fiber.StatusUnprocessableEntity))
		case stderrors.Is(err, imageclient.ErrUnauthorized):
			return response.Error(c, errors.ErrInternal("image_service 鉴权失败（检查 client_id/secret 与 image_enabled）"))
		default:
			return response.Error(c, errors.ErrInternal("image_service 上传失败: "+err.Error()))
		}
	}

	h.svc.IncrementDailyImageCount(user.ID)
	return response.OK(c, result)
}

func (h *Handler) Abort(c fiber.Ctx) error {
	var req AbortRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	user := middleware.MustGetUser(c)

	if err := h.svc.Abort(c.Context(), user.ID, req); err != nil {
		return uploadError(c, err)
	}
	return response.OKMessage(c, "已放弃上传")
}
