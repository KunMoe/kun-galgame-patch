// Package handler exposes the public /doc and admin /admin/doc endpoints.
package handler

import (
	"errors"
	"strconv"

	"kun-galgame-patch-api/internal/doc/dto"
	"kun-galgame-patch-api/internal/doc/service"
	"kun-galgame-patch-api/internal/middleware"
	apperrors "kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type DocHandler struct {
	svc *service.DocService
}

func New(svc *service.DocService) *DocHandler {
	return &DocHandler{svc: svc}
}

func docID(c *fiber.Ctx) (int, *apperrors.AppError) {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id < 1 {
		return 0, apperrors.ErrBadRequest("invalid doc id")
	}
	return id, nil
}

// ===== Public =====

// ListPosts GET /doc/posts — published flat list + category tree.
func (h *DocHandler) ListPosts(c *fiber.Ctx) error {
	out, err := h.svc.List()
	if err != nil {
		return response.Error(c, apperrors.ErrInternal(""))
	}
	return response.OK(c, out)
}

// ListPinnedPosts GET /doc/pinned — pinned published docs for the home
// carousel, newest first by display date.
func (h *DocHandler) ListPinnedPosts(c *fiber.Ctx) error {
	items, err := h.svc.ListPinned()
	if err != nil {
		return response.Error(c, apperrors.ErrInternal(""))
	}
	return response.OK(c, items)
}

// GetPost GET /doc/post?slug=<category>/<name> — published only.
func (h *DocHandler) GetPost(c *fiber.Ctx) error {
	slug := c.Query("slug")
	if slug == "" {
		return response.Error(c, apperrors.ErrBadRequest("slug 不能为空"))
	}
	out, err := h.svc.GetPost(slug)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.Error(c, apperrors.ErrNotFound("文档不存在"))
		}
		return response.Error(c, apperrors.ErrInternal(""))
	}
	return response.OK(c, out)
}

// IncrementView PUT /doc/view?slug=... — best-effort view counter.
func (h *DocHandler) IncrementView(c *fiber.Ctx) error {
	slug := c.Query("slug")
	if slug != "" {
		h.svc.IncrementViewBySlug(slug)
	}
	return response.OKMessage(c, "OK")
}

// ===== Admin (moderator+) =====

// AdminListPosts GET /admin/doc — all docs incl. drafts.
func (h *DocHandler) AdminListPosts(c *fiber.Ctx) error {
	items, err := h.svc.ListAdmin()
	if err != nil {
		return response.Error(c, apperrors.ErrInternal(""))
	}
	return response.OK(c, items)
}

// AdminGetPost GET /admin/doc/:id — raw doc for the editor.
func (h *DocHandler) AdminGetPost(c *fiber.Ctx) error {
	id, appErr := docID(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	out, err := h.svc.GetForEdit(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.Error(c, apperrors.ErrNotFound("文档不存在"))
		}
		return response.Error(c, apperrors.ErrInternal(""))
	}
	return response.OK(c, out)
}

// CreatePost POST /admin/doc
func (h *DocHandler) CreatePost(c *fiber.Ctx) error {
	var req dto.DocCreateRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, apperrors.ErrBadRequest(err.Error()))
	}
	user := middleware.MustGetUser(c)
	doc, err := h.svc.Create(c.Context(), user.ID, req)
	if err != nil {
		return response.Error(c, apperrors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, doc)
}

// UpdatePost PUT /admin/doc/:id
func (h *DocHandler) UpdatePost(c *fiber.Ctx) error {
	id, appErr := docID(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req dto.DocUpdateRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, apperrors.ErrBadRequest(err.Error()))
	}
	doc, err := h.svc.Update(id, req)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.Error(c, apperrors.ErrNotFound("文档不存在"))
		}
		return response.Error(c, apperrors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, doc)
}

// DeletePost DELETE /admin/doc/:id
func (h *DocHandler) DeletePost(c *fiber.Ctx) error {
	id, appErr := docID(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	if err := h.svc.Delete(id); err != nil {
		return response.Error(c, apperrors.ErrInternal(""))
	}
	return response.OKMessage(c, "删除成功")
}
