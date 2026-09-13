package handler

import (
	"log/slog"

	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/catalogv2"

	"github.com/gofiber/fiber/v3"
)

// If-Match is required on a claim PATCH; this takes whatever state it is in.
const anyState = "*"

func (h *PatchHandler) claimOnFirstResource(c fiber.Ctx, gid int) {
	token := middleware.GetAccessToken(c)
	if token == "" {
		return
	}
	workID := int64(gid)
	if appErr := adoptAndPublish(c, h.catalogV2(), token, workID); appErr != nil {
		slog.Warn("resource: 静默收录 catalog 作品失败", "gid", gid, "work_id", workID, "error", appErr)
	}
}

func adoptAndPublish(c fiber.Ctx, v2 *catalogv2.Client, token string, workID int64) error {
	if v2 == nil || !v2.Configured() {
		return catalogv2.ErrNotConfigured
	}
	_, claimErr := v2.CreateClaim(c.Context(), token, workID, workID)
	if _, pubErr := v2.PatchClaim(c.Context(), token, workID, catalogv2.ClaimStateLive, anyState); pubErr != nil {
		if claimErr != nil {
			return claimErr
		}
		return pubErr
	}
	return nil
}

func (h *PatchHandler) patchClaim(c fiber.Ctx, token string, workID int64, state string) error {
	v2 := h.catalogV2()
	if v2 == nil || !v2.Configured() {
		return catalogv2.ErrNotConfigured
	}
	_, err := v2.PatchClaim(c.Context(), token, workID, state, anyState)
	return err
}

// Withdrawing only moves a claim back to draft, and a draft is invisible to the
// person who filed it — /galgame/mine lists pending and declined, and nothing
// here submits from draft — so a retracted wizard submission used to sit in
// catalog forever, unreachable by its own author. Deleting it is the point, but
// only from pending: DELETE soft-deletes the catalog work, and a live claim's
// work predates this site, so that would take a VNDB entry down with the patch
// page. The read's ETag is what keeps that decision honest — an approval racing
// it answers 412 instead of pointing the delete at a work that just went live.
func (h *PatchHandler) withdrawClaim(c fiber.Ctx, token string, workID int64) error {
	v2 := h.catalogV2()
	if v2 == nil || !v2.Configured() {
		return catalogv2.ErrNotConfigured
	}
	claim, etag, err := v2.GetMyClaim(c.Context(), token, workID)
	if err != nil {
		return err
	}
	if claim.State == catalogv2.ClaimStateDraft {
		return v2.DeleteClaim(c.Context(), token, workID)
	}
	if _, err := v2.PatchClaim(c.Context(), token, workID, catalogv2.ClaimTargetWithdrawn, etag); err != nil {
		return err
	}
	if claim.State != catalogv2.ClaimStatePending {
		return nil
	}
	return v2.DeleteClaim(c.Context(), token, workID)
}
