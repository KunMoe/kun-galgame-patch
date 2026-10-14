package repack

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

func (h *Handler) Attach(c fiber.Ctx, botUserID int) error {
	id, ok := resourceID(c)
	if !ok {
		return response.Error(c, apperrors.ErrBadRequest("invalid resource id"))
	}
	var req AttachRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, apperrors.ErrBadRequest(err.Error()))
	}
	out, err := h.svc.Attach(c.Context(), id, botUserID, req)
	switch {
	case err == nil:
		return response.OK(c, out)
	case errors.Is(err, ErrResourceNotFound):
		return response.Error(c, apperrors.ErrNotFound(err.Error()))
	case errors.Is(err, ErrSourceChanged):
		return response.Error(c, apperrors.ErrConflict(err.Error()))
	case errors.Is(err, ErrArtifactUnusable):
		return response.Error(c, apperrors.ErrValidation(err.Error()))
	case errors.Is(err, upload.ErrNotUploadOwner):
		return response.Error(c, apperrors.ErrValidation("artifact_uuid is not an upload the bot started in the last 7 days"))
	default:
		return response.Upstream(c, err, "")
	}
}

func (h *Handler) Withdraw(c fiber.Ctx) error {
	id, ok := resourceID(c)
	if !ok {
		return response.Error(c, apperrors.ErrBadRequest("invalid resource id"))
	}
	deleted, err := h.svc.Withdraw(c.Context(), id)
	if err != nil {
		return response.Error(c, apperrors.ErrInternal(""))
	}
	return response.OK(c, fiber.Map{"resource_id": id, "deleted": deleted})
}
