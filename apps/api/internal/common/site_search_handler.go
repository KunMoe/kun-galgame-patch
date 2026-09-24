package common

import (
	"strconv"
	"strings"

	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

// Everything below Limit is a per-lane filter: scope belongs to 补丁资源 and the
// rest to Galgame. They are on one struct because they arrive on one endpoint;
// a filter sent to the wrong lane is ignored rather than refused, the way an
// unread query parameter always is.
type siteSearchRequest struct {
	Keywords     string `query:"keywords" validate:"required,max=107"`
	Type         string `query:"type" validate:"required,oneof=galgame resource user"`
	Page         int    `query:"page" validate:"required,min=1"`
	Limit        int    `query:"limit" validate:"required,min=1,max=24"`
	Scope        string `query:"scope" validate:"omitempty,oneof=model"`
	Sort         string `query:"sort" validate:"omitempty,oneof=relevance released_desc released_asc updated popularity"`
	TagIDs       string `query:"tag_ids" validate:"omitempty,max=107"`
	CompanyID    int    `query:"company_id" validate:"omitempty,min=1"`
	ReleasedFrom int    `query:"released_from" validate:"omitempty,min=1980,max=2100"`
	ReleasedTo   int    `query:"released_to" validate:"omitempty,min=1980,max=2100"`
}

// Catalog takes at most ten tag ids and answers 400 past that, which is the
// panel's own ceiling; anything unparseable is dropped rather than refused
// because a hand-edited URL should still search.
func parseSearchIDs(raw string) []int {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	seen := make(map[int]struct{})
	ids := make([]int, 0, 10)
	for _, part := range strings.Split(raw, ",") {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || id <= 0 {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

type siteSearchKeywordsRequest struct {
	Keywords string `query:"keywords" validate:"required,max=107"`
}

// An empty family is 全部: every family at once, which is what the 资料库 tab
// opens on. Page and limit only mean anything once one family is named.
type siteSearchEntityRequest struct {
	Keywords string `query:"keywords" validate:"required,max=107"`
	Family   string `query:"family" validate:"omitempty,oneof=character company staff tag series"`
	Page     int    `query:"page" validate:"required,min=1"`
	Limit    int    `query:"limit" validate:"required,min=1,max=24"`
}

// SiteSearch answers one category, paged. The overview below answers all of
// them at once and is what the category rail counts with, so a deep link
// straight into 用户 still knows how many games the keyword matched.
func (h *CommonHandler) SiteSearch(c fiber.Ctx) error {
	var req siteSearchRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if len(searchKeywords(req.Keywords)) == 0 {
		return response.Error(c, errors.ErrBadRequest("搜索关键词不能为空"))
	}

	switch req.Type {
	case "galgame":
		items, total, err := h.searchGalgameLane(
			c.Context(), req.Keywords, req.Page, req.Limit, galgameSearchFilter{
				TagIDs:       parseSearchIDs(req.TagIDs),
				CompanyID:    req.CompanyID,
				ReleasedFrom: req.ReleasedFrom,
				ReleasedTo:   req.ReleasedTo,
				Sort:         req.Sort,
			})
		if err != nil {
			return response.Upstream(c, err, "")
		}
		return response.Paginated(c, items, total)
	case "resource":
		items, total, appErr := h.searchResourceLane(
			c.Context(), req.Keywords, req.Page, req.Limit,
			utils.ContentLimitForListBrowse(c), req.Scope)
		if appErr != nil {
			return response.Error(c, appErr)
		}
		return response.Paginated(c, items, total)
	default:
		items, total, appErr := h.searchUserLane(c.Context(), req.Keywords, req.Page, req.Limit)
		if appErr != nil {
			return response.Error(c, appErr)
		}
		return response.Paginated(c, items, total)
	}
}

func (h *CommonHandler) SiteSearchOverview(c fiber.Ctx) error {
	var req siteSearchKeywordsRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if len(searchKeywords(req.Keywords)) == 0 {
		return response.Error(c, errors.ErrBadRequest("搜索关键词不能为空"))
	}

	return response.OK(c, h.runSearchLanes(
		c.Context(), req.Keywords, utils.ContentLimitForListBrowse(c),
		searchLaneLimits{
			galgame:  searchOverviewGalgameLimit,
			entity:   searchOverviewEntityLimit,
			resource: searchOverviewResourceLimit,
			user:     searchOverviewUserLimit,
		},
	))
}

// SiteSearchEntity is its own endpoint rather than a type of SiteSearch: the
// 资料库 tab pages one family at a time and 全部 pages none of them, so neither
// the response shape nor the paginator matches the other three lanes.
func (h *CommonHandler) SiteSearchEntity(c fiber.Ctx) error {
	var req siteSearchEntityRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if len(searchKeywords(req.Keywords)) == 0 {
		return response.Error(c, errors.ErrBadRequest("搜索关键词不能为空"))
	}

	groups, appErr := h.searchEntityLane(
		c.Context(), req.Keywords, req.Family, req.Page, req.Limit,
		utils.ContentLimitForListBrowse(c))
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, fiber.Map{"groups": groups})
}

type siteSearchEntityResolveRequest struct {
	Family string `query:"family" validate:"required,oneof=company tag"`
	IDs    string `query:"ids" validate:"required,max=107"`
}

// SiteSearchEntityResolve turns the ids in a shared 高级筛选 link back into the
// names its chips are drawn with. Only the two families the panel filters by
// have a batch face worth spending a request on.
func (h *CommonHandler) SiteSearchEntityResolve(c fiber.Ctx) error {
	var req siteSearchEntityResolveRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if h.galgame == nil {
		return response.Error(c, errors.ErrInternal("Galgame 资料库未启用"))
	}
	items, err := h.galgame.ResolveEntities(c.Context(), req.Family, parseSearchIDs(req.IDs))
	if err != nil {
		return response.Upstream(c, err, "")
	}
	return response.OK(c, fiber.Map{"items": items})
}

func (h *CommonHandler) SiteSearchQuick(c fiber.Ctx) error {
	var req siteSearchKeywordsRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if len(searchKeywords(req.Keywords)) == 0 {
		return response.Error(c, errors.ErrBadRequest("搜索关键词不能为空"))
	}

	return response.OK(c, h.runSearchLanes(
		c.Context(), req.Keywords, utils.ContentLimitForListBrowse(c),
		searchLaneLimits{
			galgame:  searchQuickLimit,
			resource: searchQuickLimit,
			user:     searchQuickLimit,
		},
	))
}
