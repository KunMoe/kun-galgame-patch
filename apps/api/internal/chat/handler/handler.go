// Package handler contains HTTP handlers for the chat module.
//
// D9 (2026-04-21): 9 REST endpoints, no WebSocket involved.
package handler

import (
	"context"
	"strconv"

	chatModel "kun-galgame-patch-api/internal/chat/model"
	"kun-galgame-patch-api/internal/chat/dto"
	"kun-galgame-patch-api/internal/chat/service"
	"kun-galgame-patch-api/internal/middleware"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/userclient"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

type ChatHandler struct {
	svc   *service.ChatService
	users *userclient.Client
}

func New(svc *service.ChatService, users *userclient.Client) *ChatHandler {
	return &ChatHandler{svc: svc, users: users}
}

// attach helpers stamp the embedded user/sender field on rows after they
// come back from the repository (Preload was removed in Phase 6e).
func (h *ChatHandler) attachMessageSenders(ctx context.Context, msgs []chatModel.ChatMessage) {
	uids := make([]int, 0, len(msgs))
	for _, m := range msgs {
		uids = append(uids, m.SenderID)
	}
	briefs := userclient.BriefMapByInt(ctx, h.users, uids)
	for i := range msgs {
		if b := briefs[msgs[i].SenderID]; b != nil {
			msgs[i].Sender = &patchModel.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar}
		}
	}
}

func (h *ChatHandler) attachMemberUsers(ctx context.Context, ms []chatModel.ChatMember) {
	uids := make([]int, 0, len(ms))
	for _, m := range ms {
		uids = append(uids, m.UserID)
	}
	briefs := userclient.BriefMapByInt(ctx, h.users, uids)
	for i := range ms {
		if b := briefs[ms[i].UserID]; b != nil {
			ms[i].User = &patchModel.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar}
		}
	}
}

func (h *ChatHandler) attachOneSender(ctx context.Context, msg *chatModel.ChatMessage) {
	if msg == nil || msg.SenderID == 0 {
		return
	}
	if b, _ := h.users.User(ctx, uint(msg.SenderID)); b != nil {
		msg.Sender = &patchModel.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar}
	}
}

func getMessageIDParam(c *fiber.Ctx) (int, error) {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id < 1 {
		return 0, errors.ErrBadRequest("invalid message id")
	}
	return id, nil
}

// ─── Room ───────────────────────────────────────────

// ListRooms GET /api/chat/room
func (h *ChatHandler) ListRooms(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	rooms, err := h.svc.ListRooms(user.UID)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.OK(c, rooms)
}

// CreateRoom POST /api/chat/room   (admin only)
func (h *ChatHandler) CreateRoom(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	if !middleware.HasRole(c, "admin") {
		return response.Error(c, errors.ErrForbidden())
	}
	var req dto.CreateRoomRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	room, err := h.svc.CreateGroupRoom(user.UID, req.Name, req.Avatar)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, room)
}

// GetRoomDetail GET /api/chat/room/:link
//
// Returns the room plus its full member list (with each user's profile).
// Caller must be a member.
func (h *ChatHandler) GetRoomDetail(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	link := c.Params("link")
	detail, err := h.svc.GetRoomDetail(user.UID, link)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	h.attachMemberUsers(c.Context(), detail.Member)
	return response.OK(c, detail)
}

// JoinRoom POST /api/chat/room/join
func (h *ChatHandler) JoinRoom(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	var req dto.JoinRoomRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	room, err := h.svc.JoinRoomByLink(user.UID, req.Link)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, room)
}

// StartPrivate POST /api/chat/room/private
//
// Returns the (created or existing) private chat room between the caller
// and req.PeerUID. Front-end clicks "发消息" on a user profile, posts here
// with peer_uid, then navigates to /message/chat/<room.link>.
func (h *ChatHandler) StartPrivate(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	var req dto.StartPrivateChatRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if req.PeerUID == user.UID {
		return response.Error(c, errors.ErrBadRequest("不能给自己发消息"))
	}
	room, err := h.svc.StartPrivateChat(user.UID, req.PeerUID)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, room)
}

// ─── Messages ───────────────────────────────────────

// ListMessages GET /api/chat/room/:link/message?after=&limit=
func (h *ChatHandler) ListMessages(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	link := c.Params("link")

	var q dto.ListMessagesQuery
	if err := utils.ParseQueryAndValidate(c, &q); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if q.Limit == 0 {
		q.Limit = 30
	}

	msgs, err := h.svc.GetMessages(user.UID, link, q.After, q.Limit)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	h.attachMessageSenders(c.Context(), msgs)
	return response.OK(c, msgs)
}

// CreateMessage POST /api/chat/room/:link/message
func (h *ChatHandler) CreateMessage(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	link := c.Params("link")

	var req dto.CreateMessageRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	msg, err := h.svc.CreateMessage(user.UID, link, req.Content, req.FileURL, req.ReplyToID)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	h.attachOneSender(c.Context(), msg)
	return response.OK(c, msg)
}

// UpdateMessage PUT /api/chat/message/:id
func (h *ChatHandler) UpdateMessage(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	id, err := getMessageIDParam(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.UpdateMessageRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	if err := h.svc.UpdateMessage(user.UID, id, req.Content); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OKMessage(c, "消息已编辑")
}

// DeleteMessage DELETE /api/chat/message/:id
func (h *ChatHandler) DeleteMessage(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	id, err := getMessageIDParam(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	isPrivileged := middleware.HasAnyRole(c, "admin", "moderator")
	if err := h.svc.DeleteMessage(user.UID, isPrivileged, id); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OKMessage(c, "消息已删除")
}

// ToggleReaction POST /api/chat/message/:id/reaction
func (h *ChatHandler) ToggleReaction(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	id, err := getMessageIDParam(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.ReactionRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	added, err := h.svc.ToggleReaction(user.UID, id, req.Emoji)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, map[string]bool{"added": added})
}

// MarkSeen PUT /api/chat/room/:link/seen
func (h *ChatHandler) MarkSeen(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	link := c.Params("link")

	var req dto.SeenRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if err := h.svc.MarkSeen(user.UID, link, req.MessageIDs); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OKMessage(c, "已标记")
}
