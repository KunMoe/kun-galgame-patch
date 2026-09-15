package handler

import (
	stderrors "errors"
	"strings"

	"kun-galgame-patch-api/internal/face/service"
	"kun-galgame-patch-api/internal/patch/model"
	patchsvc "kun-galgame-patch-api/internal/patch/service"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

type botResourceRequest struct {
	Name         string   `json:"name"`
	Size         string   `json:"size"`
	ArtifactUUID string   `json:"artifact_uuid"`
	Note         string   `json:"note"`
	Type         []string `json:"type"`
	Language     []string `json:"language"`
	Platform     []string `json:"platform"`
}

func (h *PatchHandler) BotCoverage(c fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	types, err := h.service.ResourceTypes(id)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	if types == nil {
		types = []string{}
	}
	return response.OK(c, map[string]any{"types": types})
}

func (h *PatchHandler) BotCreateResource(c fiber.Ctx, userID int) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	var req botResourceRequest
	if err := c.Bind().Body(&req); err != nil {
		return response.Error(c, errors.ErrBadRequest("invalid body"))
	}
	if strings.TrimSpace(req.ArtifactUUID) == "" || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Size) == "" {
		return response.Error(c, errors.ErrValidation("name, size and artifact_uuid are required"))
	}
	if msg := validateBotResource(req.Type, req.Language, req.Platform); msg != "" {
		return response.Error(c, errors.ErrValidation(msg))
	}
	if _, err := h.service.CreatePatchByGalgameID(c.Context(), userID, id); err != nil {
		if stderrors.Is(err, patchsvc.ErrGalgameMissing) {
			return response.Error(c, errors.ErrGalgameNotFound(""))
		}
		return catalogErr(c, err, "无法创建补丁页")
	}
	resource := &model.PatchResource{
		GalgameID:    id,
		Storage:      "s3",
		Name:         req.Name,
		ArtifactUUID: req.ArtifactUUID,
		Size:         req.Size,
		Note:         req.Note,
		Type:         model.JSONArray(req.Type),
		Language:     model.JSONArray(req.Language),
		Platform:     model.JSONArray(req.Platform),
	}
	if err := h.service.BotCreateResource(c.Context(), resource, userID); err != nil {
		return response.Error(c, errors.ErrValidation(err.Error()))
	}
	return response.OK(c, map[string]any{"id": resource.ID, "galgame_id": id})
}

func validateBotResource(types, langs, plats []string) string {
	if len(types) == 0 {
		return "type is required"
	}
	if len(langs) == 0 {
		return "language is required"
	}
	if len(plats) == 0 {
		return "platform is required"
	}
	if msg := closedVocab("type", types, service.PatchTypes); msg != "" {
		return msg
	}
	for _, t := range types {
		if t == "crack" {
			return "bot cannot submit crack patches"
		}
	}
	if msg := closedVocab("language", langs, service.PatchLanguages); msg != "" {
		return msg
	}
	return closedVocab("platform", plats, service.PatchPlatforms)
}

func closedVocab(kind string, got, allowed []string) string {
	ok := map[string]bool{}
	for _, a := range allowed {
		ok[a] = true
	}
	for _, g := range got {
		if !ok[g] {
			return "unknown " + kind + " " + g
		}
	}
	return ""
}
