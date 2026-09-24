package middleware

import (
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

// BodyLimit caps one route below the app-wide limit. It reads the raw body, so
// a compressed request is measured as sent and never inflated to be measured.
func BodyLimit(n int) fiber.Handler {
	return func(c fiber.Ctx) error {
		if len(c.Request().Body()) > n {
			return response.Error(c, errors.New(41300, "请求体过大", fiber.StatusRequestEntityTooLarge))
		}
		return c.Next()
	}
}
