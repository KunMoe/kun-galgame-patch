// Package handler exposes the /about endpoints.
package handler

import (
	"errors"
	"os"

	"kun-galgame-patch-api/internal/about/service"
	apperrors "kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type AboutHandler struct {
	svc *service.AboutService
}

func New(svc *service.AboutService) *AboutHandler {
	return &AboutHandler{svc: svc}
}

// ListPosts GET /api/v1/about/posts
//
// Returns both the flat post-metadata list (for the /about index card grid)
// and the directory tree (for the doc-detail sidebar) in a single response.
func (h *AboutHandler) ListPosts(c *fiber.Ctx) error {
	out, err := h.svc.List()
	if err != nil {
		return response.Error(c, apperrors.ErrInternal(""))
	}
	return response.OK(c, out)
}

// GetPost GET /api/v1/about/post?slug=<dir>/<name>
//
// Returns the rendered HTML, TOC (h1-h3 with CJK-friendly anchor IDs),
// frontmatter and the prev/next post metadata.
func (h *AboutHandler) GetPost(c *fiber.Ctx) error {
	slug := c.Query("slug")
	if slug == "" {
		return response.Error(c, apperrors.ErrBadRequest("slug 不能为空"))
	}

	out, err := h.svc.GetPost(slug)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return response.Error(c, apperrors.ErrNotFound("文章不存在"))
		}
		return response.Error(c, apperrors.ErrInternal(""))
	}
	return response.OK(c, out)
}
