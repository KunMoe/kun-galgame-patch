package middleware

import (
	"crypto/subtle"
	"strings"

	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

func BotKey(key string) fiber.Handler {
	return func(c fiber.Ctx) error {
		scheme, token, ok := strings.Cut(c.Get("Authorization"), " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" ||
			subtle.ConstantTimeCompare([]byte(token), []byte(key)) != 1 {
			return response.Error(c, errors.ErrUnauthorized())
		}
		return c.Next()
	}
}
