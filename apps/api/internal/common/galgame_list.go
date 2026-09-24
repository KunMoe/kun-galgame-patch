package common

import (
	"fmt"
	"slices"
	"strings"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/galgame/enricher"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

// /galgame is the patch resource list and /galgame?library=true is the catalog
// information library — the same split kungal draws between /galgame and
// /gallib. indexed=true is the sitemap's own lane and stays local whatever else
// the query says.
func catalogLibraryRequest(req galgameListRequest) bool {
	return req.Library && !req.Indexed
}

func catalogLibrarySort(field, order string) string {
	switch field {
	case "release_date":
		if order == "asc" {
			return "released_asc"
		}
		return "released_desc"
	case "created":
		return "id"
	case "updated", "resource_update_time":
		return "updated"
	default:
		return "popularity"
	}
}

// patch.language and patch.platform are jsonb arrays aggregated from the row's
// resources, and both land inside a jsonb literal — so an unknown value is
// dropped rather than escaped: `language @> '["a\""]'` is not a filter, it is a
// Postgres error the reader sees as a 500.
var (
	patchLanguages = []string{"zh-Hans", "zh-Hant", "ja", "en", "other"}
	patchPlatforms = []string{"windows", "android", "macos", "ios", "linux", "other"}
)

func allowedValues(raw string, allowed []string) []string {
	out := make([]string, 0, len(allowed))
	for _, value := range strings.Split(raw, ",") {
		value = strings.TrimSpace(value)
		if slices.Contains(allowed, value) && !slices.Contains(out, value) {
			out = append(out, value)
		}
	}
	return out
}

// Ticking two languages is an OR: a reader who wants 简体中文 or 繁體中文 wants
// either, not a game whose patches cover both.
func scopeJSONArrayAny(db *gorm.DB, column string, values []string) *gorm.DB {
	if len(values) == 0 {
		return db
	}
	clauses := make([]string, 0, len(values))
	args := make([]any, 0, len(values))
	for _, value := range values {
		clauses = append(clauses, column+" @> ?")
		args = append(args, fmt.Sprintf(`["%s"]`, value))
	}
	return db.Where(strings.Join(clauses, " OR "), args...)
}

func (h *CommonHandler) GetGalgameList(c fiber.Ctx) error {
	var req galgameListRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	cl := utils.ContentLimitForListBrowse(c)

	lower, err := utils.ParseReleaseLowerBound(req.ReleasedFrom)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	upper, err := utils.ParseReleaseUpperBound(req.ReleasedTo)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	months, err := utils.ParseMonthSet(req.ReleasedMonths)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	if catalogLibraryRequest(req) {
		return h.catalogLibrary(c, req, cl)
	}
	switch req.SortField {
	case "popularity", "updated":
		req.SortField = "resource_update_time"
	}

	base := h.db.Model(&patchModel.Patch{})
	if req.Indexed {
		base = base.Where("published")
	}
	if req.SelectedType != "all" {
		base = base.Where("type @> ?", fmt.Sprintf(`["%s"]`, req.SelectedType))
	}
	base = scopeJSONArrayAny(base, "language", allowedValues(req.Language, patchLanguages))
	base = scopeJSONArrayAny(base, "platform", allowedValues(req.Platform, patchPlatforms))
	if lower != nil {
		base = base.Where("release_date >= ?", *lower)
	}
	if upper != nil {
		base = base.Where("release_date <= ?", *upper)
	}
	if len(months) > 0 {
		base = base.Where("EXTRACT(MONTH FROM release_date)::int IN ?", months)
	}
	if !req.Indexed && !utils.IncludeEmptyGalgames(c) {
		base = base.Where("resource_count > 0")
	}
	base = utils.ScopePatchContentLimit(base, cl)

	var total int64
	base.Session(&gorm.Session{}).Count(&total)

	var patches []patchModel.Patch
	if err := base.Session(&gorm.Session{}).Order(fmt.Sprintf("%s %s, id DESC", req.SortField, req.SortOrder)).
		Offset((req.Page - 1) * req.Limit).Limit(req.Limit).
		Find(&patches).Error; err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	cards := enricher.EnrichPatchCards(c.Context(), h.galgame, h.users, patches, cl)

	return response.OK(c, map[string]any{
		"galgames": cards,
		"total":    total,
	})
}

func (h *CommonHandler) catalogLibrary(c fiber.Ctx, req galgameListRequest, cl string) error {
	if h.galgame == nil {
		return response.Error(c, errors.ErrInternal("Galgame 目录未启用"))
	}
	params := galgameClient.SearchGalgameParams{
		ContentLimit: cl,
		Sort:         catalogLibrarySort(req.SortField, req.SortOrder),
		Page:         req.Page,
		Limit:        req.Limit,
	}
	if req.ReleasedFrom != "" {
		if y, err := utils.ParseReleaseLowerBound(req.ReleasedFrom); err == nil && y != nil {
			params.ReleasedFrom = y.Year()
		}
	}
	if req.ReleasedTo != "" {
		if y, err := utils.ParseReleaseUpperBound(req.ReleasedTo); err == nil && y != nil {
			params.ReleasedTo = y.Year()
		}
	}
	// worksQueryFor always names a facet, which is what puts /v2/catalog/works on
	// the search index — so the library lane takes company_id and tag_id with no
	// keyword at all, the same as the search page's.
	if req.CompanyID > 0 {
		params.OfficialIDs = []int{req.CompanyID}
	}
	params.TagIDs = parseSearchIDs(req.TagIDs)

	res, err := h.galgame.SearchGalgame(c.Context(), params)
	if err != nil {
		return response.Upstream(c, err, "")
	}

	ids := make([]int, 0, len(res.Items))
	for i := range res.Items {
		if res.Items[i].ID > 0 {
			ids = append(ids, res.Items[i].ID)
		}
	}
	local := h.localPatchMap(ids)
	cards := overlayCatalogHits(res.Items, local, cl)
	return response.OK(c, map[string]any{
		"galgames": cards,
		"total":    res.Total,
	})
}

func (h *CommonHandler) localPatchMap(ids []int) map[int]patchModel.Patch {
	out := make(map[int]patchModel.Patch, len(ids))
	if len(ids) == 0 {
		return out
	}
	var rows []patchModel.Patch
	if err := h.db.Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return out
	}
	for i := range rows {
		out[rows[i].ID] = rows[i]
	}
	return out
}

func overlayCatalogHits(hits []galgameClient.GalgameHit, local map[int]patchModel.Patch, _ string) []enricher.GalgameCard {
	cards := make([]enricher.GalgameCard, 0, len(hits))
	for i := range hits {
		hit := &hits[i]
		brief := hitToBrief(hit)
		if p, ok := local[hit.ID]; ok {
			card := enricher.CardFromBrief(&brief)
			card.IsOnForum = true
			card.Indexed = p.Published
			card.View = p.View
			card.Download = p.Download
			card.Type = p.Type
			card.Language = p.Language
			card.Platform = p.Platform
			card.Created = p.Created
			card.ResourceUpdateTime = p.ResourceUpdateTime
			card.Count = enricher.Counts{
				FavoriteBy:   p.FavoriteCount,
				ContributeBy: p.ContributeCount,
				Resource:     p.ResourceCount,
				Comment:      p.CommentCount,
			}
			cards = append(cards, card)
			continue
		}
		cards = append(cards, enricher.CardFromBrief(&brief))
	}
	return cards
}

func hitToBrief(h *galgameClient.GalgameHit) galgameClient.GalgameBrief {
	return galgameClient.GalgameBrief{
		ID:                         h.ID,
		ForumGID:                   h.ForumGID,
		VndbID:                     h.VndbID,
		ClaimState:                 h.ClaimState,
		NameEnUs:                   h.NameEnUs,
		NameZhCn:                   h.NameZhCn,
		NameJaJp:                   h.NameJaJp,
		NameZhTw:                   h.NameZhTw,
		DisplayName:                h.DisplayName,
		Latin:                      h.Latin,
		NameMachineTranslated:      h.NameMachineTranslated,
		Banner:                     h.Banner,
		ContentLimit:               h.ContentLimit,
		AgeLimit:                   h.AgeLimit,
		OriginalLanguage:           h.OriginalLanguage,
		ReleaseDate:                h.ReleaseDate,
		ReleasePrecision:           h.ReleasePrecision,
		EffectiveBannerHash:        h.EffectiveBannerHash,
		EffectiveBannerWidth:       h.EffectiveBannerWidth,
		EffectiveBannerHeight:      h.EffectiveBannerHeight,
		EffectiveBannerThumbhash:   h.EffectiveBannerThumbhash,
		EffectivePortraitHash:      h.EffectivePortraitHash,
		EffectivePortraitWidth:     h.EffectivePortraitWidth,
		EffectivePortraitHeight:    h.EffectivePortraitHeight,
		EffectivePortraitThumbhash: h.EffectivePortraitThumbhash,
		Maker:                      h.Maker,
		Facet:                      h.Facet,
	}
}
