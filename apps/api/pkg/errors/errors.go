package errors

import "github.com/gofiber/fiber/v2"

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

// Authentication related
func ErrUnauthorized() *AppError {
	return New(40100, "Please log in first", fiber.StatusUnauthorized)
}

func ErrAuthExpired() *AppError {
	return New(40101, "Session expired, please log in again", fiber.StatusUnauthorized)
}

func ErrForbidden() *AppError {
	return New(40300, "Insufficient permissions", fiber.StatusForbidden)
}

// Request related
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

// Server related
func ErrInternal(msg string) *AppError {
	if msg == "" {
		msg = "Internal server error"
	}
	return New(50000, msg, fiber.StatusInternalServerError)
}

// Business related
func ErrBusiness(msg string) *AppError {
	return New(40000, msg, fiber.StatusBadRequest)
}

// ErrWikiGalgameNotFound is returned by POST /patch when the supplied
// vndb_id does not yet exist on the Galgame Wiki. Distinct code (44001) so
// the frontend can render a "前往 Wiki 创建" CTA instead of a generic toast.
func ErrWikiGalgameNotFound(msg string) *AppError {
	if msg == "" {
		msg = "Galgame Wiki 中不存在该游戏，请先到 Wiki 创建"
	}
	return New(44001, msg, fiber.StatusBadRequest)
}

// ErrAccountBanned mirrors OAuth's code 10014 (HTTP 403). Per
// docs/oauth/api-reference.md the frontend should land users on an
// "account banned" page rather than the login page — re-login will not help.
func ErrAccountBanned(msg string) *AppError {
	if msg == "" {
		msg = "账号已被封禁，无法登录"
	}
	return New(10014, msg, fiber.StatusForbidden)
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
