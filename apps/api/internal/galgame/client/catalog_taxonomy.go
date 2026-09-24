package client

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"kun-galgame-patch-api/pkg/catalogv2"
)

func (c *Client) TaxonomyBrowse(ctx context.Context, pathAndQuery string) (data json.RawMessage, handled bool, err error) {
	path, rawQuery := splitPathQuery(pathAndQuery)
	q, _ := url.ParseQuery(rawQuery)
	segs := strings.Split(strings.Trim(path, "/"), "/")
	if len(segs) == 0 || segs[0] == "" {
		return nil, false, nil
	}

	if len(segs) == 2 && segs[1] != "search" {
		switch segs[0] {
		case "tag":
			d, e := c.catalogTagDetail(ctx, q.Get("tag_id"), q)
			return d, true, e
		case "official":
			d, e := c.catalogLabelDetail(ctx, q.Get("official_id"), q)
			return d, true, e
		case "series":
			d, e := c.catalogSeriesDetail(ctx, q.Get("series_id"), q)
			return d, true, e
		}
	}
	return nil, false, nil
}

func splitPathQuery(pathAndQuery string) (path, rawQuery string) {
	if i := strings.IndexByte(pathAndQuery, '?'); i >= 0 {
		return pathAndQuery[:i], pathAndQuery[i+1:]
	}
	return pathAndQuery, ""
}

func taxonomyPageWindow(q url.Values) (page, limit int) {
	limit = atoiDefault(q.Get("limit"), 24)
	if limit <= 0 || limit > 50 {
		limit = 24
	}
	page = atoiDefault(q.Get("page"), 1)
	if page < 1 {
		page = 1
	}
	return page, limit
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return def
}

type catalogTagBrief struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Aliases      []string `json:"aliases"`
	Category     string   `json:"category"`
	Sexual       bool     `json:"sexual"`
	Description  string   `json:"description"`
	GalgameCount int      `json:"galgame_count"`
	Tier         string   `json:"tier"`
	Kind         string   `json:"kind"`
}

type catalogSeriesBrief struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	GalgameCount int    `json:"galgame_count"`
	// True whenever any member sits behind the r18 gate, counted before the
	// reader's own gate narrows the list — so the page can say why an SFW
	// reader is looking at fewer works than the series has.
	HasNSFW bool `json:"has_nsfw"`
}

type catalogOfficialBrief struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Aliases      []string `json:"aliases"`
	Category     string   `json:"category"`
	Lang         string   `json:"lang"`
	Link         string   `json:"link"`
	Description  string   `json:"description"`
	GalgameCount int      `json:"galgame_count"`
	// Split of galgame_count: works attributed to this company directly, and
	// works it only reaches through an imprint or subsidiary.
	OwnCount     int    `json:"own_galgame_count"`
	ImprintCount int    `json:"imprint_galgame_count"`
	LogoHash     string `json:"logo_hash"`
}

type catalogIntroRow struct {
	Lang    string `json:"lang"`
	Intro   string `json:"intro"`
	Source  string `json:"source"`
	Machine bool   `json:"machine"`
}

type catalogEntityLink struct {
	Source string `json:"source"`
	URL    string `json:"url"`
}

// Catalog wave 209 turned aliases[] from []string into these rows. Decoding
// them into the old []string failed the whole request, so every label that had
// an alias answered 50000 "调用 Galgame 资料库失败" — 35 of 60 sampled ids.
// The fake in catalog_fake_test.go still served flat strings, so the suite
// stayed green through it.
type catalogAlias struct {
	Value   string `json:"value"`
	Lang    string `json:"lang"`
	Kind    string `json:"kind"`
	Machine bool   `json:"machine"`
}

func aliasValues(rows []catalogAlias) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.Value != "" {
			out = append(out, r.Value)
		}
	}
	return out
}

func preferredIntro(rows []catalogIntroRow) string {
	order := []string{"zh-cn", "ja-jp", "en-us"}
	byKey := make(map[string]string, len(rows))
	for _, r := range rows {
		k := productLangFromCatalog(r.Lang)
		if _, taken := byKey[k]; !taken {
			byKey[k] = r.Intro
		}
	}
	for _, k := range order {
		if v := byKey[k]; v != "" {
			return v
		}
	}
	for _, r := range rows {
		if r.Intro != "" {
			return r.Intro
		}
	}
	return ""
}

func (c *Client) catalogTagDetail(ctx context.Context, idStr string, q url.Values) (json.RawMessage, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
	if err != nil || id <= 0 {
		return nil, catalogv2.Absent("GET /v2/catalog/tags/{id}")
	}
	gate := gateFor(q.Get("content_limit"))
	rec, err := c.v2.GetTag(ctx, id, true)
	if err != nil {
		return nil, err
	}
	members, total, err := c.taxonomyMembers(ctx, "tag_id", id, q, gate)
	if err != nil {
		return nil, err
	}
	tid, _ := rec.IntID()
	return json.Marshal(map[string]any{
		"tag": catalogTagBrief{
			ID:           tid,
			Name:         rec.DisplayName,
			Aliases:      []string{},
			Category:     tagCategoryFor(rec.IsSexual),
			Sexual:       rec.IsSexual,
			Description:  preferredIntro(introRowsFrom(rec.Intros)),
			GalgameCount: int(total),
			Tier:         rec.Tier,
			Kind:         rec.TagKind,
		},
		"galgames": members,
		"total":    total,
	})
}

func (c *Client) catalogSeriesDetail(ctx context.Context, idStr string, q url.Values) (json.RawMessage, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
	if err != nil || id <= 0 {
		return nil, catalogv2.Absent("GET /v2/catalog/series/{id}")
	}
	gate := gateFor(q.Get("content_limit"))
	rec, err := c.v2.GetSeries(ctx, id, true)
	if err != nil {
		return nil, err
	}
	members, total, err := c.taxonomyMembers(ctx, "series_id", id, q, gate)
	if err != nil {
		return nil, err
	}
	sid, _ := rec.IntID()
	return json.Marshal(map[string]any{
		"series": catalogSeriesBrief{
			ID:           int(sid),
			Name:         rec.DisplayName,
			Description:  preferredIntro(introRowsFrom(rec.Intros)),
			GalgameCount: int(total),
			HasNSFW:      rec.HasNSFW,
		},
		"galgames": members,
		"total":    total,
	})
}

func (c *Client) catalogLabelDetail(ctx context.Context, idStr string, q url.Values) (json.RawMessage, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
	if err != nil || id <= 0 {
		return nil, catalogv2.Absent("GET /v2/catalog/companies/{id}")
	}
	gate := gateFor(q.Get("content_limit"))
	rec, err := c.v2.GetCompany(ctx, id, true)
	if err != nil {
		return nil, err
	}
	roster, err := c.companyMembers(ctx, id, q, gate)
	if err != nil {
		return nil, err
	}
	cid, _ := rec.IntID()
	link := ""
	if links := linkRowsFrom(rec.Links); len(links) > 0 {
		link = links[0].URL
	}
	return json.Marshal(map[string]any{
		"official": catalogOfficialBrief{
			ID:           cid,
			Name:         rec.DisplayName,
			Aliases:      aliasValues(aliasRowsFrom(rec.Aliases)),
			Category:     rec.CompanyKind,
			Lang:         productLangFromCatalog(strOrEmpty(rec.Lang)),
			Link:         link,
			Description:  preferredIntro(introRowsFrom(rec.Intros)),
			LogoHash:     imageHash(rec.Logo),
			GalgameCount: int(roster.total),
			OwnCount:     int(roster.own),
			ImprintCount: int(roster.imprint),
		},
		"galgames": roster.members,
		"total":    roster.total,
	})
}

const taxonomyMemberSort = "released_desc"

func (c *Client) taxonomyMembers(ctx context.Context, filterKey string, id int64, q url.Values, gate catalogGate) ([]GalgameBrief, int64, error) {
	page, limit := taxonomyPageWindow(q)
	query := catalogv2.WorksQuery{
		Sort: taxonomyMemberSort, Page: page, Limit: limit, NSFW: true,
		Include: listCardInclude, IncludeTotal: true,
		Facets: []string{"olang"}, ContentLimit: gate.contentLimit,
	}
	switch filterKey {
	case "tag_id":
		query.TagIDs = strconv.FormatInt(id, 10)
	case "company_id":
		query.CompanyID = id
	case "series_id":
		query.SeriesID = id
	}
	data, err := c.v2.ListWorks(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	out := make([]GalgameBrief, 0, len(data.Items))
	for i := range data.Items {
		it := workToListItem(data.Items[i])
		if !it.ClaimedBy.renderable() || it.publicGID() == 0 {
			continue
		}
		b := catalogItemToBrief(&it)
		b.Facet = facetOf(data.Items[i], gate.contentLimit != "sfw")
		out = append(out, b)
	}
	return out, data.Count(), nil
}
