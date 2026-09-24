package upload

import (
	stderrors "errors"
	"slices"

	"kun-galgame-patch-api/internal/constants"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/artifactclient"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/imageclient"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/upstream"
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

var uploadWording = map[upstream.Kind]string{
	upstream.NotFound: "上传会话不存在或已过期，请重新上传",
	upstream.Conflict: "该上传已结束，无法续传，请重新上传",
	upstream.Rejected: "上传请求无效，请重新上传",
}

func uploadError(c fiber.Ctx, err error) error {
	var appErr *errors.AppError
	switch {
	case stderrors.As(err, &appErr):
		return response.Error(c, appErr)
	case stderrors.Is(err, errNotUploadOwner):
		return response.Error(c, errors.New(40300, err.Error(), fiber.StatusForbidden))
	case stderrors.Is(err, errArtifactInUse):
		return response.Error(c, errors.ErrConflict(err.Error()))
	case stderrors.Is(err, artifactclient.ErrTooBig):
		return response.Upstream(c, err, "文件大小超过上限")
	case stderrors.Is(err, artifactclient.ErrMIMEDenied):
		return response.Upstream(c, err, "不支持的文件类型")
	case stderrors.Is(err, artifactclient.ErrSizeMismatch):
		return response.Upstream(c, err, "上传文件大小与声明不符，请重新上传")
	case stderrors.Is(err, artifactclient.ErrQuotaExceeded):
		return response.Error(c, errors.ErrTooManyRequests("文件服务今日上传配额已满，请明天再试"))
	case stderrors.Is(err, artifactclient.ErrUploadDisabled):
		return response.Error(c, errors.New(50300, "上传功能暂未开放", fiber.StatusServiceUnavailable))
	}
	return response.Upstream(c, err, uploadWording[upstream.KindOf(err)])
}

func (h *Handler) Init(c fiber.Ctx) error {
	var req InitRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	user := middleware.MustGetUser(c)

	resp, err := h.svc.Init(c.Context(), user.ID, uploadTier(c), req)
	if err != nil {
		return uploadError(c, err)
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

// imagePresets is what moyu's OAuth client is allowlisted for on this lane.
// Avatars go through OAuth's /auth/me/avatar and never reach it.
var imagePresets = []string{"topic"}

const maxImageBytes = 10 * 1024 * 1024

func imageError(c fiber.Ctx, err error) error {
	switch {
	case stderrors.Is(err, imageclient.ErrFileTooLarge):
		return response.Upstream(c, err, "图片大小超过限制")
	case stderrors.Is(err, imageclient.ErrMIMEDenied):
		return response.Upstream(c, err, "不支持的图片格式")
	case stderrors.Is(err, imageclient.ErrDecodeFailed):
		return response.Upstream(c, err, "图片无法解析，请换一张图片")
	case stderrors.Is(err, imageclient.ErrModerationRejected):
		return response.Error(c, errors.New(60002, "图片未通过内容审核", fiber.StatusUnprocessableEntity))
	case stderrors.Is(err, imageclient.ErrQuotaExceeded):
		return response.Error(c, errors.New(80008, "图片服务今日上传配额已满，请明天再试", fiber.StatusTooManyRequests))
	case stderrors.Is(err, imageclient.ErrUploadDisabled):
		return response.Error(c, errors.New(50300, "图片上传暂未开放", fiber.StatusServiceUnavailable))
	}
	return response.Upstream(c, err, "")
}

func (h *Handler) UploadImageService(c fiber.Ctx) error {
	user := middleware.MustGetUser(c)

	preset := c.FormValue("preset")
	if !slices.Contains(imagePresets, preset) {
		return response.Error(c, errors.ErrBadRequest("不支持的图片用途 (preset)"))
	}

	fh, ferr := c.FormFile("file")
	if ferr != nil {
		return response.Error(c, errors.ErrBadRequest("缺少 file 字段"))
	}
	if fh.Size > maxImageBytes {
		return response.Error(c, errors.ErrBadRequest("文件超过 10MB 上限"))
	}
	f, oerr := fh.Open()
	if oerr != nil {
		return response.Error(c, errors.ErrBadRequest("无法读取上传文件"))
	}
	defer f.Close()

	if !h.img.Configured() {
		return response.Upstream(c, imageclient.ErrNotConfigured, "")
	}

	if err := h.svc.ReserveDailyImage(user.ID); err != nil {
		if stderrors.Is(err, errDailyImageLimit) {
			return response.Error(c, errors.New(80008, err.Error(), fiber.StatusTooManyRequests))
		}
		return response.Upstream(c, err, "")
	}

	result, err := h.img.Upload(c.Context(), f, fh.Filename, fh.Header.Get("Content-Type"), preset)
	if err != nil {
		h.svc.ReleaseDailyImage(user.ID)
		return imageError(c, err)
	}
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
