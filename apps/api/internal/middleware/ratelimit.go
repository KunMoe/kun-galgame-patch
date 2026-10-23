package middleware

import (
	"context"
	"fmt"
	"time"

	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
)

func RateLimit(rdb *redis.Client, prefix string, maxRequests int, window time.Duration) fiber.Handler {
	return func(c fiber.Ctx) error {
		userID := GetUserID(c)
		ip := c.IP()

		var key string
		if userID > 0 {
			key = fmt.Sprintf("ratelimit:%s:user:%d", prefix, userID)
		} else {
			key = fmt.Sprintf("ratelimit:%s:ip:%s", prefix, ip)
		}

		// Expiry set only on the first hit let a key outlive its window for
		// good: production held moyu counters with no TTL at 5,398 to
		// 1,429,391 on 2026-10-23, each answering 429 forever. EXPIRE NX on
		// every hit restores a missing TTL without extending a live one.
		ctx := context.Background()
		pipe := rdb.TxPipeline()
		incr := pipe.Incr(ctx, key)
		pipe.ExpireNX(ctx, key, window)
		if _, err := pipe.Exec(ctx); err != nil {
			return c.Next()
		}
		count := incr.Val()

		if count > int64(maxRequests) {
			return response.Error(c, errors.ErrTooManyRequests(""))
		}

		return c.Next()
	}
}
