// Package handler contains HTTP handlers for the chat module.
//
// D9 (2026-04-21): 9 REST endpoints, no WebSocket involved.
package handler

import (
	"context"
	"strconv"
	"strings"

	chatModel "kun-galgame-patch-api/internal/chat/model"
	"kun-galgame-patch-api/internal/chat/dto"
	"kun-galgame-patch-api/internal/chat/service"
	"kun-galgame-patch-api/internal/infrastructure/markdown"
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
			msgs[i].Sender = &patchModel.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar, AvatarImageHash: b.AvatarImageHash, Roles: b.Roles}
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
			ms[i].User = &patchModel.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar, AvatarImageHash: b.AvatarImageHash, Roles: b.Roles}
		}
	}
}

func (h *ChatHandler) attachOneSender(ctx context.Context, msg *chatModel.ChatMessage) {
	if msg == nil || msg.SenderID == 0 {
		return
	}
	if b, _ := h.users.User(ctx, uint(msg.SenderID)); b != nil {
		msg.Sender = &patchModel.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar, AvatarImageHash: b.AvatarImageHash, Roles: b.Roles}
	}
}

// enrichMessages fills ContentHTML (rendered markdown), Reaction[] (with the
// reacting users' briefs) and QuoteMessage (the replied-to message preview)
// for a page of messages, using batched queries so there is no N+1.
//
// Senders are assumed already attached by attachMessageSenders. DELETED
// messages are left with empty ContentHTML (the frontend renders a tombstone
// chip and never reaches the markdown branch).
func (h *ChatHandler) enrichMessages(ctx context.Context, msgs []chatModel.ChatMessage) {
	if len(msgs) == 0 {
		return
	}

	// 1. Markdown → sanitized HTML. Content stays raw (edit modal needs it).
	for i := range msgs {
		if msgs[i].Status == "DELETED" {
			continue
		}
		msgs[i].ContentHTML = markdown.MustRender(msgs[i].Content)
	}

	ids := make([]int, 0, len(msgs))
	replyIDs := make([]int, 0)
	for i := range msgs {
		ids = append(ids, msgs[i].ID)
		if msgs[i].ReplyToID != nil && *msgs[i].ReplyToID > 0 {
			replyIDs = append(replyIDs, *msgs[i].ReplyToID)
		}
	}

	// 2. Reactions, grouped per message, each with the reacting user brief.
	if reactions, err := h.svc.ReactionsByMessageIDs(ids); err == nil && len(reactions) > 0 {
		ruids := make([]int, 0, len(reactions))
		for _, r := range reactions {
			ruids = append(ruids, r.UserID)
		}
		briefs := userclient.BriefMapByInt(ctx, h.users, ruids)
		byMsg := make(map[int][]chatModel.ChatReactionView, len(msgs))
		for _, r := range reactions {
			var u *patchModel.PatchUser
			if b := briefs[r.UserID]; b != nil {
				u = &patchModel.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar, AvatarImageHash: b.AvatarImageHash, Roles: b.Roles}
			}
			byMsg[r.ChatMessageID] = append(byMsg[r.ChatMessageID], chatModel.ChatReactionView{
				ID: r.ID, Emoji: r.Emoji, User: u,
			})
		}
		for i := range msgs {
			msgs[i].Reaction = byMsg[msgs[i].ID]
		}
	}

	// 3. Reply quotes: load the replied-to messages + their sender names.
	if len(replyIDs) > 0 {
		quoted, err := h.svc.MessagesByIDs(replyIDs)
		if err == nil && len(quoted) > 0 {
			qSenderIDs := make([]int, 0, len(quoted))
			for _, q := range quoted {
				qSenderIDs = append(qSenderIDs, q.SenderID)
			}
			qBriefs := userclient.BriefMapByInt(ctx, h.users, qSenderIDs)
			byID := make(map[int]chatModel.ChatQuoteView, len(quoted))
			for _, q := range quoted {
				name := "未知用户"
				if b := qBriefs[q.SenderID]; b != nil {
					name = b.Name
				}
				content := markdown.MustRender(q.Content)
				if q.Status == "DELETED" {
					content = "该消息已删除"
				}
				byID[q.ID] = chatModel.ChatQuoteView{ID: q.ID, SenderName: name, Content: content}
			}
			for i := range msgs {
				if msgs[i].ReplyToID != nil {
					if qv, ok := byID[*msgs[i].ReplyToID]; ok {
						v := qv
						msgs[i].QuoteMessage = &v
					}
				}
			}
		}
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
//
// Enriches each room with: the last-message text preview, and — for PRIVATE
// rooms — the peer's name/avatar (the room row itself has none; the link is
// "{lowUid}-{highUid}"). Briefs and last messages are batch-fetched, no N+1.
func (h *ChatHandler) ListRooms(c *fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	rooms, err := h.svc.ListRooms(user.UID)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	roomIDs := make([]int, 0, len(rooms))
	peerUIDs := make([]int, 0)
	peerByRoom := make(map[int]int, len(rooms))
	for i := range rooms {
		roomIDs = append(roomIDs, rooms[i].ID)
		if rooms[i].Type == "PRIVATE" {
			if a, b, ok := parsePrivateLink(rooms[i].Link); ok {
				peer := a
				if peer == user.UID {
					peer = b
				}
				peerByRoom[rooms[i].ID] = peer
				peerUIDs = append(peerUIDs, peer)
			}
		}
	}

	lastMsgs, _ := h.svc.LatestMessagePerRoom(roomIDs)
	peerBriefs := userclient.BriefMapByInt(c.Context(), h.users, peerUIDs)

	out := make([]chatModel.RoomSummaryView, 0, len(rooms))
	for i := range rooms {
		r := &rooms[i]
		v := chatModel.RoomSummaryView{
			ID:              r.ID,
			Link:            r.Link,
			Type:            r.Type,
			Name:            r.Name,
			Avatar:          r.Avatar,
			LastMessageTime: r.LastMessageTime,
			Created:         r.Created,
			Updated:         r.Updated,
		}
		if r.Type == "PRIVATE" {
			if b := peerBriefs[peerByRoom[r.ID]]; b != nil {
				v.Name = b.Name
				v.Avatar = b.Avatar
			} else {
				v.Name = "未知用户"
			}
		}
		if lm, ok := lastMsgs[r.ID]; ok {
			v.LastMessage = previewMessage(&lm)
		}
		out = append(out, v)
	}
	return response.OK(c, out)
}

// parsePrivateLink splits a "{a}-{b}" private-room link into the two uids.
func parsePrivateLink(link string) (a, b int, ok bool) {
	parts := strings.SplitN(link, "-", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	x, e1 := strconv.Atoi(parts[0])
	y, e2 := strconv.Atoi(parts[1])
	if e1 != nil || e2 != nil {
		return 0, 0, false
	}
	return x, y, true
}

// previewMessage returns a one-line preview of a message for the room list.
func previewMessage(m *chatModel.ChatMessage) string {
	if m.Status == "DELETED" {
		return "[消息已撤回]"
	}
	s := m.Content
	if s == "" && m.FileURL != "" {
		return "[图片]"
	}
	// Sticker messages are stored as `![sticker](url)`.
	if strings.HasPrefix(s, "![sticker](") {
		return "[贴纸]"
	}
	runes := []rune(s)
	if len(runes) > 30 {
		return string(runes[:30]) + "…"
	}
	return s
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

	msgs, err := h.svc.GetMessages(user.UID, link, q.After, q.Before, q.Limit)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	h.attachMessageSenders(c.Context(), msgs)
	h.enrichMessages(c.Context(), msgs)
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
	one := []chatModel.ChatMessage{*msg}
	h.enrichMessages(c.Context(), one)
	return response.OK(c, one[0])
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
