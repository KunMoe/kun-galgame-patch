package handler

import (
	"context"
	"regexp"
	"strconv"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/message/dto"
	"kun-galgame-patch-api/internal/message/service"
	"kun-galgame-patch-api/internal/middleware"
	patchModel "kun-galgame-patch-api/internal/patch/model"
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
	wiki    *galgameClient.Client
}

func New(svc *service.MessageService, users *userclient.Client, wiki *galgameClient.Client) *MessageHandler {
	return &MessageHandler{service: svc, users: users, wiki: wiki}
}

// galgameNameTypes are the message types whose Content bakes a single-language
// (zh-cn-first) game name; for these we attach the multilingual name so the
// frontend can render it in the viewer's preferred title language.
var galgameNameTypes = map[string]bool{
	"favorite":         true,
	"favoriteResource": true,
	"likeResource":     true,
}

var patchLinkRe = regexp.MustCompile(`^/patch/(\d+)`)

// attachGalgameNames batch-resolves the multilingual game name (from each
// game-scoped message's /patch/<id> link) and stamps msg.GalgameName, so the
// frontend renders the name in the viewer's preferred title language instead of
// the baked Content. Best-effort: unresolved messages keep GalgameName=nil and
// the frontend falls back to Content.
func (h *MessageHandler) attachGalgameNames(ctx context.Context, msgs []userModel.UserMessage) {
	if h.wiki == nil || len(msgs) == 0 {
		return
	}
	idByIdx := make(map[int]int, len(msgs))
	idSet := make(map[int]struct{})
	for i := range msgs {
		if !galgameNameTypes[msgs[i].Type] {
			continue
		}
		m := patchLinkRe.FindStringSubmatch(msgs[i].Link)
		if m == nil {
			continue
		}
		id, _ := strconv.Atoi(m[1])
		if id <= 0 {
			continue
		}
		idByIdx[i] = id
		idSet[id] = struct{}{}
	}
	if len(idSet) == 0 {
		return
	}
	ids := make([]int, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	briefs, err := h.wiki.GalgameBatch(ctx, ids, "")
	if err != nil {
		return
	}
	nameByID := make(map[int]map[string]string, len(briefs))
	for i := range briefs {
		b := &briefs[i]
		nameByID[b.ID] = map[string]string{
			"en-us": b.NameEnUs,
			"ja-jp": b.NameJaJp,
			"zh-cn": b.NameZhCn,
			"zh-tw": b.NameZhTw,
		}
	}
	for idx, id := range idByIdx {
		if n := nameByID[id]; n != nil {
			msgs[idx].GalgameName = n
		}
	}
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
	h.attachGalgameNames(c.Context(), messages)
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
	h.attachGalgameNames(c.Context(), messages)
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
