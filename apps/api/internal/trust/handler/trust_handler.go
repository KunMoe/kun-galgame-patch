package handler

import (
	"encoding/json"
	"strconv"
	"time"

	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/internal/trust/dto"
	"kun-galgame-patch-api/internal/trust/enforce"
	"kun-galgame-patch-api/internal/trust/service"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/trustclient"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

type TrustHandler struct {
	trustService   *service.TrustService
	enforce        *enforce.Service
	callbackSecret string
}

func NewTrustHandler(
	trustService *service.TrustService,
	enforceService *enforce.Service,
	callbackSecret string,
) *TrustHandler {
	return &TrustHandler{
		trustService:   trustService,
		enforce:        enforceService,
		callbackSecret: callbackSecret,
	}
}

// GetReasons returns the report-reason catalog for the report dropdown.
// GET /api/v1/report/reasons
func (h *TrustHandler) GetReasons(c fiber.Ctx) error {
	return response.OK(c, h.trustService.Reasons(c.Context()))
}

// SubmitReport files a report against a content subject on behalf of the session
// user. Generic — the subject kind/id come straight from the body.
// POST /api/v1/report/submit
func (h *TrustHandler) SubmitReport(c fiber.Ctx) error {
	user := middleware.MustGetUser(c)

	var req dto.SubmitReportRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	res, appErr := h.trustService.SubmitReport(c.Context(), user.ID, &req)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, res)
}

// Callback receives an enforcement disposition from the trust service and
// applies it. PUBLIC route (no session) — authenticated by the HMAC signature
// over the raw body. Idempotent on disposition_id. Returns 200 on success so the
// trust worker marks the callback delivered; any non-2xx triggers its
// retry/dead-letter path.
// POST /api/v1/trust/callback
func (h *TrustHandler) Callback(c fiber.Ctx) error {
	body := c.Body()
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
		return response.Error(c, errors.ErrInternal("处置执行失败"))
	}
	return response.OK(c, fiber.Map{"ok": true})
}

// ─────────────────────────────────────────────
// Moderator inbox proxy (Phase 3) — moderator-gated. Forwards the moderator's
// OAuth token to the trust admin API; site is forced to moyu.
// ─────────────────────────────────────────────

// ListReviewItems — GET /api/v1/admin/trust/review-items
func (h *TrustHandler) ListReviewItems(c fiber.Ctx) error {
	req := &dto.ListReviewItemsRequest{
		Status: fiber.Query(c, "status", -1),
		Source: fiber.Query(c, "source", -1),
		Page:   max(fiber.Query(c, "page", 1), 1),
		Limit:  min(max(fiber.Query(c, "limit", 30), 1), 200),
	}
	data, appErr := h.trustService.ListReviewItems(c.Context(), middleware.GetAccessToken(c), req)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, data)
}

// GetReviewItem — GET /api/v1/admin/trust/review-items/:id
func (h *TrustHandler) GetReviewItem(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的条目 ID"))
	}
	data, appErr := h.trustService.GetReviewItem(c.Context(), middleware.GetAccessToken(c), id)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, data)
}

// ClaimReviewItem — POST /api/v1/admin/trust/review-items/:id/claim
func (h *TrustHandler) ClaimReviewItem(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的条目 ID"))
	}
	data, appErr := h.trustService.ClaimReviewItem(c.Context(), middleware.GetAccessToken(c), id)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, data)
}

// DecideReviewItem — POST /api/v1/admin/trust/review-items/:id/decide
// The body (DecideRequest) is forwarded to the trust service verbatim, which
// owns its validation.
func (h *TrustHandler) DecideReviewItem(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的条目 ID"))
	}
	data, appErr := h.trustService.DecideReviewItem(c.Context(), middleware.GetAccessToken(c), id, c.Body())
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, data)
}
