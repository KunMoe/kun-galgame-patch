package middleware

import (
	"context"
	"fmt"
	"time"

	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

func RateLimit(rdb *redis.Client, prefix string, maxRequests int, window time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := GetUserID(c)
		ip := c.IP()

		var key string
		if userID > 0 {
			key = fmt.Sprintf("ratelimit:%s:user:%d", prefix, userID)
		} else {
			key = fmt.Sprintf("ratelimit:%s:ip:%s", prefix, ip)
		}

		ctx := context.Background()
		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			return c.Next()
		}

		if count == 1 {
			rdb.Expire(ctx, key, window)
		}

		if count > int64(maxRequests) {
			return response.Error(c, errors.ErrTooManyRequests(""))
		}

		return c.Next()
	}
}
