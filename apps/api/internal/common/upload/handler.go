package upload

import (
	stderrors "errors"
	"io"

	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/imageclient"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

// Handler exposes 5 HTTP endpoints + the image_service upload proxy.
type Handler struct {
	svc *Service
	img *imageclient.Client // image_service SDK (W2 / PR3b)
}

// NewHandler constructs a Handler. img may be nil in tests; UploadImageService
// then returns 503.
func NewHandler(svc *Service, img *imageclient.Client) *Handler {
	return &Handler{svc: svc, img: img}
}

// InitSmall POST /api/upload/small/init
func (h *Handler) InitSmall(c *fiber.Ctx) error {
	var req SmallInitRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	user := middleware.MustGetUser(c)
	privileged := middleware.HasAnyRole(c, "admin", "moderator")

	resp, err := h.svc.InitSmall(c.Context(), user.ID, privileged, req)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, resp)
}

// CompleteSmall POST /api/upload/small/complete
func (h *Handler) CompleteSmall(c *fiber.Ctx) error {
	var req SmallCompleteRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	user := middleware.MustGetUser(c)
	privileged := middleware.HasAnyRole(c, "admin", "moderator")

	resp, err := h.svc.CompleteSmall(c.Context(), user.ID, privileged, req)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, resp)
}

// InitMultipart POST /api/upload/multipart/init
func (h *Handler) InitMultipart(c *fiber.Ctx) error {
	var req MultipartInitRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	user := middleware.MustGetUser(c)
	privileged := middleware.HasAnyRole(c, "admin", "moderator")

	resp, err := h.svc.InitMultipart(c.Context(), user.ID, privileged, req)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, resp)
}

// CompleteMultipart POST /api/upload/multipart/complete
func (h *Handler) CompleteMultipart(c *fiber.Ctx) error {
	var req MultipartCompleteRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	user := middleware.MustGetUser(c)
	privileged := middleware.HasAnyRole(c, "admin", "moderator")

	resp, err := h.svc.CompleteMultipart(c.Context(), user.ID, privileged, req)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, resp)
}

// UploadImageService POST /api/upload/image-service
//
// Proxies a multipart image upload to the centralized image_service
// (kun-galgame-infra :9278) and returns the content hash + variant URLs. Used
// by the galgame screenshot editor (Wiki PR5: screenshots must reference
// image_service by hash, and Wiki itself accepts no multipart for them).
// Covers can also flow through this if the client wants to add a non-pinned
// cover; pinned-banner uploads still go through PUT /galgame/:gid multipart
// where Wiki auto-promotes the hash to covers[sort_order=0].
//
// Body: multipart/form-data with required `file` (image binary) and
// `preset` form field (one of the image_service-enabled presets — typically
// `topic` for screenshots; `galgame_banner` if the OAuth client has it
// allowed; `avatar` for avatars).
//
// 10MB body cap is inherited from the Fiber app config; image_service itself
// enforces per-preset size + per-client daily quota.
func (h *Handler) UploadImageService(c *fiber.Ctx) error {
	_ = middleware.MustGetUser(c)

	if h.img == nil {
		return response.Error(c, errors.ErrInternal("image_service 客户端未配置"))
	}

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
	return response.OK(c, result)
}

// AbortMultipart POST /api/upload/multipart/abort
func (h *Handler) AbortMultipart(c *fiber.Ctx) error {
	var req MultipartAbortRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	_ = middleware.MustGetUser(c)

	if err := h.svc.AbortMultipart(c.Context(), req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OKMessage(c, "已放弃上传")
}
