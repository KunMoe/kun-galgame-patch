package handler

import (
	"strconv"

	"kun-galgame-patch-api/internal/community/engagement"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

type EngagementHandler struct {
	service *engagement.Service
}

func NewEngagementHandler(service *engagement.Service) *EngagementHandler {
	return &EngagementHandler{service: service}
}

// MarkRead is a POST and not a side effect of the wall's GET: a read face that
// writes cannot be cached, retried or prefetched safely.
func (h *EngagementHandler) MarkRead(c fiber.Ctx) error {
	threadID, appErr := threadIDParam(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	user := middleware.MustGetUser(c)
	return response.OK(c, h.service.MarkRead(c.Context(), user.ID, threadID))
}

func (h *EngagementHandler) SetNotification(c fiber.Ctx) error {
	threadID, appErr := threadIDParam(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req struct {
		Level int32 `json:"level" validate:"min=0,max=3"`
	}
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	user := middleware.MustGetUser(c)
	state, appErr := h.service.SetLevel(c.Context(), user.ID, threadID, req.Level)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, state)
}

func (h *EngagementHandler) Unread(c fiber.Ctx) error {
	var req struct {
		Cursor string `query:"cursor" validate:"omitempty,max=256"`
		Limit  int    `query:"limit" validate:"omitempty,min=1,max=50"`
	}
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	user := middleware.MustGetUser(c)
	res, appErr := h.service.Unread(c.Context(), user.ID, req.Cursor, req.Limit)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, res)
}

func (h *EngagementHandler) UnreadCount(c fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	return response.OK(c, fiber.Map{"total": h.service.Count(c.Context(), user.ID)})
}

func threadIDParam(c fiber.Ctx) (int64, *errors.AppError) {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.ErrBadRequest("评论区 ID 不正确")
	}
	return id, nil
}
