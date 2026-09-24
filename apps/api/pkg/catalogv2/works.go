package catalogv2

import (
	"context"
	"net/url"
	"strconv"
	"strings"
)

const batchIDsLimit = 100

var workDetailInclude = []string{
	"titles", "refs", "credits", "ratings", "tags", "intros", "covers",
	"screenshots", "characters", "companies", "series",
}

// The spoiler ceiling of the tags block, defaulting to none upstream. moyu
// renders the whole vocabulary and lets the reader raise the ceiling in the
// browser, so it always asks for the top one; asking for none silently drops
// eight of work 3's eighty-three tags and leaves the page's 剧透 control dead.
const spoilerMajor = "major"

type WorksQuery struct {
	Q              string
	Sort           string
	Cursor         string
	Page           int
	Limit          int
	TagIDs         string
	OLang          string
	ReleasedAfter  string
	ReleasedBefore string
	CompanyID      int64
	CompanyRollup  bool
	SeriesID       int64
	EngineID       int64
	Facets         []string
	Include        []string
	IncludeTotal   bool
	NSFW           bool
	ContentRating  string
	ContentLimit   string
	IDs            []int64
	Refs           []Ref
	SearchIntro    bool
}

func (c *Client) ListWorks(ctx context.Context, q WorksQuery) (*List[Work], error) {
	v := url.Values{}
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}
	v.Set("limit", strconv.Itoa(limit))
	if q.Q != "" {
		v.Set("q", q.Q)
	}
	if q.Sort != "" {
		v.Set("sort", q.Sort)
	}
	cursor := q.Cursor
	if cursor == "" && q.Page > 1 {
		cursor = PageCursor(q.Page)
	}
	if cursor != "" {
		v.Set("cursor", cursor)
	}
	if q.TagIDs != "" {
		v.Set("tag_id", q.TagIDs)
	}
	if q.OLang != "" {
		v.Set("olang", q.OLang)
	}
	if q.ReleasedAfter != "" {
		v.Set("released_after", q.ReleasedAfter)
	}
	if q.ReleasedBefore != "" {
		v.Set("released_before", q.ReleasedBefore)
	}
	if q.CompanyID > 0 {
		v.Set("company_id", FormatID(q.CompanyID))
		if q.CompanyRollup {
			v.Set("company_rollup", "true")
		}
	}
	if q.SeriesID > 0 {
		v.Set("series_id", FormatID(q.SeriesID))
	}
	if q.EngineID > 0 {
		v.Set("engine_id", FormatID(q.EngineID))
	}
	if len(q.Facets) > 0 {
		v.Set("facets", strings.Join(q.Facets, ","))
	}
	if len(q.Include) > 0 {
		v.Set("include", strings.Join(q.Include, ","))
	}
	if q.IncludeTotal {
		v.Set("include_total", "true")
	}
	if q.NSFW {
		v.Set("nsfw", "true")
	} else {
		v.Set("nsfw", "false")
	}
	if q.ContentRating != "" {
		v.Set("content_rating", q.ContentRating)
	}
	if q.ContentLimit != "" {
		v.Set("content_limit", q.ContentLimit)
	}
	if q.SearchIntro {
		v.Set("search_intro", "true")
	}
	if len(q.IDs) > 0 {
		v.Set("ids", joinIDs(q.IDs))
		v.Set("limit", strconv.Itoa(batchIDsLimit))
	}
	if len(q.Refs) > 0 {
		parts := make([]string, 0, len(q.Refs))
		for _, r := range q.Refs {
			if r.Source != "" && r.ExternalID != "" {
				parts = append(parts, r.Source+":"+r.ExternalID)
			}
		}
		v.Set("refs", strings.Join(parts, ","))
		v.Set("limit", strconv.Itoa(batchIDsLimit))
	}
	var out List[Work]
	if err := c.get(ctx, "/v2/catalog/works?"+v.Encode(), &out); err != nil {
		return nil, err
	}
	if out.Items == nil {
		out.Items = []Work{}
	}
	return &out, nil
}

func (c *Client) GetWork(ctx context.Context, id int64, nsfw bool) (*Work, error) {
	v := url.Values{}
	v.Set("view", "full")
	v.Set("include", strings.Join(workDetailInclude, ","))
	v.Set("spoiler", spoilerMajor)
	if nsfw {
		v.Set("nsfw", "true")
	} else {
		v.Set("nsfw", "false")
	}
	var out Work
	if err := c.get(ctx, "/v2/catalog/works/"+FormatID(id)+"?"+v.Encode(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) WorkByRef(ctx context.Context, source, externalID string, nsfw bool) (*Work, error) {
	page, err := c.ListWorks(ctx, WorksQuery{
		Refs:    []Ref{{Source: source, ExternalID: externalID}},
		NSFW:    nsfw,
		Include: []string{"titles", "refs", "covers"},
	})
	if err != nil {
		return nil, err
	}
	if len(page.Items) == 0 {
		return nil, Absent("GET /v2/catalog/works?refs")
	}
	return &page.Items[0], nil
}

func (c *Client) Calendar(ctx context.Context, month string, nsfw bool, cursor string, limit int) (*List[Work], error) {
	v := url.Values{}
	if month != "" {
		v.Set("month", month)
	}
	v.Set("include", "titles,covers,refs,companies")
	if limit <= 0 {
		limit = 100
	}
	v.Set("limit", strconv.Itoa(limit))
	if cursor != "" {
		v.Set("cursor", cursor)
	}
	if nsfw {
		v.Set("nsfw", "true")
	} else {
		v.Set("nsfw", "false")
	}
	v.Set("include_total", "true")
	var out List[Work]
	if err := c.get(ctx, "/v2/catalog/calendar?"+v.Encode(), &out); err != nil {
		return nil, err
	}
	if out.Items == nil {
		out.Items = []Work{}
	}
	return &out, nil
}
