package app

import (
	"kun-galgame-patch-api/internal/face"
	faceHandler "kun-galgame-patch-api/internal/face/handler"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/etag"
)

// faceCache matches catalog /v2's public lane verbatim. Every response here but
// the file route's is viewer-independent, so a shared cache is correct; the
// cost, accepted by infra in §16.5, is that a request Cloudflare answers is one
// the platform never meters. The file route overrides it with no-store: a
// shared copy would hand its link and password to callers the gateway never
// checked. So does the change feed, whose cached tail would hide new changes.
const faceCache = "public, max-age=300, s-maxage=1800, stale-while-revalidate=3600"

// faceCORS is deliberately not the site's CORS. The site allows a fixed origin
// list and sends credentials; the face is a public read API with no cookies, so
// it allows any origin and no credentials -- the two cannot be expressed by one
// policy. Preflight is answered here for completeness, though a browser cannot
// get one past the gateway: an OPTIONS request carries no API key, so
// ForwardAuth refuses it before it arrives. That is intended; an nmk_ key does
// not belong in a browser.
func faceCORS() fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{fiber.MethodGet, fiber.MethodOptions},
		AllowHeaders: []string{"Authorization", "X-API-Key", "Accept", "If-None-Match"},
		ExposeHeaders: []string{
			"ETag", "X-Request-ID",
			"X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset",
			"X-Quota-Limit", "X-Quota-Remaining", "Retry-After",
		},
		MaxAge: 86400,
	})
}

func faceHeaders(c fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, faceCache)
	c.Set(fiber.HeaderVary, "Accept-Encoding, Origin")
	return c.Next()
}

// mountFace registers the read face. There is no local rate limiter: the only
// client address this service can see is Traefik's, so an IP limiter would put
// every application on one budget. The real per-key budget is spent at the
// gateway before the request arrives.
func mountFace(app *fiber.App, h *faceHandler.Handler) {
	face := app.Group(face.Prefix, faceCORS(), faceHeaders, etag.New())

	face.Get("/patches", h.ListPatches)
	face.Get("/patches/:id", h.GetPatch)
	face.Get("/patches/:id/resources", h.ListPatchResources)
	face.Get("/resources/:id", h.GetResource)
	face.Get("/resources/:id/file", h.GetResourceFile)
	face.Get("/changes", h.ListChanges)
}
