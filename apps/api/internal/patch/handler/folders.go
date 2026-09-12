package handler

import (
	"strconv"
	"strings"

	"kun-galgame-patch-api/internal/galgame/enricher"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/catalogv2"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

const (
	folderNameMax        = 60
	folderDescriptionMax = 500
)

type folderWriteRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Visibility  *string `json:"visibility"`
}

type setFoldersRequest struct {
	FolderIDs []int64 `json:"folder_ids"`
}

func folderIDParam(c fiber.Ctx) (int64, *errors.AppError) {
	id, err := strconv.ParseInt(c.Params("folderId"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.ErrBadRequest("无效的收藏夹 ID")
	}
	return id, nil
}

func validFolderWrite(in folderWriteRequest, nameRequired bool) *errors.AppError {
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return errors.ErrValidation("收藏夹名称不能为空")
		}
		if len([]rune(name)) > folderNameMax {
			return errors.ErrValidation("收藏夹名称过长")
		}
		*in.Name = name
	} else if nameRequired {
		return errors.ErrValidation("收藏夹名称不能为空")
	}
	if in.Description != nil && len([]rune(*in.Description)) > folderDescriptionMax {
		return errors.ErrValidation("收藏夹简介过长")
	}
	if in.Visibility != nil &&
		*in.Visibility != catalogv2.FolderVisibilityPublic &&
		*in.Visibility != catalogv2.FolderVisibilityPrivate {
		return errors.ErrValidation("未知的可见性")
	}
	return nil
}

func (h *PatchHandler) MyFolders(c fiber.Ctx) error {
	token, appErr := catalogUserToken(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	folders, err := h.service.MyFolders(c.Context(), token)
	if err != nil {
		return catalogErr(c, err, "读取收藏夹失败")
	}
	return response.OK(c, fiber.Map{"folders": folders})
}

func (h *PatchHandler) UserFolders(c fiber.Ctx) error {
	ownerID, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	// Somebody else's shelf shows only what they published. Their own shelf is
	// MyFolders, which is the only face that knows about private folders.
	if user := middleware.GetUser(c); user != nil && user.ID == ownerID {
		if token := middleware.GetAccessToken(c); token != "" {
			folders, mErr := h.service.MyFolders(c.Context(), token)
			if mErr != nil {
				return catalogErr(c, mErr, "读取收藏夹失败")
			}
			return response.OK(c, fiber.Map{"folders": folders})
		}
	}
	folders, pErr := h.service.PublicFolders(c.Context(), ownerID)
	if pErr != nil {
		return catalogErr(c, pErr, "读取收藏夹失败")
	}
	return response.OK(c, fiber.Map{"folders": folders})
}

func (h *PatchHandler) CreateFolder(c fiber.Ctx) error {
	token, appErr := catalogUserToken(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req folderWriteRequest
	if err := c.Bind().Body(&req); err != nil {
		return response.Error(c, errors.ErrBadRequest("请求参数错误"))
	}
	if vErr := validFolderWrite(req, true); vErr != nil {
		return response.Error(c, vErr)
	}
	visibility := catalogv2.FolderVisibilityPublic
	if req.Visibility != nil {
		visibility = *req.Visibility
	}
	description := ""
	if req.Description != nil {
		description = *req.Description
	}
	folder, err := h.service.CreateFolder(c.Context(), token, *req.Name, description, visibility)
	if err != nil {
		return catalogErr(c, err, "创建收藏夹失败")
	}
	return response.OK(c, folder)
}

func (h *PatchHandler) UpdateFolder(c fiber.Ctx) error {
	token, appErr := catalogUserToken(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	folderID, idErr := folderIDParam(c)
	if idErr != nil {
		return response.Error(c, idErr)
	}
	var req folderWriteRequest
	if err := c.Bind().Body(&req); err != nil {
		return response.Error(c, errors.ErrBadRequest("请求参数错误"))
	}
	if vErr := validFolderWrite(req, false); vErr != nil {
		return response.Error(c, vErr)
	}
	if req.Name == nil && req.Description == nil && req.Visibility == nil {
		return response.Error(c, errors.ErrBadRequest("没有要修改的内容"))
	}
	folder, err := h.service.UpdateFolder(c.Context(), token, folderID, catalogv2.FolderWrite{
		Name: req.Name, Description: req.Description, Visibility: req.Visibility,
	})
	if err != nil {
		return catalogErr(c, err, "更新收藏夹失败")
	}
	return response.OK(c, folder)
}

func (h *PatchHandler) DeleteFolder(c fiber.Ctx) error {
	token, appErr := catalogUserToken(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	folderID, idErr := folderIDParam(c)
	if idErr != nil {
		return response.Error(c, idErr)
	}
	if err := h.service.DeleteFolder(c.Context(), token, folderID); err != nil {
		return catalogErr(c, err, "删除收藏夹失败")
	}
	return response.OK(c, fiber.Map{"deleted": true})
}

func (h *PatchHandler) FolderDetail(c fiber.Ctx) error {
	folderID, idErr := folderIDParam(c)
	if idErr != nil {
		return response.Error(c, idErr)
	}
	token := middleware.GetAccessToken(c)
	// Try the owner's lane first when there is a token: it is the only one that
	// answers a private folder, and it 404s just the same when the reader is
	// not the owner, so nothing leaks by attempting it.
	folder, patches, err := h.service.FolderPatches(c.Context(), token, folderID, token != "")
	if err != nil && token != "" {
		folder, patches, err = h.service.FolderPatches(c.Context(), "", folderID, false)
	}
	if err != nil {
		return catalogErr(c, err, "读取收藏夹失败")
	}
	// The rows have to leave here as cards. Sent raw they carry no name, no
	// cover and no count, and the shelf drew ten framed placeholders reading
	// 暂无补丁 that still linked to the right game — the card reads count.resource,
	// which a bare patch row does not have at all.
	cl := utils.ContentLimitForListBrowse(c)
	cards := enricher.EnrichPatchCards(c.Context(), h.galgame, h.users, patches, cl)
	return response.OK(c, fiber.Map{"folder": folder, "patches": cards})
}

func (h *PatchHandler) FoldersForPatch(c fiber.Ctx) error {
	token, appErr := catalogUserToken(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	folders, fErr := h.service.FoldersForPatch(c.Context(), token, id)
	if fErr != nil {
		return catalogErr(c, fErr, "读取收藏夹失败")
	}
	return response.OK(c, fiber.Map{"folders": folders})
}

func (h *PatchHandler) SetPatchFolders(c fiber.Ctx) error {
	token, appErr := catalogUserToken(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	var req setFoldersRequest
	if bErr := c.Bind().Body(&req); bErr != nil {
		return response.Error(c, errors.ErrBadRequest("请求参数错误"))
	}
	if len(req.FolderIDs) > catalogv2.FoldersPerUserMax {
		return response.Error(c, errors.ErrBadRequest("收藏夹数量超出上限"))
	}
	if sErr := h.service.SetPatchFolders(c.Context(), token, id, req.FolderIDs); sErr != nil {
		return catalogErr(c, sErr, "更新收藏失败")
	}
	return response.OK(c, fiber.Map{"favorited": len(req.FolderIDs) > 0})
}
