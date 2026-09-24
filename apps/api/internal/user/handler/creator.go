package handler

import (
	"encoding/json"
	stderrors "errors"

	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

func creatorErr(c fiber.Ctx, err error) error {
	var appErr *errors.AppError
	if stderrors.As(err, &appErr) {
		return response.Error(c, appErr)
	}
	return response.Upstream(c, err, "")
}

func (h *UserHandler) CreatorStatus(c fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	token := middleware.GetAccessToken(c)
	if userID == 0 || token == "" {
		return response.Error(c, errors.ErrUnauthorized())
	}
	elig, app, err := h.service.CreatorStatus(c.Context(), userID, token)
	if err != nil {
		return creatorErr(c, err)
	}
	return response.OK(c, fiber.Map{"eligibility": elig, "application": app})
}

func (h *UserHandler) CreatorApply(c fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	token := middleware.GetAccessToken(c)
	if userID == 0 || token == "" {
		return response.Error(c, errors.ErrUnauthorized())
	}
	var body struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(c.Body(), &body)
	app, err := h.service.ApplyCreator(c.Context(), userID, token, body.Message)
	if err != nil {
		return creatorErr(c, err)
	}
	return response.OK(c, app)
}
