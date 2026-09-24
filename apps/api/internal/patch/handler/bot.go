package handler

import (
	stderrors "errors"
	"strings"

	"kun-galgame-patch-api/internal/patch/model"
	patchsvc "kun-galgame-patch-api/internal/patch/service"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

// The lane is 汉化 / 修正 / 存档 / 破解. decensor / r18 / mod stay human-only.
var botSubmittableTypes = []string{"manual", "ai", "machine_polishing", "machine", "fix", "save", "crack"}

type botResourceRequest struct {
	Name         string   `json:"name" validate:"required,max=300"`
	Size         string   `json:"size" validate:"required,max=107"`
	ArtifactUUID string   `json:"artifact_uuid" validate:"required,max=36"`
	Note         string   `json:"note" validate:"max=10007"`
	Type         []string `json:"type" validate:"required,min=1,max=10"`
	Language     []string `json:"language" validate:"required,min=1,max=10"`
	Platform     []string `json:"platform" validate:"required,min=1,max=10"`
}

func (h *PatchHandler) BotCoverage(c fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	coverage, err := h.service.ResourceCoverage(id)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.OK(c, coverage)
}

func (h *PatchHandler) BotCreateResource(c fiber.Ctx, userID int) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	var req botResourceRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if msg := validateBotResource(req.Type, req.Language, req.Platform); msg != "" {
		return response.Error(c, errors.ErrValidation(msg))
	}
	artifactUUID := strings.TrimSpace(req.ArtifactUUID)

	// Before CreatePatchByGalgameID, not after: a rejected artifact used to
	// leave behind the page it had already minted, plus its moemoepoint award
	// and contributor row, for a submission that never landed a resource.
	if err := h.service.EnsureArtifactReady(c.Context(), artifactUUID); err != nil {
		if stderrors.Is(err, patchsvc.ErrArtifactNotReady) {
			return response.Error(c, errors.ErrValidation(err.Error()))
		}
		return response.Upstream(c, err, "")
	}
	if _, err := h.service.CreatePatchByGalgameID(c.Context(), userID, id); err != nil {
		if stderrors.Is(err, patchsvc.ErrGalgameMissing) {
			return response.Error(c, errors.ErrGalgameNotFound(""))
		}
		return catalogErr(c, err, "无法创建补丁页")
	}

	// No adoptUnlessLive here. Adopting the catalog work needs the publisher's
	// own OAuth token (POST /v2/me/claims) and a bot key is not one, so a
	// bot-published page stays unclaimed in catalog until a person uploads to
	// it. Closing that needs a bot user token from infra, not a code fix.
	resource := &model.PatchResource{
		GalgameID:    id,
		Storage:      "s3",
		Name:         req.Name,
		ArtifactUUID: artifactUUID,
		Size:         req.Size,
		Note:         req.Note,
		Type:         model.JSONArray(req.Type),
		Language:     model.JSONArray(req.Language),
		Platform:     model.JSONArray(req.Platform),
	}
	if err := h.service.CreateResource(c.Context(), resource, userID); err != nil {
		return response.Error(c, errors.ErrValidation(err.Error()))
	}
	return response.OK(c, map[string]any{"id": resource.ID, "galgame_id": id})
}

func validateBotResource(types, langs, plats []string) string {
	return validateVocab(types, langs, plats, botSubmittableTypes)
}
