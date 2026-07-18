package handler

// Galgame taxonomy + relation proxy surface. Galgame metadata editing
// (revision history, edit-request PRs, direct edit) moved to kungal in the
// "编辑面归 kungal" wave; what remains here is a verbatim proxy to the Wiki
// Service for the links / aliases relations and the tag/official/engine/series
// taxonomy CRUD. Every endpoint is a verbatim proxy to the Wiki Service:
//
//   - Route paths mirror Wiki 1:1 (sans the /api/v1 prefix), so the Wiki path
//     is derived by stripping /api/v1 from the original URL — there are no
//     per-route path templates to keep in sync.
//   - Reads are public (optionalAuth: a token is forwarded only if the caller
//     happens to be logged in). Writes require login (auth) so a Bearer token
//     exists to forward; Wiki enforces creator/admin/role rules and we forward
//     its business code+message verbatim. We deliberately do NOT re-implement
//     authorization locally — §15: "鉴权语义以 wiki 端为准，下游不得放宽或收紧".

import (
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/galgame/enricher"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

const apiV1Prefix = "/api/v1"

// wikiPathFromRequest turns the incoming original URL into the Wiki path,
// preserving exact path/query encoding. c.OriginalURL() keeps the raw,
// undecoded path+query — important for the cosmetic non-ASCII /tag/:name
// segment (Wiki ignores :name and queries by tag_id, but the URL must stay
// syntactically valid).
func wikiPathFromRequest(c fiber.Ctx) string {
	return strings.TrimPrefix(c.OriginalURL(), apiV1Prefix)
}

// WikiEditProxy is the generic GET / JSON-write pass-through used by every
// remaining relation (links/aliases) and taxonomy proxy endpoint.
func (h *PatchHandler) WikiEditProxy(c fiber.Ctx) error {
	method := c.Method()
	accessToken := middleware.GetAccessToken(c)
	if method != fiber.MethodGet && accessToken == "" {
		return response.Error(c, errors.ErrUnauthorized())
	}

	// NSFW gate for GET on :gid-scoped sub-resources (links / aliases). Wiki's
	// content_limit protocol (docs/galgame_wiki/00-handbook §16.2) only lists
	// main list / search / taxonomy / batch / detail — these sub-resources fall
	// outside the matrix, so wiki won't filter them. An anonymous caller hitting
	// /galgame/<nsfw-gid>/aliases would otherwise receive the aliases of an
	// NSFW-gated entry. Mutating GETs don't exist on these routes; mutating
	// methods require auth above and are wiki-side authz'd, so they bypass the gate.
	if method == fiber.MethodGet {
		if gidStr := c.Params("gid"); gidStr != "" {
			if gid, err := strconv.Atoi(gidStr); err == nil && gid > 0 {
				if !h.gatePatchByContentLimit(c, gid) {
					return response.Error(c, errors.ErrNotFound("patch not found"))
				}
			}
		}
	}

	var body []byte
	if method != fiber.MethodGet {
		body = c.Body()
	}
	data, err := h.wiki.Proxy(
		c.Context(),
		method,
		wikiPathFromRequest(c),
		accessToken,
		body,
		string(c.Request().Header.ContentType()),
	)
	return writeWikiResult(c, data, err)
}

// WikiTaxonomyDetailProxy specializes WikiEditProxy for /tag/:name and
// /official/:name: it forwards to Wiki same as the generic proxy, then
// rewrites the response so the `galgame` array (Wiki's flat brief shape) is
// replaced with moyu's enriched `GalgameCard` shape (KunLanguage `name`,
// per-patch counts, banner-hash resolution etc.) — the same shape the home /
// galgame index pages already consume.
//
// For Wiki-listed galgames moyu has no local patch row for, a degraded card
// is emitted (Wiki name/banner/content_limit, zero counts) so the listing
// stays visually consistent and the user can still see the full Wiki set.
//
// The rewritten field is renamed `galgame` → `galgames` to match the rest
// of moyu's paginated list shape (and the existing FE TaxonomyListOpts
// reader fallbacks).
func (h *PatchHandler) WikiTaxonomyDetailProxy(c fiber.Ctx) error {
	if c.Method() != fiber.MethodGet {
		// Non-GETs (PUT/POST/DELETE) on tag/official go through the generic
		// proxy in WikiEditProxy; this method is read-only enrichment.
		return h.WikiEditProxy(c)
	}

	accessToken := middleware.GetAccessToken(c)
	raw, err := h.wiki.Proxy(
		c.Context(),
		fiber.MethodGet,
		wikiPathFromRequest(c),
		accessToken,
		nil,
		"",
	)
	if err != nil {
		if werr, ok := err.(*galgameClient.WikiError); ok {
			return response.Error(c, errors.New(werr.Code, werr.Message, fiber.StatusBadRequest))
		}
		return response.Error(c, errors.ErrInternal("调用 Galgame Wiki 失败"))
	}

	// Parse the Wiki `data` field into a generic map so we can swap out the
	// galgame array without touching the tag/official metadata around it.
	var envelope map[string]json.RawMessage
	if jerr := json.Unmarshal(raw, &envelope); jerr != nil {
		// Shape is unexpected (not a JSON object) — degrade to passthrough.
		return c.JSON(response.Response{Code: 0, Message: "OK", Data: raw})
	}

	// Wiki's TagDetail / OfficialDetail use `"galgame"` (singular). Be
	// defensive about `galgames` / `items` in case the upstream changes.
	var galgameKey string
	var briefs []galgameClient.GalgameBrief
	for _, key := range []string{"galgame", "galgames", "items"} {
		raw, ok := envelope[key]
		if !ok {
			continue
		}
		if jerr := json.Unmarshal(raw, &briefs); jerr == nil {
			galgameKey = key
			break
		}
	}
	if galgameKey == "" {
		return c.JSON(response.Response{Code: 0, Message: "OK", Data: raw})
	}

	// 1) Collect ids in Wiki order. 2) Find which moyu has locally.
	// 3) Enrich those (one /galgame/batch + one users batch internally).
	// 4) Walk the original Wiki order and emit either enriched or degraded.
	ids := make([]int, 0, len(briefs))
	for i := range briefs {
		if briefs[i].ID > 0 {
			ids = append(ids, briefs[i].ID)
		}
	}
	localPatches, lerr := h.service.GetPatchesByIDs(ids)
	if lerr != nil {
		slog.Warn("拉本地 patch 失败，将走 Wiki 仅元信息的降级路径",
			"error", lerr, "count", len(ids))
	}
	// content_limit="" — the briefs slice was already content_limit-filtered
	// by wiki itself at the /tag/:name level (wiki's taxonomy endpoints
	// default to sfw per docs/galgame_wiki/00-handbook §16.2, and the query
	// param if any was forwarded verbatim by the proxy above). Re-filtering
	// here would double-call wiki for no semantic gain; the walk below
	// preserves wiki's order and drops anything wiki excluded.
	enriched := enricher.EnrichPatches(c.Context(), h.wiki, h.users, localPatches, "")
	enrichedByID := make(map[int]enricher.GalgameCard, len(enriched))
	for i := range enriched {
		enrichedByID[enriched[i].ID] = enriched[i]
	}

	finalCards := make([]enricher.GalgameCard, 0, len(briefs))
	for i := range briefs {
		if card, ok := enrichedByID[briefs[i].ID]; ok {
			finalCards = append(finalCards, card)
			continue
		}
		finalCards = append(finalCards, enricher.CardFromBrief(&briefs[i]))
	}

	cardsJSON, merr := json.Marshal(finalCards)
	if merr != nil {
		return c.JSON(response.Response{Code: 0, Message: "OK", Data: raw})
	}
	// Standardize on `galgames` regardless of the upstream key — the FE
	// type already permits this and dropping the old key avoids shipping
	// the same data twice.
	if galgameKey != "galgames" {
		delete(envelope, galgameKey)
	}
	envelope["galgames"] = cardsJSON

	out, merr2 := json.Marshal(envelope)
	if merr2 != nil {
		return c.JSON(response.Response{Code: 0, Message: "OK", Data: raw})
	}
	return c.JSON(response.Response{Code: 0, Message: "OK", Data: json.RawMessage(out)})
}
