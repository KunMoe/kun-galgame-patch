package common

import (
	"strconv"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/galgame/enricher"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

type seriesDetailResponse struct {
	ID          int                    `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	HasNSFW     bool                   `json:"has_nsfw"`
	Galgames    []enricher.GalgameCard `json:"galgames"`
	Total       int64                  `json:"total"`
}

func (h *CommonHandler) GetGalgameSeries(c fiber.Ctx) error {
	id, err := entityID(c)
	if err != nil {
		return response.Error(c, err)
	}
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "24"))
	gate := ""
	if utils.ContentLimitForListBrowse(c) == "sfw" {
		gate = "sfw"
	}

	rec, cerr := h.galgame.GetSeries(c.Context(), id)
	if cerr != nil {
		return response.Error(c, catalogEntityError(cerr, "系列"))
	}

	out := seriesDetailResponse{
		ID:          rec.ID,
		Name:        rec.Name,
		Description: rec.Description,
		HasNSFW:     rec.HasNSFW,
		Galgames:    []enricher.GalgameCard{},
	}

	search, serr := h.galgame.SearchGalgame(c.Context(), galgameClient.SearchGalgameParams{
		SeriesID:     id,
		ContentLimit: gate,
		Page:         page,
		Limit:        limit,
	})
	if serr != nil {
		return response.Error(c, errors.ErrInternal("拉取系列作品失败"))
	}
	out.Total = search.Total

	gids := make([]int, 0, len(search.Items))
	for i := range search.Items {
		if search.Items[i].ID > 0 {
			gids = append(gids, search.Items[i].ID)
		}
	}
	var patches []patchModel.Patch
	if len(gids) > 0 {
		h.db.Where("id IN ?", gids).Find(&patches)
	}
	enriched := enricher.EnrichPatches(c.Context(), h.galgame, h.users, patches, "")
	enrichedByID := make(map[int]enricher.GalgameCard, len(enriched))
	for i := range enriched {
		enrichedByID[enriched[i].ID] = enriched[i]
	}
	for i := range search.Items {
		hit := &search.Items[i]
		if card, ok := enrichedByID[hit.ID]; ok {
			out.Galgames = append(out.Galgames, card)
			continue
		}
		brief := hitToBrief(hit)
		out.Galgames = append(out.Galgames, enricher.CardFromBrief(&brief))
	}
	return response.OK(c, out)
}

func hitToBrief(h *galgameClient.GalgameHit) galgameClient.GalgameBrief {
	return galgameClient.GalgameBrief{
		ID:                       h.ID,
		CatalogWorkID:            h.CatalogWorkID,
		VndbID:                   h.VndbID,
		ClaimState:               h.ClaimState,
		NameEnUs:                 h.NameEnUs,
		NameZhCn:                 h.NameZhCn,
		NameJaJp:                 h.NameJaJp,
		NameZhTw:                 h.NameZhTw,
		Banner:                   h.Banner,
		ContentLimit:             h.ContentLimit,
		AgeLimit:                 h.AgeLimit,
		OriginalLanguage:         h.OriginalLanguage,
		ReleaseDate:              h.ReleaseDate,
		EffectiveBannerHash:      h.EffectiveBannerHash,
		EffectiveBannerWidth:     h.EffectiveBannerWidth,
		EffectiveBannerHeight:    h.EffectiveBannerHeight,
		EffectiveBannerThumbhash: h.EffectiveBannerThumbhash,
		Covers:                   h.Covers,
		Screenshots:              h.Screenshots,
	}
}
