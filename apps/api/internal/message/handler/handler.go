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

	"github.com/gofiber/fiber/v3"
)

type MessageHandler struct {
	service *service.MessageService
	users   *userclient.Client
	galgame *galgameClient.Client
}

func New(svc *service.MessageService, users *userclient.Client, galgame *galgameClient.Client) *MessageHandler {
	return &MessageHandler{service: svc, users: users, galgame: galgame}
}

var galgameNameTypes = map[string]bool{
	"favorite":         true,
	"favoriteResource": true,
	"likeResource":     true,
}

// The page moved to /galgame/<id> and every stored link was rewritten with it
// (migration 037), so a regexp still reading /patch/ matched nothing and the
// 84,307 favourite and like notices carrying such a link silently fell back to
// the name frozen into content at write time -- readable, but never translated
// and never refreshed. favoriteResource links are /resource/<id> and have never
// matched here; that type reads as a whole sentence without a name.
var galgameLinkRe = regexp.MustCompile(`^/galgame/(\d+)`)

func (h *MessageHandler) attachGalgameNames(ctx context.Context, msgs []userModel.UserMessage) {
	if h.galgame == nil || len(msgs) == 0 {
		return
	}
	idByIdx := make(map[int]int, len(msgs))
	idSet := make(map[int]struct{})
	for i := range msgs {
		if !galgameNameTypes[msgs[i].Type] {
			continue
		}
		m := galgameLinkRe.FindStringSubmatch(msgs[i].Link)
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
	briefs, err := h.galgame.GalgameBatch(ctx, ids, "")
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
				SiteRoles:       b.SiteRoles,
			}
		}
	}
}

func (h *MessageHandler) GetMessages(c fiber.Ctx) error {
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

func (h *MessageHandler) GetAllMessages(c fiber.Ctx) error {
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

func (h *MessageHandler) GetUnreadTypes(c fiber.Ctx) error {
	user := middleware.MustGetUser(c)
	types, err := h.service.GetUnreadTypes(user.ID)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.OK(c, types)
}

func (h *MessageHandler) MarkAsRead(c fiber.Ctx) error {
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
