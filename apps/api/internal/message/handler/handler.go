package handler

import (
	"context"

	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/internal/message/dto"
	"kun-galgame-patch-api/internal/message/service"
	"kun-galgame-patch-api/internal/middleware"
	userModel "kun-galgame-patch-api/internal/user/model"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/userclient"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

type MessageHandler struct {
	service *service.MessageService
	users   *userclient.Client
}

func New(svc *service.MessageService, users *userclient.Client) *MessageHandler {
	return &MessageHandler{service: svc, users: users}
}

// attachSenders batch-resolves sender briefs from OAuth /users/batch and
// stamps msg.Sender. Without this every message serialized with sender=nil,
// and the frontend's message/Card.vue rendered "系统" for all of them.
//
// Best-effort: messages whose sender_id is NULL (true system messages) or
// whose brief can't be resolved keep Sender=nil and correctly fall back to
// the "系统" placeholder on the frontend.
func (h *MessageHandler) attachSenders(ctx context.Context, msgs []userModel.UserMessage) {
	if h.users == nil || len(msgs) == 0 {
		return
	}
	ids := make([]int, 0, len(msgs))
	for i := range msgs {
		if msgs[i].SenderID != nil && *msgs[i].SenderID > 0 {
			ids = append(ids, *msgs[i].SenderID)
		}
	}
	if len(ids) == 0 {
		return
	}
	briefs := userclient.BriefMapByInt(ctx, h.users, ids)
	for i := range msgs {
		if msgs[i].SenderID == nil {
			continue
		}
		if b := briefs[*msgs[i].SenderID]; b != nil {
			msgs[i].Sender = &patchModel.PatchUser{
				ID:              int(b.ID),
				Name:            b.Name,
				Avatar:          b.Avatar,
				AvatarImageHash: b.AvatarImageHash,
				Roles:           b.Roles,
			}
		}
	}
}

// GetMessages GET /api/message
func (h *MessageHandler) GetMessages(c *fiber.Ctx) error {
	var req dto.GetMessageRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	user := middleware.MustGetUser(c)
	messages, total, err := h.service.GetMessages(user.ID, req.Type, req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	h.attachSenders(c.Context(), messages)
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
	messages, total, err := h.service.GetMessages(user.ID, "", req.Page, req.Limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	h.attachSenders(c.Context(), messages)
	return response.Paginated(c, messages, total)
}

// GetUnreadTypes GET /api/message/unread
func (h *MessageHandler) GetUnreadTypes(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	types, err := h.service.GetUnreadTypes(user.ID)
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
	if err := h.service.CreateMessage(user.ID, req.RecipientID, req.Type, req.Content, req.Link); err != nil {
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
	if err := h.service.MarkAsRead(user.ID, req.Type); err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	return response.OKMessage(c, "Messages marked as read")
}
