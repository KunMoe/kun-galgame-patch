package client

import (
	"context"
	"net/url"
	"sort"

	"kun-galgame-patch-api/pkg/catalogv2"
)

// company_rollup only works on catalog's SQL lane, and that lane is keyset-only:
// sort=released_desc — or any facets= — flips /v2/catalog/works to the search
// lane, whose filter has no rollup field and drops the parameter without saying
// so. VISUAL ARTS answered 19 works instead of the 540 it publishes through Key,
// SAGA PLANETS and the rest. Walking the whole rollup here is what buys back the
// numbered pages and the release-date order the company page has always had.
const (
	catalogRollupPageSize = 100
	catalogRollupWorksMax = 2000
)

type companyRoster struct {
	members []GalgameBrief
	total   int64
	own     int64
	imprint int64
}

func (c *Client) companyMembers(ctx context.Context, id int64, q url.Values, gate catalogGate) (*companyRoster, error) {
	works, walked, err := c.companyRollupWorks(ctx, id, gate)
	if err != nil {
		return nil, err
	}
	if !walked {
		members, total, err := c.taxonomyMembers(ctx, "company_id", id, q, gate)
		if err != nil {
			return nil, err
		}
		return &companyRoster{members: members, total: total, own: total}, nil
	}

	rows := make([]catalogWorkListItem, 0, len(works))
	var imprint int64
	for i := range works {
		it := workToListItem(works[i])
		if !it.ClaimedBy.renderable() || it.publicGID() == 0 {
			continue
		}
		rows = append(rows, it)
		if works[i].ViaCompany != nil {
			imprint++
		}
	}
	order := make([]int, len(rows))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		x, y := &rows[order[a]], &rows[order[b]]
		xd, yd := strOrEmpty(x.ReleaseDate), strOrEmpty(y.ReleaseDate)
		if xd != yd {
			return xd > yd
		}
		return x.ID > y.ID
	})

	page, limit := taxonomyPageWindow(q)
	total := int64(len(order))
	start := (page - 1) * limit
	members := make([]GalgameBrief, 0, limit)
	shown := make([]int64, 0, limit)
	for i := start; i < len(order) && i < start+limit; i++ {
		it := rows[order[i]]
		members = append(members, catalogItemToBrief(&it))
		shown = append(shown, it.ID)
	}
	facets := c.facetsByWorkID(ctx, shown, gate)
	for i := range members {
		members[i].Facet = facets[shown[i]]
	}
	return &companyRoster{members: members, total: total, own: total - imprint, imprint: imprint}, nil
}

// walked is false when the rollup is larger than this page can honestly hold; the
// caller then serves the direct attributions the paged face can answer.
func (c *Client) companyRollupWorks(ctx context.Context, id int64, gate catalogGate) (works []catalogv2.Work, walked bool, err error) {
	cursor := ""
	for {
		data, err := c.v2.ListWorks(ctx, catalogv2.WorksQuery{
			CompanyID: id, CompanyRollup: true, Cursor: cursor,
			Limit: catalogRollupPageSize, NSFW: true, IncludeTotal: cursor == "",
			Include: cardInclude, ContentLimit: gate.contentLimit,
		})
		if err != nil {
			return nil, false, err
		}
		if cursor == "" && data.Count() > catalogRollupWorksMax {
			return nil, false, nil
		}
		works = append(works, data.Items...)
		cursor = data.Next()
		if cursor == "" || len(data.Items) == 0 || len(works) >= catalogRollupWorksMax {
			return works, true, nil
		}
	}
}
