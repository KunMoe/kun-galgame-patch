package handler

// Galgame editing surface mandated by docs/galgame_wiki/00-handbook-for-downstream.md
// §15 for kungal AND moyu: revisions, PRs, links/aliases/contributors and the
// tag/official/engine/series taxonomy CRUD. Every endpoint is a verbatim proxy
// to the Wiki Service:
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
	"io"
	"log/slog"
	"strings"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/galgame/enricher"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

const apiV1Prefix = "/api/v1"

// wikiPathFromRequest turns the incoming original URL into the Wiki path,
// preserving exact path/query encoding. c.OriginalURL() keeps the raw,
// undecoded path+query — important for the cosmetic non-ASCII /tag/:name
// segment (Wiki ignores :name and queries by tag_id, but the URL must stay
// syntactically valid).
func wikiPathFromRequest(c *fiber.Ctx) string {
	return strings.TrimPrefix(c.OriginalURL(), apiV1Prefix)
}

// WikiEditProxy is the generic GET / JSON-write pass-through used by every
// §15 endpoint except the multipart-capable PR submit (see WikiPRSubmit).
func (h *PatchHandler) WikiEditProxy(c *fiber.Ctx) error {
	method := c.Method()
	accessToken := middleware.GetAccessToken(c)
	if method != fiber.MethodGet && accessToken == "" {
		return response.Error(c, errors.ErrUnauthorized())
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
func (h *PatchHandler) WikiTaxonomyDetailProxy(c *fiber.Ctx) error {
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
	enriched := enricher.EnrichPatches(c.Context(), h.wiki, h.users, localPatches)
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

// WikiPRSubmit handles POST /api/v1/galgame/:gid/prs. Like Submit/Update it
// accepts JSON or multipart/form-data (`data` JSON + optional `file` banner)
// so a PR proposal can carry a new banner thumbnail for the reviewer
// (docs/galgame_wiki/02-revisions-and-prs.md §PR).
func (h *PatchHandler) WikiPRSubmit(c *fiber.Ctx) error {
	accessToken := middleware.GetAccessToken(c)
	if accessToken == "" {
		return response.Error(c, errors.ErrUnauthorized())
	}
	wikiPath := wikiPathFromRequest(c)
	ctype := string(c.Request().Header.ContentType())

	if !strings.HasPrefix(ctype, "multipart/form-data") {
		data, err := h.wiki.Proxy(
			c.Context(), fiber.MethodPost, wikiPath, accessToken, c.Body(), ctype,
		)
		return writeWikiResult(c, data, err)
	}

	form, err := c.MultipartForm()
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("multipart 表单解析失败"))
	}
	dataStrs := form.Value["data"]
	if len(dataStrs) == 0 {
		return response.Error(c, errors.ErrBadRequest("缺少 data 字段"))
	}
	dataJSON := []byte(dataStrs[0])

	var fileName, fileMime string
	var fileBytes []byte
	if fhs := form.File["file"]; len(fhs) > 0 {
		fh := fhs[0]
		if fh.Size > 10*1024*1024 {
			return response.Error(c, errors.ErrBadRequest("banner 超过 10MB 上限"))
		}
		f, oerr := fh.Open()
		if oerr != nil {
			return response.Error(c, errors.ErrBadRequest("无法读取上传文件"))
		}
		defer f.Close()
		b, rerr := io.ReadAll(f)
		if rerr != nil {
			return response.Error(c, errors.ErrBadRequest("读取上传文件失败"))
		}
		fileBytes = b
		fileName = fh.Filename
		fileMime = fh.Header.Get("Content-Type")
		if fileMime == "" {
			fileMime = "application/octet-stream"
		}
	}

	data, callErr := h.wiki.ProxyMultipart(
		c.Context(), fiber.MethodPost, wikiPath, accessToken,
		dataJSON, fileName, fileBytes, fileMime,
	)
	return writeWikiResult(c, data, callErr)
}
