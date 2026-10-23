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
		// Without these c.IP() was Traefik's address for every reader, so each
		// per-IP limit was one bucket for the whole site: on 2026-10-23 the
		// 60/min /resource/:id limit answered every anonymous request 429.
		// Cloudflare sets CF-Connecting-IP; it is honored only from a private
		// peer (Traefik, or moyu-web forwarding it on SSR), and is spoofable
		// only by a request that reaches Traefik off the Cloudflare path.
		ProxyHeader:        "CF-Connecting-IP",
		TrustProxy:         true,
		TrustProxyConfig:   fiber.TrustProxyConfig{Private: true},
		EnableIPValidation: true,
	})
}
