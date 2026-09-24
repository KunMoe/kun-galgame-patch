package handler

import (
	"log/slog"
	"strconv"

	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/catalogv2"
	"kun-galgame-patch-api/pkg/upstream"

	"github.com/gofiber/fiber/v3"
)

// If-Match is required on a claim PATCH; this takes whatever state it is in.
const anyState = "*"

// Only the resource that published the page adopts its work. Every upload used
// to send the pair, spending the uploader's shared catalog allowance, and their
// catalog.claim_writes_per_day, on works adopted long before.
func (h *PatchHandler) claimOnFirstResource(c fiber.Ctx, gid int, published bool) {
	token := middleware.GetAccessToken(c)
	if !published || token == "" {
		return
	}
	workID := int64(gid)
	if appErr := adoptAndPublish(c, h.catalogV2(), token, middleware.GetUserID(c), workID); appErr != nil {
		slog.Warn("resource: 静默收录 catalog 作品失败", "gid", gid, "work_id", workID, "error", appErr)
	}
}

func adoptAndPublish(c fiber.Ctx, v2 *catalogv2.Client, token string, actor int, workID int64) error {
	if v2 == nil || !v2.Configured() {
		return catalogv2.ErrNotConfigured
	}
	// Keyed on the work alone, unlike the reader's creates: a replayed answer
	// changes nothing here, since the claim either exists or is made and the
	// PATCH below is what decides the outcome.
	_, claimErr := v2.CreateClaim(c.Context(), token,
		upstream.IdempotencyKey(strconv.Itoa(actor), "catalog.adopt", strconv.FormatInt(workID, 10)), workID, workID)
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
//
// Once the withdraw has landed the submission is off its author's list whatever
// the delete does, so a delete that fails after it is logged rather than
// answered as a failed withdraw.
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
	if err := v2.DeleteClaim(c.Context(), token, workID); err != nil {
		slog.Warn("withdraw: the claim is withdrawn but its draft was not deleted",
			"work_id", workID, "error", err)
	}
	return nil
}
