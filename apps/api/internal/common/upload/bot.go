package upload

import (
	"kun-galgame-patch-api/internal/constants"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

// A bot key cannot open /api/v1/upload (that group is session-gated), so
// without these the submit lane asks for an artifact_uuid the caller has no way
// to mint -- the only alternative being to hand moyu's artifact client secret
// to the forge, which would let it write under this site's key unsupervised.
//
// Deliberately no /abort: Service.Abort deletes any artifact by uuid with no
// ownership check, and that is not a capability to hand a machine key. An
// interrupted multipart is reclaimed by the artifact service's orphan GC.
var botUploadTier = constants.CreatorUploadTier

func (h *Handler) BotInit(c fiber.Ctx, userID int) error {
	var req InitRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	resp, err := h.svc.Init(c.Context(), userID, botUploadTier, req)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, resp)
}

func (h *Handler) BotComplete(c fiber.Ctx, userID int) error {
	var req CompleteRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	resp, err := h.svc.Complete(c.Context(), userID, botUploadTier, req)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, resp)
}

func (h *Handler) BotResume(c fiber.Ctx) error {
	var req ResumeRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	resp, err := h.svc.Resume(c.Context(), req)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, resp)
}
