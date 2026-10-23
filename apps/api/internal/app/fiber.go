package app

import "github.com/gofiber/fiber/v3"

func newFiber() *fiber.App {
	return fiber.New(fiber.Config{
		// Off, every c.Cookies / c.Query / c.IP string aliases a buffer fasthttp
		// reuses for the next request, and a background session refresh wrote one
		// reader into another's session (the 2026-10-22 account swaps).
		Immutable:      true,
		BodyLimit:      10 * 1024 * 1024,
		ReadBufferSize: 32 * 1024,
		ErrorHandler:   globalErrorHandler,
	})
}
