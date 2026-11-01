package permission

import (
	stderrors "errors"
	"log/slog"
	"strconv"
	"strings"

	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/perm"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func caller(c fiber.Ctx) Caller {
	return Caller{ID: middleware.GetUserID(c), Roles: middleware.EffectiveRoles(c)}
}

func fail(c fiber.Ctx, op string, err error) error {
	var ve *ValidationError
	switch {
	case stderrors.As(err, &ve):
		return response.Error(c, errors.ErrValidation(ve.Error()))
	case stderrors.Is(err, ErrUserNotFound):
		return response.Error(c, errors.ErrNotFound("用户不存在"))
	case stderrors.Is(err, ErrStale):
		return response.Error(c, errors.ErrConflict("权限已被其他管理员修改，请刷新后重新调整"))
	}
	slog.Error("permission: "+op+" failed", "error", err)
	return response.Error(c, errors.ErrInternal(""))
}

func (h *Handler) Mine(c fiber.Ctx) error {
	return response.OK(c, fiber.Map{"permissions": middleware.Permissions(c)})
}

func (h *Handler) GetMatrix(c fiber.Ctx) error {
	m, err := h.svc.Matrix(c.Context(), caller(c))
	if err != nil {
		return fail(c, "matrix", err)
	}
	return response.OK(c, m)
}

func (h *Handler) UpdateRoles(c fiber.Ctx) error {
	var req struct {
		Version int          `json:"version" validate:"min=0"`
		Reason  string       `json:"reason" validate:"max=200"`
		Changes []RoleChange `json:"changes" validate:"required,min=1,max=4"`
	}
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	m, err := h.svc.ApplyRoles(c.Context(), caller(c), req.Version, strings.TrimSpace(req.Reason), req.Changes)
	if err != nil {
		return fail(c, "apply roles", err)
	}
	return response.OK(c, m)
}

func userIDParam(c fiber.Ctx) (int, error) {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id < 1 {
		return 0, errors.ErrBadRequest("invalid ID")
	}
	return id, nil
}

func (h *Handler) GetUser(c fiber.Ctx) error {
	uid, err := userIDParam(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	v, err := h.svc.User(c.Context(), caller(c), uid)
	if err != nil {
		return fail(c, "get user", err)
	}
	return response.OK(c, v)
}

func (h *Handler) ReplaceUser(c fiber.Ctx) error {
	uid, err := userIDParam(c)
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	var req struct {
		Version   int             `json:"version" validate:"min=0"`
		Reason    string          `json:"reason" validate:"max=200"`
		Overrides []perm.Override `json:"overrides" validate:"max=64"`
	}
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	v, err := h.svc.ReplaceUser(c.Context(), caller(c), uid, req.Version, strings.TrimSpace(req.Reason), req.Overrides)
	if err != nil {
		return fail(c, "replace user", err)
	}
	return response.OK(c, v)
}

func (h *Handler) GetOverridden(c fiber.Ctx) error {
	items, err := h.svc.Overridden(c.Context())
	if err != nil {
		return fail(c, "overridden", err)
	}
	return response.OK(c, items)
}

func (h *Handler) GetChanges(c fiber.Ctx) error {
	var req struct {
		Page   int    `query:"page" validate:"required,min=1"`
		Limit  int    `query:"limit" validate:"required,min=1,max=50"`
		Role   string `query:"role" validate:"omitempty,max=16"`
		UserID int    `query:"user_id" validate:"min=0"`
	}
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	items, total, err := h.svc.Changes(c.Context(), ChangeFilter{Role: req.Role, UserID: req.UserID}, req.Page, req.Limit)
	if err != nil {
		return fail(c, "changes", err)
	}
	return response.Paginated(c, items, total)
}
