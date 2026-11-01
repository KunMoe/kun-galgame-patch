package middleware

import (
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/perm"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

func RequireRole(roles ...string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if GetUser(c) == nil {
			return response.Error(c, errors.ErrUnauthorized())
		}
		if !HasAnyRole(c, roles...) {
			return response.Error(c, errors.ErrForbidden())
		}
		return c.Next()
	}
}

func RequirePermission(p perm.Permission) fiber.Handler {
	return func(c fiber.Ctx) error {
		if GetUser(c) == nil {
			return response.Error(c, errors.ErrUnauthorized())
		}
		if !Can(c, p) {
			return response.Error(c, errors.ErrForbidden())
		}
		return c.Next()
	}
}

// RequireAnyPermission is a floor, not a gate: each route behind it still
// names its own permission.
func RequireAnyPermission(ps ...perm.Permission) fiber.Handler {
	return func(c fiber.Ctx) error {
		if GetUser(c) == nil {
			return response.Error(c, errors.ErrUnauthorized())
		}
		for _, p := range ps {
			if Can(c, p) {
				return c.Next()
			}
		}
		return response.Error(c, errors.ErrForbidden())
	}
}

func Can(c fiber.Ctx, p perm.Permission) bool {
	return GetUser(c) != nil && perm.CanUser(GetUserID(c), effectiveRoles(c), p)
}

func Permissions(c fiber.Ctx) []perm.Permission {
	if GetUser(c) == nil {
		return []perm.Permission{}
	}
	return perm.Effective(GetUserID(c), effectiveRoles(c))
}

func EffectiveRoles(c fiber.Ctx) []string { return effectiveRoles(c) }
