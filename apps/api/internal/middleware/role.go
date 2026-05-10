package middleware

import (
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// RequireRole returns a middleware that admits requests whose OAuth roles
// claim contains any of the listed role strings. With no roles passed it
// admits any authenticated request.
//
// Role strings are the OAuth-side names ("admin", "moderator", ...) -- see
// docs/user-migration/02-data-mapping.md §7 for the mapping from legacy
// integer roles to OAuth roles.
func RequireRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if GetUser(c) == nil {
			return response.Error(c, errors.ErrUnauthorized())
		}
		if !HasAnyRole(c, roles...) {
			return response.Error(c, errors.ErrForbidden())
		}
		return c.Next()
	}
}
