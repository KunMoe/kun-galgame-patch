// Package handler exposes the public /blog and admin /admin/blog endpoints.
package handler

import (
	"errors"
	"strconv"

	"kun-galgame-patch-api/internal/blog/dto"
	"kun-galgame-patch-api/internal/blog/service"
	"kun-galgame-patch-api/internal/middleware"
	apperrors "kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type BlogHandler struct {
	svc *service.BlogService
}

func New(svc *service.BlogService) *BlogHandler {
	return &BlogHandler{svc: svc}
}

func blogID(c *fiber.Ctx) (int, *apperrors.AppError) {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id < 1 {
		return 0, apperrors.ErrBadRequest("invalid blog id")
	}
	return id, nil
}

// ===== Public =====

// ListBlogs GET /blog?page=&limit= — published posts only.
func (h *BlogHandler) ListBlogs(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "12"))
	items, total, err := h.svc.ListPublic(c.Context(), page, limit)
	if err != nil {
		return response.Error(c, apperrors.ErrInternal(""))
	}
	return response.Paginated(c, items, total)
}

// GetBlog GET /blog/:id — published only (draft/missing → 404).
func (h *BlogHandler) GetBlog(c *fiber.Ctx) error {
	id, appErr := blogID(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	detail, err := h.svc.GetPublic(c.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.Error(c, apperrors.ErrNotFound("博客不存在"))
		}
		return response.Error(c, apperrors.ErrInternal(""))
	}
	return response.OK(c, detail)
}

// IncrementView PUT /blog/:id/view — best-effort counter bump.
func (h *BlogHandler) IncrementView(c *fiber.Ctx) error {
	id, appErr := blogID(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	_ = h.svc.IncrementView(id)
	return response.OKMessage(c, "OK")
}

// ===== Admin (moderator+) =====

// AdminListBlogs GET /admin/blog — all posts incl. drafts.
func (h *BlogHandler) AdminListBlogs(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "12"))
	items, total, err := h.svc.ListAdmin(c.Context(), page, limit)
	if err != nil {
		return response.Error(c, apperrors.ErrInternal(""))
	}
	return response.Paginated(c, items, total)
}

// AdminGetBlog GET /admin/blog/:id — raw post for the editor (any status).
func (h *BlogHandler) AdminGetBlog(c *fiber.Ctx) error {
	id, appErr := blogID(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	out, err := h.svc.GetForEdit(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.Error(c, apperrors.ErrNotFound("博客不存在"))
		}
		return response.Error(c, apperrors.ErrInternal(""))
	}
	return response.OK(c, out)
}

// CreateBlog POST /admin/blog
func (h *BlogHandler) CreateBlog(c *fiber.Ctx) error {
	var req dto.BlogCreateRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, apperrors.ErrBadRequest(err.Error()))
	}
	user := middleware.MustGetUser(c)
	blog, err := h.svc.Create(user.ID, req)
	if err != nil {
		return response.Error(c, apperrors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, blog)
}

// UpdateBlog PUT /admin/blog/:id
func (h *BlogHandler) UpdateBlog(c *fiber.Ctx) error {
	id, appErr := blogID(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req dto.BlogUpdateRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, apperrors.ErrBadRequest(err.Error()))
	}
	blog, err := h.svc.Update(id, req)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.Error(c, apperrors.ErrNotFound("博客不存在"))
		}
		return response.Error(c, apperrors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, blog)
}

// DeleteBlog DELETE /admin/blog/:id
func (h *BlogHandler) DeleteBlog(c *fiber.Ctx) error {
	id, appErr := blogID(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	if err := h.svc.Delete(id); err != nil {
		return response.Error(c, apperrors.ErrInternal(""))
	}
	return response.OKMessage(c, "删除成功")
}
