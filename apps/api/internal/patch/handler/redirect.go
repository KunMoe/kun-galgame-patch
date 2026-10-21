package handler

import (
	"context"
	"time"

	"kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/catalogv2"
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
// so this ledger is the only place the successor is still written down. A work
// catalog deleted says so, because a reviewer can still restore it.
func (h *PatchHandler) patchGone(c fiber.Ctx, id int) error {
	if to, ok := h.service.MergeRedirect(id); ok {
		return response.OK(c, fiber.Map{"moved_to": to})
	}
	if d := h.catalogDeletion(c.Context(), id); d != nil {
		return workDeleted(c, d.Reason, d.DeletedAt)
	}
	return response.Error(c, errors.ErrNotFound("patch not found"))
}

// A failed probe leaves the 404 the page answered before deletions existed.
func (h *PatchHandler) catalogDeletion(ctx context.Context, id int) *catalogv2.Deletion {
	v2 := h.catalogV2()
	if v2 == nil || !v2.Configured() {
		return nil
	}
	d, err := v2.WorkDeletion(ctx, int64(id))
	if err != nil {
		return nil
	}
	return d
}

type workDeletion struct {
	Reason    string `json:"deleted_reason"`
	DeletedAt string `json:"deleted_at"`
}

func workDeleted(c fiber.Ctx, reason, deletedAt string) error {
	return response.ErrorData(c, errors.ErrWorkDeleted(), workDeletion{Reason: reason, DeletedAt: deletedAt})
}

func stampedDeletion(p *model.Patch) (reason, deletedAt string, ok bool) {
	if p.CatalogDeletedAt == nil {
		return "", "", false
	}
	if p.CatalogDeletedReason != nil {
		reason = *p.CatalogDeletedReason
	}
	return reason, p.CatalogDeletedAt.UTC().Format(time.RFC3339), true
}
