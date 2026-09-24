package handler

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	"kun-galgame-patch-api/internal/chat/dto"
	chatModel "kun-galgame-patch-api/internal/chat/model"
	"kun-galgame-patch-api/internal/chat/service"
	"kun-galgame-patch-api/internal/infrastructure/markdown"
	"kun-galgame-patch-api/internal/middleware"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/userclient"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

type ChatHandler struct {
	svc   *service.ChatService
	users *userclient.Client
}

func New(svc *service.ChatService, users *userclient.Client) *ChatHandler {
	return &ChatHandler{svc: svc, users: users}
}

func (h *ChatHandler) attachMessageSenders(ctx context.Context, msgs []chatModel.ChatMessage) {
	uids := make([]int, 0, len(msgs))
	for _, m := range msgs {
		uids = append(uids, m.SenderID)
	}
	briefs := userclient.BriefMapByInt(ctx, h.users, uids)
	for i := range msgs {
		if b := briefs[msgs[i].SenderID]; b != nil {
			msgs[i].Sender = patchModel.NewPatchUser(b)
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
			ms[i].User = patchModel.NewPatchUser(b)
		}
	}
}

func (h *ChatHandler) attachOneSender(ctx context.Context, msg *chatModel.ChatMessage) {
	if msg == nil || msg.SenderID == 0 {
		return
	}
	if b := userclient.BriefMapByInt(ctx, h.users, []int{msg.SenderID})[msg.SenderID]; b != nil {
		msg.Sender = patchModel.NewPatchUser(b)
	}
}

func (h *ChatHandler) enrichMessages(ctx context.Context, msgs []chatModel.ChatMessage) {
	if len(msgs) == 0 {
		return
	}

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

	if reactions, err := h.svc.ReactionsByMessageIDs(ids); err == nil && len(reactions) > 0 {
		ruids := make([]int, 0, len(reactions))
		for _, r := range reactions {
			ruids = append(ruids, r.UserID)
		}
		briefs := userclient.BriefMapByInt(ctx, h.users, ruids)
		byMsg := make(map[int][]chatModel.ChatReactionView, len(msgs))
		for _, r := range reactions {
			byMsg[r.ChatMessageID] = append(byMsg[r.ChatMessageID], chatModel.ChatReactionView{
				ID: r.ID, Emoji: r.Emoji, User: patchModel.NewPatchUser(briefs[r.UserID]),
			})
		}
		for i := range msgs {
			msgs[i].Reaction = byMsg[msgs[i].ID]
		}
	}

	if len(replyIDs) > 0 {
		quoted, err := h.svc.MessagesByIDs(replyIDs)
		if err == nil && len(quoted) > 0 {
			roomID := msgs[0].ChatRoomID
			qSenderIDs := make([]int, 0, len(quoted))
			for _, q := range quoted {
				if q.ChatRoomID != roomID {
					continue
				}
				qSenderIDs = append(qSenderIDs, q.SenderID)
			}
			qBriefs := userclient.BriefMapByInt(ctx, h.users, qSenderIDs)
			byID := make(map[int]chatModel.ChatQuoteView, len(quoted))
			for _, q := range quoted {
				if q.ChatRoomID != roomID {
					continue
				}
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

func getMessageIDParam(c fiber.Ctx) (int, error) {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id < 1 {
		return 0, errors.ErrBadRequest("invalid message id")
	}
	return id, nil
}

func (h *ChatHandler) ListRooms(c fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	rooms, err := h.svc.ListRooms(user.ID)
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
				if peer == user.ID {
					peer = b
				}
				peerByRoom[rooms[i].ID] = peer
				peerUIDs = append(peerUIDs, peer)
			}
		}
	}

	lastMsgs, lmErr := h.svc.LatestMessagePerRoom(roomIDs)
	if lmErr != nil {
		slog.Warn("LatestMessagePerRoom failed; room list will omit previews", "error", lmErr)
	}
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

func previewMessage(m *chatModel.ChatMessage) string {
	if m.Status == "DELETED" {
		return "[消息已撤回]"
	}
	s := m.Content
	if s == "" && m.FileURL != "" {
		return "[图片]"
	}
	if strings.HasPrefix(s, "![sticker](") {
		return "[贴纸]"
	}
	runes := []rune(s)
	if len(runes) > 30 {
		return string(runes[:30]) + "…"
	}
	return s
}

func (h *ChatHandler) CreateRoom(c fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	if !middleware.IsAdmin(c) {
		return response.Error(c, errors.ErrForbidden())
	}
	var req dto.CreateRoomRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	room, err := h.svc.CreateGroupRoom(user.ID, req.Name, req.Avatar)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, room)
}

func (h *ChatHandler) GetRoomDetail(c fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	link := c.Params("link")
	detail, err := h.svc.GetRoomDetail(user.ID, link)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	h.attachMemberUsers(c.Context(), detail.Member)
	return response.OK(c, detail)
}

func (h *ChatHandler) JoinRoom(c fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	var req dto.JoinRoomRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	room, err := h.svc.JoinRoomByLink(user.ID, req.Link)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, room)
}

func (h *ChatHandler) StartPrivate(c fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	var req dto.StartPrivateChatRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if req.PeerUID == user.ID {
		return response.Error(c, errors.ErrBadRequest("不能给自己发消息"))
	}
	room, err := h.svc.StartPrivateChat(user.ID, req.PeerUID)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, room)
}

func (h *ChatHandler) ListMessages(c fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	link := c.Params("link")

	var q dto.ListMessagesQuery
	if err := utils.ParseQueryAndValidate(c, &q); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if q.Limit == 0 {
		q.Limit = 30
	}

	var msgs []chatModel.ChatMessage
	var err error
	if ids := parseCSVInts(q.IDs); len(ids) > 0 {
		msgs, err = h.svc.GetMessagesByIDsInRoom(user.ID, link, ids)
	} else {
		msgs, err = h.svc.GetMessages(user.ID, link, q.After, q.Before, q.Limit)
	}
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	h.attachMessageSenders(c.Context(), msgs)
	h.enrichMessages(c.Context(), msgs)
	return response.OK(c, msgs)
}

func parseCSVInts(s string) []int {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		if n, e := strconv.Atoi(strings.TrimSpace(p)); e == nil && n > 0 {
			out = append(out, n)
			if len(out) >= 200 {
				break
			}
		}
	}
	return out
}

func (h *ChatHandler) CreateMessage(c fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	link := c.Params("link")

	var req dto.CreateMessageRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	msg, err := h.svc.CreateMessage(user.ID, link, req.Content, req.FileURL, req.ReplyToID)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	h.attachOneSender(c.Context(), msg)
	one := []chatModel.ChatMessage{*msg}
	h.enrichMessages(c.Context(), one)
	return response.OK(c, one[0])
}

func (h *ChatHandler) UpdateMessage(c fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	id, err := getMessageIDParam(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.UpdateMessageRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	if err := h.svc.UpdateMessage(user.ID, id, req.Content); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OKMessage(c, "消息已编辑")
}

func (h *ChatHandler) DeleteMessage(c fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	id, err := getMessageIDParam(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	isPrivileged := middleware.IsModerator(c)
	if err := h.svc.DeleteMessage(user.ID, isPrivileged, id); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OKMessage(c, "消息已删除")
}

func (h *ChatHandler) ToggleReaction(c fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	id, err := getMessageIDParam(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.ReactionRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	added, err := h.svc.ToggleReaction(user.ID, id, req.Emoji)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, map[string]bool{"added": added})
}

func (h *ChatHandler) MarkSeen(c fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	link := c.Params("link")

	var req dto.SeenRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if err := h.svc.MarkSeen(user.ID, link, req.MessageIDs); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OKMessage(c, "已标记")
}
