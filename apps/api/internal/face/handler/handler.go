// Package handler is the HTTP edge of the public developer-platform face.
//
// It reads no credential. The gateway has already decided who may call:
// Traefik runs ForwardAuth against oauth's
// /internal/devapi/forward-auth?face=moyu, which validates the nmk_ key,
// spends the key's rate and quota budget and meters the admitted request.
// Nothing reaches here unkeyed from the public internet, and every answer but
// the file route's is the same for every caller -- which is what makes the
// rest of the face cacheable.
package handler

import (
	"slices"
	"strconv"
	"strings"

	"kun-galgame-patch-api/internal/face/service"
	"kun-galgame-patch-api/pkg/problem"

	"github.com/gofiber/fiber/v3"
)

type Handler struct {
	svc *service.Service
}

func New(svc *service.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) ListPatches(c fiber.Ctx) error {
	q, prob := service.ParsePatchQuery(c)
	if prob != nil {
		return problem.Write(c, prob)
	}
	out, prob := h.svc.ListPatches(c.Context(), q)
	if prob != nil {
		return problem.Write(c, prob)
	}
	return c.JSON(out)
}

func (h *Handler) GetPatch(c fiber.Ctx) error {
	id, prob := pathID(c, "id")
	if prob != nil {
		return problem.Write(c, prob)
	}
	include, prob := service.ParseInclude(c, service.PatchIncludes)
	if prob != nil {
		return problem.Write(c, prob)
	}
	out, prob := h.svc.GetPatch(c.Context(), id, include)
	if prob != nil {
		return problem.Write(c, prob)
	}
	return c.JSON(out)
}

func (h *Handler) ListPatchResources(c fiber.Ctx) error {
	id, prob := pathID(c, "id")
	if prob != nil {
		return problem.Write(c, prob)
	}
	include, prob := service.ParseInclude(c, service.ResourceIncludes)
	if prob != nil {
		return problem.Write(c, prob)
	}
	limit, prob := service.ParseLimit(c, false)
	if prob != nil {
		return problem.Write(c, prob)
	}
	offset, prob := service.ParseCursor(c)
	if prob != nil {
		return problem.Write(c, prob)
	}
	includeTotal, prob := service.ParseBool(c, "include_total", false)
	if prob != nil {
		return problem.Write(c, prob)
	}
	out, prob := h.svc.ListPatchResources(c.Context(), id, offset, limit, include, includeTotal)
	if prob != nil {
		return problem.Write(c, prob)
	}
	return c.JSON(out)
}

func (h *Handler) GetResource(c fiber.Ctx) error {
	id, prob := pathID(c, "id")
	if prob != nil {
		return problem.Write(c, prob)
	}
	include, prob := service.ParseInclude(c, service.ResourceIncludes)
	if prob != nil {
		return problem.Write(c, prob)
	}
	out, prob := h.svc.GetResource(c.Context(), id, include)
	if prob != nil {
		return problem.Write(c, prob)
	}
	return c.JSON(out)
}

func (h *Handler) ListChanges(c fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	after, prob := service.ParseChangeCursor(c)
	if prob != nil {
		return problem.Write(c, prob)
	}
	limit, prob := service.ParseLimit(c, false)
	if prob != nil {
		return problem.Write(c, prob)
	}
	out, prob := h.svc.ListChanges(c.Context(), after, limit)
	if prob != nil {
		return problem.Write(c, prob)
	}
	return c.JSON(out)
}

const filesScope = "moyu:files"

func (h *Handler) GetResourceFile(c fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "private, no-store")
	// Trusted only because docker-compose.prod.yml lists X-NextMoe-Scopes in the
	// forwardauth authResponseHeaders: Traefik overwrites a listed header and
	// passes an unlisted one through exactly as the client sent it.
	if !slices.Contains(strings.Fields(c.Get("X-NextMoe-Scopes")), filesScope) {
		return problem.WriteCode(c, problem.CodeScopeRequired,
			"this operation needs the "+filesScope+" scope, which an operator grants")
	}
	id, prob := pathID(c, "id")
	if prob != nil {
		return problem.Write(c, prob)
	}
	out, prob := h.svc.GetResourceFile(c.Context(), id)
	if prob != nil {
		return problem.Write(c, prob)
	}
	return c.JSON(out)
}

// pathID refuses a non-numeric id rather than looking it up and answering 404.
// The two are different answers: one says this URL is malformed, the other says
// nothing lives at a well-formed one.
func pathID(c fiber.Ctx, name string) (int, *problem.Problem) {
	raw := c.Params(name)
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		return 0, problem.Param(problem.CodeInvalidParameter, name,
			problem.ReasonInvalidFormat, "not an id: "+raw)
	}
	return id, nil
}
