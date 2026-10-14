package rating

import (
	"strconv"

	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

type LibraryHandler struct {
	library *Library
}

func NewLibraryHandler(library *Library) *LibraryHandler {
	return &LibraryHandler{library: library}
}

type libraryQuery struct {
	State  string `query:"state" validate:"omitempty,oneof=wish doing done on_hold dropped"`
	Cursor string `query:"cursor" validate:"omitempty,max=512"`
}

type privacyBody struct {
	PlayRecords string `json:"play_records"`
}

func privacyOf(private bool) privacyBody {
	if private {
		return privacyBody{PlayRecords: "private"}
	}
	return privacyBody{PlayRecords: "public"}
}

func (h *LibraryHandler) Get(c fiber.Ctx) error {
	uid, err := strconv.Atoi(c.Params("id"))
	if err != nil || uid <= 0 {
		return response.Error(c, errors.ErrBadRequest("无效的用户 ID"))
	}
	var q libraryQuery
	if err := utils.ParseQueryAndValidate(c, &q); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	page, err := h.library.Page(c.Context(), LibraryQuery{
		UserID:       uid,
		ViewerID:     middleware.GetUserID(c),
		AccessToken:  middleware.GetAccessToken(c),
		State:        q.State,
		Cursor:       q.Cursor,
		ContentLimit: utils.ContentLimitForListBrowse(c),
	})
	if err != nil {
		return fail(c, err)
	}
	return response.OK(c, page)
}

func (h *LibraryHandler) GetPrivacy(c fiber.Ctx) error {
	private, err := h.library.Private(c.Context(), middleware.GetAccessToken(c))
	if err != nil {
		return fail(c, err)
	}
	return response.OK(c, privacyOf(private))
}

func (h *LibraryHandler) SetPrivacy(c fiber.Ctx) error {
	var req privacyBody
	if err := c.Bind().Body(&req); err != nil {
		return response.Error(c, errors.ErrBadRequest("请求参数错误"))
	}
	if req.PlayRecords != "public" && req.PlayRecords != "private" {
		return response.Error(c, errors.ErrValidation("未知的公开范围"))
	}
	private, err := h.library.SetPrivate(c.Context(), middleware.GetAccessToken(c), req.PlayRecords == "private")
	if err != nil {
		return fail(c, err)
	}
	return response.OK(c, privacyOf(private))
}
