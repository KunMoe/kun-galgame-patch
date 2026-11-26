package handler

import (
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

func (h *Handler) GetComment(c fiber.Ctx) error {
	postID, appErr := postIDParam(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	item, surface, err := h.service.Post(c.Context(), postID, middleware.GetUserID(c))
	if err != nil {
		return fail(c, err, onPost)
	}
	if !h.gate(c, surface.PatchID) {
		return response.Error(c, errors.ErrNotFound("comment not found"))
	}
	return response.OK(c, item)
}

func (h *Handler) GetCommentReplies(c fiber.Ctx) error {
	postID, appErr := postIDParam(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req struct {
		After string `query:"after" validate:"omitempty,numeric,max=10"`
		Limit int    `query:"limit" validate:"omitempty,min=1,max=50"`
	}
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	page, surface, err := h.service.Replies(c.Context(), postID, middleware.GetUserID(c), req.After, req.Limit)
	if err != nil {
		return fail(c, err, onPost)
	}
	if !h.gate(c, surface.PatchID) {
		return response.Error(c, errors.ErrNotFound("comment not found"))
	}
	return response.OK(c, page)
}

func (h *Handler) PinComment(c fiber.Ctx) error { return h.setPinned(c, true) }

func (h *Handler) UnpinComment(c fiber.Ctx) error { return h.setPinned(c, false) }

func (h *Handler) setPinned(c fiber.Ctx, pinned bool) error {
	postID, appErr := postIDParam(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	item, err := h.service.SetPinned(c.Context(), postID, middleware.MustGetUser(c).ID, pinned)
	if err != nil {
		return fail(c, err, onPin)
	}
	return response.OK(c, item)
}

func (h *Handler) RestoreComment(c fiber.Ctx) error {
	postID, appErr := postIDParam(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	item, err := h.service.Restore(c.Context(), postID, middleware.MustGetUser(c).ID)
	if err != nil {
		return fail(c, err, onRestore)
	}
	return response.OK(c, item)
}
