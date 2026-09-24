package response

import (
	"log/slog"
	"strconv"

	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/upstream"

	"github.com/gofiber/fiber/v3"
)

var serviceLabel = map[string]string{
	"catalog":   "资料库服务",
	"community": "评论服务",
	"oauth":     "登录服务",
	"image":     "图片服务",
	"artifact":  "文件服务",
	"trust":     "举报服务",
	"store":     "商店服务",
}

// The frontend's catalog editor reads 50320; the rest are new.
var unavailableCode = map[string]int{
	"catalog":   50320,
	"community": 50321,
}

// Upstream answers a failed call to a NextMoe service. msg is the wording for
// the kinds the reader can act on (NotFound, Conflict, Rejected); an empty msg
// takes a generic one. The upstream's detail is an English diagnostic, so it
// only reaches the log, together with its request id.
func Upstream(c fiber.Ctx, err error, msg string) error {
	e, _ := upstream.As(err)
	switch upstream.KindOf(err) {
	case upstream.NotFound:
		return Error(c, errors.ErrNotFound(or(msg, "内容不存在或已被删除")))
	case upstream.Conflict:
		return Error(c, errors.ErrConflict(or(msg, "内容已被修改，请刷新后重试")))
	case upstream.Rejected:
		status := fiber.StatusBadRequest
		if e != nil && (e.Status == fiber.StatusForbidden || e.Status == fiber.StatusUnprocessableEntity) {
			status = e.Status
		}
		return Error(c, errors.New(status*100, or(msg, "请求未被接受"), status))
	case upstream.RateLimited:
		if e != nil && e.RetryAfter > 0 {
			c.Set("Retry-After", strconv.Itoa(int(e.RetryAfter.Seconds())))
		}
		return Error(c, errors.ErrTooManyRequests("操作过于频繁，请稍后再试"))
	case upstream.Unavailable:
		logUpstream(c, slog.LevelWarn, err, e)
		service := ""
		if e != nil {
			service = e.Service
		}
		code, ok := unavailableCode[service]
		if !ok {
			code = 50300
		}
		return Error(c, errors.New(code, or(serviceLabel[service], "依赖服务")+"暂不可用，请稍后再试", fiber.StatusServiceUnavailable))
	default:
		logUpstream(c, slog.LevelError, err, e)
		return Error(c, errors.ErrInternal("服务出错了，请稍后再试"))
	}
}

func logUpstream(c fiber.Ctx, level slog.Level, err error, e *upstream.Error) {
	attrs := []any{"method", c.Method(), "path", c.Path(), "error", err}
	if e != nil {
		attrs = append(attrs, "service", e.Service, "op", e.Op, "status", e.Status,
			"code", e.Code, "request_id", e.RequestID)
	}
	slog.Log(c.Context(), level, "upstream call failed", attrs...)
}

func or(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
