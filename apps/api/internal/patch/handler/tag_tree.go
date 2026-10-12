package handler

import (
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

func (h *PatchHandler) TagDescendants(c fiber.Ctx) error {
	tagID, idErr := tagIDParam(c)
	if idErr != nil {
		return response.Error(c, idErr)
	}
	items, err := h.galgame.TagDescendants(c.Context(), tagID)
	if err != nil {
		return response.Upstream(c, err, "标签不存在")
	}
	return response.OK(c, fiber.Map{"items": items})
}
