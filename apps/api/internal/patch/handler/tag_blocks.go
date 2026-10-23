package handler

import (
	"strconv"

	"kun-galgame-patch-api/pkg/catalogv2"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/upstream"

	"github.com/gofiber/fiber/v3"
)

type tagBlockRequest struct {
	Mode string `json:"mode"`
}

func tagIDParam(c fiber.Ctx) (int64, *errors.AppError) {
	id, err := strconv.ParseInt(c.Params("tagId"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.ErrBadRequest("无效的标签 ID")
	}
	return id, nil
}

func tagBlockErr(c fiber.Ctx, err error, notFound string) error {
	if upstream.KindOf(err) == upstream.Rejected && !catalogv2.ReauthRequired(err) {
		return response.Error(c, errors.ErrValidation("最多屏蔽 100 个标签"))
	}
	return catalogErr(c, err, notFound)
}

func (h *PatchHandler) MyTagBlocks(c fiber.Ctx) error {
	token, appErr := catalogUserToken(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	blocks, err := h.galgame.V2().MyTagBlocks(c.Context(), token)
	if err != nil {
		return tagBlockErr(c, err, "")
	}
	return response.OK(c, fiber.Map{"blocks": blocks})
}

func (h *PatchHandler) MyTagBlock(c fiber.Ctx) error {
	token, appErr := catalogUserToken(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	tagID, idErr := tagIDParam(c)
	if idErr != nil {
		return response.Error(c, idErr)
	}
	block, err := h.galgame.V2().MyTagBlock(c.Context(), token, tagID)
	if err != nil {
		return tagBlockErr(c, err, "未屏蔽该标签")
	}
	return response.OK(c, block)
}

func (h *PatchHandler) PutTagBlock(c fiber.Ctx) error {
	token, appErr := catalogUserToken(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	tagID, idErr := tagIDParam(c)
	if idErr != nil {
		return response.Error(c, idErr)
	}
	var req tagBlockRequest
	if len(c.Body()) > 0 {
		if err := c.Bind().JSON(&req); err != nil {
			return response.Error(c, errors.ErrBadRequest("请求参数错误"))
		}
	}
	switch req.Mode {
	case "":
		req.Mode = catalogv2.TagBlockTree
	case catalogv2.TagBlockTree, catalogv2.TagBlockExact:
	default:
		return response.Error(c, errors.ErrBadRequest("未知的屏蔽方式"))
	}
	block, err := h.galgame.V2().PutTagBlock(c.Context(), token, tagID, req.Mode)
	if err != nil {
		return tagBlockErr(c, err, "标签不存在")
	}
	return response.OK(c, block)
}

func (h *PatchHandler) DeleteTagBlock(c fiber.Ctx) error {
	token, appErr := catalogUserToken(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	tagID, idErr := tagIDParam(c)
	if idErr != nil {
		return response.Error(c, idErr)
	}
	if err := h.galgame.V2().DeleteTagBlock(c.Context(), token, tagID); err != nil {
		return tagBlockErr(c, err, "未屏蔽该标签")
	}
	return response.OK(c, fiber.Map{"deleted": true})
}
