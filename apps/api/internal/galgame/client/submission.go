package client

import (
	"context"

	"kun-galgame-patch-api/pkg/catalogv2"
)

func (c *Client) SearchPublishItems(ctx context.Context, q string, limit int) ([]GalgameHit, int64, error) {
	page, err := c.v2.ListWorks(ctx, catalogv2.WorksQuery{
		Q: q, Limit: limit, NSFW: true,
		Include: cardInclude, IncludeTotal: true,
		Facets: []string{"olang"},
	})
	if err != nil {
		return nil, 0, err
	}
	items := make([]GalgameHit, 0, len(page.Items))
	for i := range page.Items {
		it := workToListItem(page.Items[i])
		if !it.ClaimedBy.renderable() || it.publicGID() == 0 {
			continue
		}
		items = append(items, catalogItemToHit(&it))
	}
	return items, page.Count(), nil
}
