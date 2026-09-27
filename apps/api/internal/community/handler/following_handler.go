package handler

import (
	"strconv"
	"time"

	"kun-galgame-patch-api/internal/community/following"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

type FollowingHandler struct {
	service *following.Service
}

func NewFollowingHandler(service *following.Service) *FollowingHandler {
	return &FollowingHandler{service: service}
}

type feedPageQuery struct {
	Cursor string `query:"cursor" validate:"max=512"`
	Limit  int    `query:"limit" validate:"omitempty,min=1,max=50"`
}

type seenRequest struct {
	At *time.Time `json:"at"`
}

func feedLimit(c fiber.Ctx) string {
	return following.FeedLimit(utils.ContentLimitFromQuery(c))
}

func (h *FollowingHandler) Groups(c fiber.Ctx) error {
	var q feedPageQuery
	if err := utils.ParseQueryAndValidate(c, &q); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	user := middleware.MustGetUser(c)
	page, err := h.service.Groups(c.Context(), user.ID, q.Cursor, q.Limit, feedLimit(c))
	if err != nil {
		return response.Upstream(c, err, "")
	}
	return response.OK(c, page)
}

func (h *FollowingHandler) GroupItems(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id <= 0 {
		return response.Error(c, errors.ErrBadRequest("invalid group id"))
	}
	var q feedPageQuery
	if err := utils.ParseQueryAndValidate(c, &q); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	page, err := h.service.GroupItems(c.Context(), id, q.Cursor, q.Limit, feedLimit(c))
	if err != nil {
		return response.Upstream(c, err, "这组动态已不存在")
	}
	return response.OK(c, page)
}

func (h *FollowingHandler) Unseen(c fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	n, err := h.service.Unseen(c.Context(), user.ID, feedLimit(c))
	if err != nil {
		return response.Upstream(c, err, "")
	}
	return response.OK(c, fiber.Map{"unseen_count": n})
}

func (h *FollowingHandler) MarkSeen(c fiber.Ctx) error {
	var req seenRequest
	if len(c.Body()) > 0 {
		if err := utils.ParseAndValidate(c, &req); err != nil {
			return response.Error(c, errors.ErrBadRequest(err.Error()))
		}
	}
	user := middleware.MustGetUser(c)
	at, err := h.service.MarkSeen(c.Context(), user.ID, req.At)
	if err != nil {
		return response.Upstream(c, err, "")
	}
	return response.OK(c, fiber.Map{"seen_at": at})
}
