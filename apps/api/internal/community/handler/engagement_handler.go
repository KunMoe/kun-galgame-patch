package handler

import (
	"kun-galgame-patch-api/internal/community/anchor"
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

type wallRequest struct {
	Kind string `json:"kind" validate:"required,oneof=patch resource"`
	ID   int    `json:"id" validate:"required,min=1"`
}

type wallNotificationRequest struct {
	wallRequest
	Level int32 `json:"level"`
}

// ReadWall is a POST and not a side effect of the wall's GET: a read face that
// writes cannot be cached, retried or prefetched safely.
func (h *EngagementHandler) ReadWall(c fiber.Ctx) error {
	var req wallRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	user := middleware.MustGetUser(c)
	kind, id := wallAnchor(req.Kind, req.ID)
	return response.OK(c, h.service.ReadWall(c.Context(), user.ID, kind, id))
}

func (h *EngagementHandler) SetWallNotification(c fiber.Ctx) error {
	var req wallNotificationRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	user := middleware.MustGetUser(c)
	kind, id := wallAnchor(req.Kind, req.ID)
	state, err := h.service.SetWallLevel(c.Context(), user.ID, kind, id, req.Level)
	if err != nil {
		return fail(c, err)
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
	res, err := h.service.Unread(c.Context(), user.ID, req.Cursor, req.Limit)
	if err != nil {
		return fail(c, err)
	}
	return response.OK(c, res)
}

func fail(c fiber.Ctx, err error) error {
	if appErr, ok := err.(*errors.AppError); ok {
		return response.Error(c, appErr)
	}
	return response.Upstream(c, err, "评论区不存在")
}

func wallAnchor(kind string, id int) (int32, string) {
	if kind == "resource" {
		return anchor.ResourceAnchor(id)
	}
	return anchor.PatchAnchor(id)
}
