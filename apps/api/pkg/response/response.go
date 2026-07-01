package response

import (
	"kun-galgame-patch-api/pkg/errors"

	"github.com/gofiber/fiber/v3"
)

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// PaginatedData is the inner payload for a paginated response: { items, total }.
type PaginatedData struct {
	Items any   `json:"items"`
	Total int64 `json:"total"`
}

func OK(c fiber.Ctx, data any) error {
	return c.JSON(Response{
		Code:    0,
		Message: "OK",
		Data:    data,
	})
}

func OKMessage(c fiber.Ctx, msg string) error {
	return c.JSON(Response{
		Code:    0,
		Message: msg,
		Data:    nil,
	})
}

// Paginated emits { code, message, data: { items, total } }.
// Nullish items are normalized to an empty slice to keep the shape stable for the frontend.
func Paginated(c fiber.Ctx, items any, total int64) error {
	return c.JSON(Response{
		Code:    0,
		Message: "OK",
		Data:    PaginatedData{Items: items, Total: total},
	})
}

func Error(c fiber.Ctx, err *errors.AppError) error {
	return c.Status(err.HTTPStatus).JSON(Response{
		Code:    err.Code,
		Message: err.Message,
		Data:    nil,
	})
}
