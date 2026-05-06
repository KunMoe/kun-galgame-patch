package handler

import (
	"kun-galgame-patch-api/internal/message/dto"
	"kun-galgame-patch-api/internal/message/service"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

type MessageHandler struct {
	service *service.MessageService
}

func New(svc *service.MessageService) *MessageHandler {
	return &MessageHandler{service: svc}
}

// GetMessages GET /api/message
func (h *MessageHandler) GetMessages(c *fiber.Ctx) error {
	var req dto.GetMessageRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	user := middleware.MustGetUser(c)
	messages, total, err := h.service.GetMessages(user.UID, req.Type, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	return response.Paginated(c, messages, total)
}

// GetAllMessages GET /api/message/all
//
// Same as GET /api/message but ignores the type filter. Kept as a separate
// route for parity with the legacy frontend that has /message/all hard-coded.
func (h *MessageHandler) GetAllMessages(c *fiber.Ctx) error {
	var req dto.GetMessageRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	user := middleware.MustGetUser(c)
	messages, total, err := h.service.GetMessages(user.UID, "", req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	return response.Paginated(c, messages, total)
}

// GetUnreadTypes GET /api/message/unread
func (h *MessageHandler) GetUnreadTypes(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	types, err := h.service.GetUnreadTypes(user.UID)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.OK(c, types)
}

// CreateMessage POST /api/message
func (h *MessageHandler) CreateMessage(c *fiber.Ctx) error {
	var req dto.CreateMessageRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	user := middleware.MustGetUser(c)
	if err := h.service.CreateMessage(user.UID, req.RecipientID, req.Type, req.Content, req.Link); err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	return response.OKMessage(c, "Message sent")
}

// MarkAsRead PUT /api/message/read
func (h *MessageHandler) MarkAsRead(c *fiber.Ctx) error {
	var req dto.ReadMessageRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	user := middleware.MustGetUser(c)
	if err := h.service.MarkAsRead(user.UID, req.Type); err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	return response.OKMessage(c, "Messages marked as read")
}
