package upload

import (
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

// Handler exposes 5 HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// InitSmall POST /api/upload/small/init
func (h *Handler) InitSmall(c *fiber.Ctx) error {
	var req SmallInitRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	user := middleware.MustGetUser(c)
	privileged := middleware.HasAnyRole(c, "admin", "moderator")

	resp, err := h.svc.InitSmall(c.Context(), user.UID, privileged, req)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, resp)
}

// CompleteSmall POST /api/upload/small/complete
func (h *Handler) CompleteSmall(c *fiber.Ctx) error {
	var req SmallCompleteRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	user := middleware.MustGetUser(c)
	privileged := middleware.HasAnyRole(c, "admin", "moderator")

	resp, err := h.svc.CompleteSmall(c.Context(), user.UID, privileged, req)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, resp)
}

// InitMultipart POST /api/upload/multipart/init
func (h *Handler) InitMultipart(c *fiber.Ctx) error {
	var req MultipartInitRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	user := middleware.MustGetUser(c)
	privileged := middleware.HasAnyRole(c, "admin", "moderator")

	resp, err := h.svc.InitMultipart(c.Context(), user.UID, privileged, req)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, resp)
}

// CompleteMultipart POST /api/upload/multipart/complete
func (h *Handler) CompleteMultipart(c *fiber.Ctx) error {
	var req MultipartCompleteRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	user := middleware.MustGetUser(c)
	privileged := middleware.HasAnyRole(c, "admin", "moderator")

	resp, err := h.svc.CompleteMultipart(c.Context(), user.UID, privileged, req)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, resp)
}

// AbortMultipart POST /api/upload/multipart/abort
func (h *Handler) AbortMultipart(c *fiber.Ctx) error {
	var req MultipartAbortRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	_ = middleware.MustGetUser(c)

	if err := h.svc.AbortMultipart(c.Context(), req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OKMessage(c, "已放弃上传")
}
