package upload

import (
	"encoding/json"
	stderrors "errors"
	"io"

	"kun-galgame-patch-api/internal/constants"
	galgameclient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/imageclient"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

// Handler exposes 5 HTTP endpoints + the image_service upload proxy.
type Handler struct {
	svc *Service
	// img uploads moyu's OWN content images (preset=topic) under site=moyu.
	img *imageclient.Client // image_service SDK (W2 / PR3b)
	// wiki proxies galgame cover/screenshot uploads to the wiki's
	// POST /galgame/image so they're owned by site=galgame_wiki, not site=moyu.
	wiki *galgameclient.Client
}

// NewHandler constructs a Handler. img/wiki may be nil in tests.
func NewHandler(svc *Service, img *imageclient.Client, wiki *galgameclient.Client) *Handler {
	return &Handler{svc: svc, img: img, wiki: wiki}
}

// uploadTier resolves the caller's per-role upload allowance from the OAuth
// roles claim: admin/ren > moderator > creator > user. Each tier is distinct;
// a user holding several roles gets the highest (admin checked first).
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

// Init POST /api/upload/init — start an upload; the artifact service decides
// single-PUT vs multipart and returns the presigned URL(s).
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

// Complete POST /api/upload/complete — finalize (size verified by artifact) and
// deduct the per-user daily quota.
func (h *Handler) Complete(c fiber.Ctx) error {
	var req CompleteRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	user := middleware.MustGetUser(c)

	resp, err := h.svc.Complete(c.Context(), user.ID, uploadTier(c), req)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, resp)
}

// Resume POST /api/upload/resume — continue an interrupted upload. The artifact
// service lists the parts already in B2 and re-presigns only the missing ones, so
// a paused / dropped / page-refreshed upload finishes without re-sending bytes.
func (h *Handler) Resume(c fiber.Ctx) error {
	var req ResumeRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	_ = middleware.MustGetUser(c)

	resp, err := h.svc.Resume(c.Context(), req)
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
// Body: multipart/form-data with required `file` (image binary) and `preset`
// form field. moyu forwards the preset VERBATIM — image_service owns the
// allowlist (`image_allowed_presets` per OAuth client) and rejects anything not
// enabled, so there is intentionally no preset allowlist here. moyu's callers
// send `galgame_screenshot` (galgame screenshots) and `topic` (editor-inline /
// admin doc images). (Banners use the multipart PUT /galgame/:gid Wiki flow;
// avatars use OAuth's /auth/me/avatar — neither hits this endpoint.)
//
// 10MB body cap is inherited from the Fiber app config; image_service itself
// enforces per-preset size + per-client daily quota.
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

	// Galgame images (covers/screenshots) are WIKI-owned: proxy them to the
	// wiki's canonical POST /galgame/image (uploaded under site=galgame_wiki),
	// forwarding the user's Bearer — moyu no longer uploads galgame images
	// under its own site=moyu. Other presets (topic = moyu's own content
	// images) keep using moyu's local image_service client.
	if preset == "galgame_banner" || preset == "galgame_screenshot" {
		if h.wiki == nil {
			return response.Error(c, errors.ErrInternal("Wiki 客户端未配置"))
		}
		fileBytes, rerr := io.ReadAll(f)
		if rerr != nil {
			return response.Error(c, errors.ErrBadRequest("读取上传文件失败"))
		}
		data, werr := h.wiki.UploadGalgameImage(
			c.Context(), middleware.GetAccessToken(c), preset, fh.Filename, fileBytes, mime,
		)
		if werr != nil {
			// Forward the wiki/image_service business code + message; map the
			// common ones to sensible HTTP statuses (matches the local path).
			var we *galgameclient.WikiError
			if stderrors.As(werr, &we) {
				switch we.Code {
				case 80008:
					return response.Error(c, errors.New(80008, we.Message, fiber.StatusTooManyRequests))
				case 60002:
					return response.Error(c, errors.New(60002, we.Message, fiber.StatusUnprocessableEntity))
				default:
					return response.Error(c, errors.New(we.Code, we.Message, fiber.StatusBadRequest))
				}
			}
			return response.Error(c, errors.ErrInternal("Wiki 图片上传失败: "+werr.Error()))
		}
		// Wiki returns image_service's UploadResult, identical shape to moyu's
		// imageclient.UploadResult — re-marshal into it so the FE sees the same
		// response as the local path.
		var result imageclient.UploadResult
		if jerr := json.Unmarshal(data, &result); jerr != nil {
			return response.Error(c, errors.ErrInternal("解析 Wiki 上传响应失败"))
		}
		return response.OK(c, result)
	}

	// Non-galgame presets (topic, ...) → moyu's own image_service client.
	if h.img == nil {
		return response.Error(c, errors.ErrInternal("image_service 客户端未配置"))
	}

	// Per-USER daily cap: image_service enforces only a per-SITE quota, so moyu
	// applies its own per-user fair-use limit here (aligned with kungal). Only
	// moyu-owned content images (this branch) count — the wiki-proxied galgame
	// images above do not.
	if qErr := h.svc.CheckDailyImageQuota(user.ID); qErr != nil {
		if stderrors.Is(qErr, errDailyImageLimit) {
			return response.Error(c, errors.New(80008, qErr.Error(), fiber.StatusTooManyRequests))
		}
		// A DB-read failure must not masquerade as a 429 rate-limit.
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

// Abort POST /api/upload/abort — voluntarily cancel an in-progress upload.
func (h *Handler) Abort(c fiber.Ctx) error {
	var req AbortRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	_ = middleware.MustGetUser(c)

	if err := h.svc.Abort(c.Context(), req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OKMessage(c, "已放弃上传")
}
