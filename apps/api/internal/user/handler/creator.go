package handler

import (
	"encoding/json"

	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// CreatorStatus — GET /api/user/creator/status: moyu eligibility snapshot +
// the user's current creator application (from the central OAuth queue).
func (h *UserHandler) CreatorStatus(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		return response.Error(c, errors.ErrUnauthorized())
	}
	token := middleware.GetAccessToken(c)
	elig, app, appErr := h.service.CreatorStatus(c.Context(), userID, token)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, fiber.Map{"eligibility": elig, "application": app})
}

// CreatorApply — POST /api/user/creator/apply {message?}: checks moyu's
// eligibility gate then files the application on the OAuth queue.
func (h *UserHandler) CreatorApply(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		return response.Error(c, errors.ErrUnauthorized())
	}
	token := middleware.GetAccessToken(c)
	var body struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(c.Body(), &body)
	app, appErr := h.service.ApplyCreator(c.Context(), userID, token, body.Message)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, app)
}
