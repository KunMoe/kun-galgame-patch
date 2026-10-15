package normalize

import (
	"errors"
	"strconv"

	"kun-galgame-patch-api/internal/common/upload"
	apperrors "kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func resourceID(c fiber.Ctx) (int, bool) {
	id, err := strconv.Atoi(c.Params("id"))
	return id, err == nil && id > 0
}

type ClaimRequest struct {
	Limit int `json:"limit" validate:"omitempty,min=1,max=20"`
}

func (h *Handler) Claim(c fiber.Ctx) error {
	var req ClaimRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, apperrors.ErrBadRequest(err.Error()))
	}
	if req.Limit == 0 {
		req.Limit = 1
	}
	jobs, err := h.svc.Claim(c.Context(), min(req.Limit, MaxClaim))
	if err != nil {
		return response.Upstream(c, err, "")
	}
	return response.OK(c, fiber.Map{"items": jobs})
}

func (h *Handler) Complete(c fiber.Ctx, botUserID int) error {
	id, ok := resourceID(c)
	if !ok {
		return response.Error(c, apperrors.ErrBadRequest("invalid resource id"))
	}
	var req CompleteRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, apperrors.ErrBadRequest(err.Error()))
	}
	out, err := h.svc.Complete(c.Context(), id, botUserID, req)
	switch {
	case err == nil:
		return response.OK(c, out)
	case errors.Is(err, ErrResourceNotFound):
		return response.Error(c, apperrors.ErrNotFound(err.Error()))
	case errors.Is(err, ErrSourceChanged), errors.Is(err, ErrAlreadyDone):
		return response.Error(c, apperrors.ErrConflict(err.Error()))
	case errors.Is(err, ErrArtifactUnusable):
		return response.Error(c, apperrors.ErrValidation(err.Error()))
	case errors.Is(err, upload.ErrNotUploadOwner):
		return response.Error(c, apperrors.ErrValidation("artifact_uuid is not an upload the bot started in the last 7 days"))
	default:
		return response.Upstream(c, err, "")
	}
}

func (h *Handler) Fail(c fiber.Ctx) error {
	id, ok := resourceID(c)
	if !ok {
		return response.Error(c, apperrors.ErrBadRequest("invalid resource id"))
	}
	var req FailRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, apperrors.ErrBadRequest(err.Error()))
	}
	err := h.svc.Fail(c.Context(), id, req)
	switch {
	case err == nil:
		return response.OK(c, fiber.Map{"resource_id": id, "source_artifact_uuid": req.SourceArtifactUUID})
	case errors.Is(err, ErrAlreadyDone):
		return response.Error(c, apperrors.ErrConflict(err.Error()))
	case errors.Is(err, ErrNoJob):
		return response.Error(c, apperrors.ErrNotFound(err.Error()))
	default:
		return response.Error(c, apperrors.ErrInternal(""))
	}
}
