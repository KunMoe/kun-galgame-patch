package handler

import (
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/galgame/enricher"
	"kun-galgame-patch-api/internal/galgame/taxonomyid"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

const apiV1Prefix = "/api/v1"

func galgamePathFromRequest(c fiber.Ctx) string {
	return strings.TrimPrefix(c.OriginalURL(), apiV1Prefix)
}

func (h *PatchHandler) GalgameTaxonomyDetailProxy(c fiber.Ctx) error {
	raw, handled, err := h.galgame.TaxonomyBrowse(c.Context(), galgamePathFromRequest(c))
	if !handled && err == nil {
		return response.Error(c, errors.ErrNotFound("词条不存在"))
	}
	if err != nil {
		if to, ok := galgameClient.MovedTarget(err); ok {
			return c.JSON(response.Response{
				Code: 0, Message: "OK", Data: fiber.Map{"moved_to": to},
			})
		}
		return response.Upstream(c, err, "词条不存在")
	}

	var envelope map[string]json.RawMessage
	if jerr := json.Unmarshal(raw, &envelope); jerr != nil {
		return c.JSON(response.Response{Code: 0, Message: "OK", Data: raw})
	}

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

	ids := make([]int, 0, len(briefs))
	for i := range briefs {
		if briefs[i].ID > 0 {
			ids = append(ids, briefs[i].ID)
		}
	}
	localPatches, lerr := h.service.GetPatchesByIDs(ids)
	if lerr != nil {
		slog.Warn("拉本地 patch 失败，将走 galgame 仅元信息的降级路径",
			"error", lerr, "count", len(ids))
	}
	enriched := enricher.EnrichPatches(c.Context(), h.galgame, h.users, localPatches, "")
	enrichedByID := make(map[int]enricher.GalgameCard, len(enriched))
	for i := range enriched {
		enrichedByID[enriched[i].ID] = enriched[i]
	}

	finalCards := make([]enricher.GalgameCard, 0, len(briefs))
	for i := range briefs {
		if card, ok := enrichedByID[briefs[i].ID]; ok {
			// The shelf comes from the taxonomy read, not the re-hydrate: this
			// enrichment runs with no content limit (the catalog read above
			// already picked the rows), and a shelf built that way printed 拔作
			// to a reader browsing 全年龄.
			card.Facet = briefs[i].Facet
			finalCards = append(finalCards, card)
			continue
		}
		finalCards = append(finalCards, enricher.CardFromBrief(&briefs[i]))
	}

	cardsJSON, merr := json.Marshal(finalCards)
	if merr != nil {
		return c.JSON(response.Response{Code: 0, Message: "OK", Data: raw})
	}
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

func (h *PatchHandler) ResolveTaxonomyID(c fiber.Ctx) error {
	kind := c.Params("kind")
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return response.Error(c, errors.ErrBadRequest("id 必须是正整数"))
	}

	switch kind {
	case "tag":
		catalogID, verdict := taxonomyid.ResolveTag(id)
		switch verdict {
		case taxonomyid.Moved:
			return response.OK(c, fiber.Map{"catalog_id": catalogID})
		case taxonomyid.Gone:
			return c.Status(fiber.StatusGone).JSON(response.Response{
				Code: 410, Message: "该标签已永久退役，没有对应的新词条",
			})
		}
		return response.Error(c, errors.ErrNotFound("标签不存在"))

	case "official":
		catalogID, found, lErr := h.galgame.ResolveWikiLabel(c.Context(), id)
		if lErr != nil {
			return response.Upstream(c, lErr, "会社不存在")
		}
		if !found {
			return response.Error(c, errors.ErrNotFound("会社不存在"))
		}
		return response.OK(c, fiber.Map{"catalog_id": catalogID})
	}
	return response.Error(c, errors.ErrBadRequest("未知的分类类型"))
}
