package handler

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"log/slog"
	"strconv"
	"time"

	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/internal/trust/dto"
	"kun-galgame-patch-api/internal/trust/service"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/trustclient"
	"kun-galgame-patch-api/pkg/upstream"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

type Enforcer interface {
	Apply(ctx context.Context, cb dto.TrustCallback) error
}

type TrustHandler struct {
	trustService   *service.TrustService
	enforce        Enforcer
	callbackSecret string
}

func NewTrustHandler(
	trustService *service.TrustService,
	enforcer Enforcer,
	callbackSecret string,
) *TrustHandler {
	return &TrustHandler{
		trustService:   trustService,
		enforce:        enforcer,
		callbackSecret: callbackSecret,
	}
}

var errTrustDisabled = errors.New(50300, "举报服务暂未启用", fiber.StatusServiceUnavailable)

func (h *TrustHandler) GetReasons(c fiber.Ctx) error {
	reasons, err := h.trustService.Reasons(c.Context())
	if err != nil {
		return response.Upstream(c, err, "")
	}
	return response.OK(c, reasons)
}

func (h *TrustHandler) SubmitReport(c fiber.Ctx) error {
	user := middleware.MustGetUser(c)

	var req dto.SubmitReportRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	res, err := h.trustService.SubmitReport(c.Context(), user.ID, &req)
	switch {
	case err == nil:
		return response.OK(c, res)
	case stderrors.Is(err, trustclient.ErrNotConfigured):
		return response.Error(c, errTrustDisabled)
	case upstream.KindOf(err) == upstream.RateLimited:
		return response.Error(c, errors.ErrTooManyRequests("举报过于频繁，请稍后再试"))
	}
	return response.Upstream(c, err, "举报原因或内容链接无效，请刷新后重试")
}

func (h *TrustHandler) Callback(c fiber.Ctx) error {
	body := c.Request().Body()
	if !trustclient.VerifyCallbackSignature(
		h.callbackSecret,
		c.Get("X-Trust-Timestamp"),
		c.Get("X-Trust-Signature"),
		body,
		time.Now(),
	) {
		return response.Error(c, errors.ErrUnauthorized())
	}

	var cb dto.TrustCallback
	if err := json.Unmarshal(body, &cb); err != nil {
		return response.Error(c, errors.ErrBadRequest("回调内容无效"))
	}

	if err := h.enforce.Apply(c.Context(), cb); err != nil {
		slog.Error("trust disposition not applied; infra will redeliver it",
			"disposition_id", cb.DispositionID, "subject_kind", cb.SubjectKind,
			"subject_id", cb.SubjectID, "action", cb.Action, "error", err)
		return response.Error(c, errors.ErrInternal("处置执行失败"))
	}
	return response.OK(c, fiber.Map{"ok": true})
}

func adminFailure(c fiber.Ctx, err error) error {
	if stderrors.Is(err, trustclient.ErrNotConfigured) {
		return response.Error(c, errors.New(50300, "审核服务暂未启用", fiber.StatusServiceUnavailable))
	}
	msg := ""
	switch upstream.KindOf(err) {
	case upstream.NotFound:
		msg = "未找到该审核条目"
	case upstream.Conflict:
		msg = "该条目状态已变化，请刷新后重试"
	case upstream.Rejected:
		msg = "处置参数无效"
		if e, _ := upstream.As(err); e != nil && e.Status == fiber.StatusForbidden {
			msg = "没有审核队列的权限"
		}
	}
	return response.Upstream(c, err, msg)
}

func (h *TrustHandler) ListReviewItems(c fiber.Ctx) error {
	req := &dto.ListReviewItemsRequest{
		Status: fiber.Query(c, "status", -1),
		Source: fiber.Query(c, "source", -1),
		Page:   max(fiber.Query(c, "page", 1), 1),
		Limit:  min(max(fiber.Query(c, "limit", 30), 1), 200),
	}
	data, err := h.trustService.ListReviewItems(c.Context(), middleware.GetAccessToken(c), req)
	if err != nil {
		return adminFailure(c, err)
	}
	return response.OK(c, data)
}

func (h *TrustHandler) GetReviewItem(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的条目 ID"))
	}
	data, err := h.trustService.GetReviewItem(c.Context(), middleware.GetAccessToken(c), id)
	if err != nil {
		return adminFailure(c, err)
	}
	return response.OK(c, data)
}

func (h *TrustHandler) ClaimReviewItem(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的条目 ID"))
	}
	data, err := h.trustService.ClaimReviewItem(c.Context(), middleware.GetAccessToken(c), id)
	if err != nil {
		return adminFailure(c, err)
	}
	return response.OK(c, data)
}

func (h *TrustHandler) DecideReviewItem(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的条目 ID"))
	}
	data, err := h.trustService.DecideReviewItem(c.Context(), middleware.GetAccessToken(c), id, c.Body())
	if err != nil {
		return adminFailure(c, err)
	}
	return response.OK(c, data)
}
