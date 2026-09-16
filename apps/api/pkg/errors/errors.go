package errors

import "github.com/gofiber/fiber/v3"

type AppError struct {
	Code       int    `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

func (e *AppError) Error() string {
	return e.Message
}

func New(code int, message string, httpStatus int) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: httpStatus}
}

func ErrUnauthorized() *AppError {
	return New(40100, "Please log in first", fiber.StatusUnauthorized)
}

func ErrAuthExpired() *AppError {
	return New(40101, "Session expired, please log in again", fiber.StatusUnauthorized)
}

func ErrForbidden() *AppError {
	return New(40300, "Insufficient permissions", fiber.StatusForbidden)
}

func ErrBadRequest(msg string) *AppError {
	if msg == "" {
		msg = "Invalid request parameters"
	}
	return New(40000, msg, fiber.StatusBadRequest)
}

func ErrNotFound(msg string) *AppError {
	if msg == "" {
		msg = "Resource not found"
	}
	return New(40400, msg, fiber.StatusNotFound)
}

func ErrValidation(msg string) *AppError {
	return New(42200, msg, fiber.StatusUnprocessableEntity)
}

func ErrInternal(msg string) *AppError {
	if msg == "" {
		msg = "Internal server error"
	}
	return New(50000, msg, fiber.StatusInternalServerError)
}

func ErrGalgameNotFound(msg string) *AppError {
	if msg == "" {
		msg = "游戏资料库中不存在该游戏，请先提交新作"
	}
	return New(44001, msg, fiber.StatusBadRequest)
}

func ErrAccountBanned(msg string) *AppError {
	if msg == "" {
		msg = "账号已被封禁，无法登录"
	}
	return New(10014, msg, fiber.StatusForbidden)
}

func ErrCatalogReauthRequired(msg string) *AppError {
	if msg == "" {
		msg = "登录凭证尚未包含资料库投稿权限，请退出登录后重新登录一次即可继续"
	}
	return New(40399, msg, fiber.StatusForbidden)
}

func ErrCatalogUnavailable(msg string) *AppError {
	if msg == "" {
		msg = "资料库服务暂不可用，请稍后再试"
	}
	return New(50320, msg, fiber.StatusServiceUnavailable)
}

func ErrCommunityUnavailable(msg string) *AppError {
	if msg == "" {
		msg = "评论服务暂不可用，请稍后再试"
	}
	return New(50321, msg, fiber.StatusServiceUnavailable)
}

func ErrConflict(msg string) *AppError {
	return New(40900, msg, fiber.StatusConflict)
}

func ErrTooManyRequests(msg string) *AppError {
	if msg == "" {
		msg = "Too many requests, please try again later"
	}
	return New(42900, msg, fiber.StatusTooManyRequests)
}
