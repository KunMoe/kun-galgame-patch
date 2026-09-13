package handler

import (
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

// GetLegacyRedirect backs the /patch/<n> shim. Every page number this site used
// before migration 037 resolves here and nowhere else: the same number is
// usually a live catalog work too, so /galgame/<n> means a different game and
// must never consult this lane.
func (h *PatchHandler) GetLegacyRedirect(c fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	to, ok := h.service.LegacyRedirect(id)
	if !ok {
		return response.Error(c, errors.ErrNotFound("patch not found"))
	}
	return response.OK(c, fiber.Map{"moved_to": to})
}

// patchGone is the answer for a page id that resolves to nothing. When catalog
// merged that work away the successor is knowable and the browser gets a 301
// out of it; the merge erases the claim naming the work in the same statement,
// so this ledger is the only place the successor is still written down.
func (h *PatchHandler) patchGone(c fiber.Ctx, id int) error {
	if to, ok := h.service.MergeRedirect(id); ok {
		return response.OK(c, fiber.Map{"moved_to": to})
	}
	return response.Error(c, errors.ErrNotFound("patch not found"))
}
