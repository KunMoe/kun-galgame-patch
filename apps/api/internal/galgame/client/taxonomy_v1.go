package client

// Taxonomy read reshapers (open-API phase 2 wave 07, W3). The generic Proxy
// forwards moyu's verbatim taxonomy/relation reads; this file intercepts the
// A-bucket GET reads and serves them from the /v1 public contract, reshaping the
// curated records back to the bridge `data` the moyu handler + FE consume.
//
// The B-bucket reads (tag/official/engine/series /:id/revisions) and every write
// are NOT handled here — they fall through to the internal / legacy face
// unchanged (see Proxy). The /v1 curation deliberately drops raw-model-only
// fields (taxonomy alias-row {id,created,updated}, engine created/updated,
// link/alias row bookkeeping) that the moyu FE does not consume (W3 census);
// those diffs are the expected route-B curation, not a regression.

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
)

// proxyReadV1 classifies pathAndQuery as an A-bucket taxonomy/relation read and,
// when it is one, serves it from /v1 (returning the reshaped bridge `data` and
// handled=true). Returns handled=false to let Proxy fall through to the
// internal/legacy routing (B-bucket revisions, writes, anything unrecognized).
func (c *Client) proxyReadV1(ctx context.Context, pathAndQuery string) (data json.RawMessage, handled bool, err error) {
	path := pathAndQuery
	rawQuery := ""
	if i := strings.IndexByte(path, '?'); i >= 0 {
		rawQuery = path[i+1:]
		path = path[:i]
	}
	q, _ := url.ParseQuery(rawQuery)
	segs := strings.Split(strings.Trim(path, "/"), "/")
	if len(segs) == 0 || segs[0] == "" {
		return nil, false, nil
	}

	switch segs[0] {
	case "tag":
		switch {
		case len(segs) == 1:
			return c.wrap(c.v1TaxList(ctx, "/galgame/tags", q))
		case segs[1] == "search":
			return c.wrap(c.v1TaxSearch(ctx, "/galgame/tags/search", q, "q", "category", "limit"))
		case segs[1] == "multi":
			return c.wrap(c.v1TagMulti(ctx, q))
		case len(segs) == 2:
			return c.wrap(c.v1EntityDetail(ctx, "tags", "tag", q.Get("tag_id"), q))
		}
	case "official":
		switch {
		case len(segs) == 1:
			return c.wrap(c.v1TaxList(ctx, "/galgame/officials", q))
		case segs[1] == "search":
			return c.wrap(c.v1TaxSearch(ctx, "/galgame/officials/search", q, "q", "category", "lang", "limit"))
		case len(segs) == 2:
			return c.wrap(c.v1EntityDetail(ctx, "officials", "official", q.Get("official_id"), q))
		}
	case "engine":
		switch {
		case len(segs) == 1:
			return c.wrap(c.v1EngineList(ctx, q))
		case len(segs) == 2:
			return c.wrap(c.v1EngineDetail(ctx, q.Get("engine_id"), q))
		}
	case "series":
		switch {
		case len(segs) == 1:
			return c.wrap(c.v1TaxList(ctx, "/galgame/series", q))
		case segs[1] == "search":
			return c.wrap(c.v1SeriesSearch(ctx, q))
		case len(segs) == 2:
			return c.wrap(c.v1SeriesDetail(ctx, segs[1], q))
		}
	case "galgame":
		if len(segs) == 3 {
			switch segs[2] {
			case "links":
				return c.wrap(c.v1GalgameLinks(ctx, segs[1]))
			case "aliases":
				return c.wrap(c.v1GalgameAliases(ctx, segs[1]))
			}
		}
	}
	return nil, false, nil
}

// wrap adapts a (data, err) reshaper result to the (data, handled, err) contract
// — every reshaper below OWNS its path, so handled is always true.
func (c *Client) wrap(data json.RawMessage, err error) (json.RawMessage, bool, error) {
	return data, true, err
}

// copyParams copies the named query params (when present) from src into a fresh
// url.Values for forwarding to /v1.
func copyParams(src url.Values, names ...string) url.Values {
	out := url.Values{}
	for _, n := range names {
		if v := src.Get(n); v != "" {
			out.Set(n, v)
		}
	}
	return out
}

// v1TaxList forwards a curated list read (tags / officials / series) — the /v1
// {items,total} envelope IS the shape moyu emits, so the data passes through.
func (c *Client) v1TaxList(ctx context.Context, path string, q url.Values) (json.RawMessage, error) {
	return c.getV1Raw(ctx, path, copyParams(q, "page", "limit", "content_limit"))
}

// v1TaxSearch forwards a curated Meili-backed search read (tags / officials). The
// /v1 {items,total,processing_time_ms} envelope passes through.
func (c *Client) v1TaxSearch(ctx context.Context, path string, q url.Values, params ...string) (json.RawMessage, error) {
	return c.getV1Raw(ctx, path, copyParams(q, params...))
}

// v1TagMulti serves the tag-intersection read: the bridge tag_ids param maps to
// the /v1 ids param; the {items,total} thin-item envelope passes through.
func (c *Client) v1TagMulti(ctx context.Context, q url.Values) (json.RawMessage, error) {
	fq := copyParams(q, "page", "limit", "content_limit")
	if v := q.Get("tag_ids"); v != "" {
		fq.Set("ids", v)
	}
	return c.getV1Raw(ctx, "/galgame/tags/multi", fq)
}

// v1EntityDetail composes a tag/official detail from the by-id entity record
// (/v1/galgame/{kind}/{id}) + the reverse-lookup galgame page
// (/v1/galgame/{kind}/{id}/galgames), reshaping to the bridge
// {<entityKey>: entity, galgames: [...], total} the moyu TaxonomyDetailProxy
// enriches. entityKey is "tag" / "official". idStr comes from the tag_id /
// official_id query param (the :name path segment is cosmetic).
func (c *Client) v1EntityDetail(ctx context.Context, kind, entityKey, idStr string, q url.Values) (json.RawMessage, error) {
	if idStr == "" {
		idStr = "0" // mirror the bridge: a missing/0 id resolves to 404
	}
	entity, err := c.getV1Raw(ctx, "/galgame/"+kind+"/"+idStr, nil)
	if err != nil {
		return nil, err
	}
	rev, err := c.getV1Raw(ctx, "/galgame/"+kind+"/"+idStr+"/galgames",
		copyParams(q, "page", "limit", "content_limit", "sort_field", "sort_order"))
	if err != nil {
		return nil, err
	}
	var revData struct {
		Galgames json.RawMessage `json:"galgames"`
		Total    int64           `json:"total"`
	}
	if err := json.Unmarshal(rev, &revData); err != nil {
		return nil, err
	}
	galgames := revData.Galgames
	if len(galgames) == 0 {
		galgames = json.RawMessage("[]")
	}
	return json.Marshal(map[string]any{
		entityKey:  entity,
		"galgames": galgames,
		"total":    revData.Total,
	})
}

// v1EngineList reconstructs the bridge bare-array engine list (ListAll, cnt DESC)
// by paginating the /v1 engines list (page/limit, capped at 100) and
// concatenating the curated records into a bare array — the shape the moyu FE
// engineList reader expects (data itself is the array).
func (c *Client) v1EngineList(ctx context.Context, q url.Values) (json.RawMessage, error) {
	const pageSize = 100
	items := []json.RawMessage{}
	for page := 1; ; page++ {
		fq := url.Values{}
		fq.Set("page", strconv.Itoa(page))
		fq.Set("limit", strconv.Itoa(pageSize))
		data, err := c.getV1Raw(ctx, "/galgame/engines", fq)
		if err != nil {
			return nil, err
		}
		var env struct {
			Items []json.RawMessage `json:"items"`
		}
		if err := json.Unmarshal(data, &env); err != nil {
			return nil, err
		}
		items = append(items, env.Items...)
		if len(env.Items) < pageSize {
			break
		}
	}
	return json.Marshal(items)
}

// v1EngineDetail composes an engine detail (unconsumed by the moyu FE; kept
// functional). /v1 has no engine reverse-lookup, so the member galgames come
// from a search on engine_ids projected to briefs.
func (c *Client) v1EngineDetail(ctx context.Context, idStr string, q url.Values) (json.RawMessage, error) {
	if idStr == "" {
		idStr = "0"
	}
	engine, err := c.getV1Raw(ctx, "/galgame/engines/"+idStr, nil)
	if err != nil {
		return nil, err
	}
	sq := copyParams(q, "page", "limit", "content_limit")
	sq.Set("engine_ids", idStr)
	sq.Set("include", "meta")
	var sd v1SearchData
	if err := c.getV1(ctx, "/galgame/search", sq, &sd); err != nil {
		return nil, err
	}
	briefs := make([]GalgameBrief, 0, len(sd.Items))
	for i := range sd.Items {
		briefs = append(briefs, v1ItemToBrief(&sd.Items[i]))
	}
	return json.Marshal(map[string]any{
		"engine":   engine,
		"galgames": briefs,
		"total":    sd.Total,
	})
}

// v1SeriesSearch adapts the bridge /series/search (galgame keyword search for
// series assignment; unconsumed by the moyu FE) to a /v1 galgame search,
// returning a bare array of briefs.
func (c *Client) v1SeriesSearch(ctx context.Context, q url.Values) (json.RawMessage, error) {
	sq := url.Values{}
	if kw := q.Get("keywords"); kw != "" {
		sq.Set("q", kw)
	}
	sq.Set("include", "meta")
	var sd v1SearchData
	if err := c.getV1(ctx, "/galgame/search", sq, &sd); err != nil {
		return nil, err
	}
	briefs := make([]GalgameBrief, 0, len(sd.Items))
	for i := range sd.Items {
		briefs = append(briefs, v1ItemToBrief(&sd.Items[i]))
	}
	return json.Marshal(briefs)
}

// v1SeriesDetail forwards the curated /v1 series by-id record (unconsumed by the
// moyu FE). The {id,name,description,galgame_count,galgames} shape passes through.
func (c *Client) v1SeriesDetail(ctx context.Context, idStr string, q url.Values) (json.RawMessage, error) {
	return c.getV1Raw(ctx, "/galgame/series/"+idStr, copyParams(q, "content_limit"))
}

// v1GalgameLinks serves the edit-prefill link read from the /v1 detail's
// include=links block (curated [{id,name,link,source}]). content_limit=all so an
// NSFW entry's links are still returned (the handler already gated visibility).
func (c *Client) v1GalgameLinks(ctx context.Context, gid string) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("include", "links")
	q.Set("content_limit", "all")
	data, err := c.getV1Raw(ctx, "/galgame/"+gid, q)
	if err != nil {
		return nil, err
	}
	var det struct {
		Links json.RawMessage `json:"links"`
	}
	if err := json.Unmarshal(data, &det); err != nil {
		return nil, err
	}
	if len(det.Links) == 0 {
		return json.RawMessage("[]"), nil
	}
	return det.Links, nil
}

// v1GalgameAliases serves the edit-prefill alias read from the /v1 detail's
// top-level aliases block ([]string, always present).
func (c *Client) v1GalgameAliases(ctx context.Context, gid string) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("content_limit", "all")
	data, err := c.getV1Raw(ctx, "/galgame/"+gid, q)
	if err != nil {
		return nil, err
	}
	var det struct {
		Aliases json.RawMessage `json:"aliases"`
	}
	if err := json.Unmarshal(data, &det); err != nil {
		return nil, err
	}
	if len(det.Aliases) == 0 {
		return json.RawMessage("[]"), nil
	}
	return det.Aliases, nil
}
