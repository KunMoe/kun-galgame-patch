package client

import (
	"context"
	"fmt"

	"kun-galgame-patch-api/pkg/catalogv2"
)

// DisplayVerdict is one catalog work's display axis, already keyed by the id
// moyu's patch table uses — which is the catalog id itself since migration 037.
// This used to read the claim and then fall back to scanning the work's
// `curated` anchor, carrying a comment that publicGID's own fallback could not
// be used here because "the two id spaces overlap and only 9% of moyu's own
// rows have equal values". There is one id space now.
type DisplayVerdict struct {
	GID          int
	ContentLimit string
}

// DisplayVerdictsByCatalogIDs hydrates ids taken from the catalog changes feed.
//
// Both gates are open on purpose — nsfw=true and no content_limit. Sending the
// reader's gate here would hide exactly the works this is meant to mark nsfw,
// and they would stay NULL and keep passing the list predicate forever.
func (c *Client) DisplayVerdictsByCatalogIDs(ctx context.Context, ids []int64) ([]DisplayVerdict, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) > CatalogWorksIDsMax {
		return nil, fmt.Errorf(
			"DisplayVerdictsByCatalogIDs: %d ids exceeds the %d-id ceiling — chunk by client.CatalogWorksIDsMax",
			len(ids), CatalogWorksIDsMax,
		)
	}
	page, err := c.v2.ListWorks(ctx, catalogv2.WorksQuery{
		IDs: ids, NSFW: true, Limit: CatalogWorksIDsMax,
	})
	if err != nil {
		return nil, catalogErr(err)
	}
	out := make([]DisplayVerdict, 0, len(page.Items))
	for i := range page.Items {
		it := workToListItem(page.Items[i])
		if it.ID <= 0 {
			continue
		}
		cl, _ := contentAxisOf(it.ClaimedBy, it.ContentRating)
		out = append(out, DisplayVerdict{GID: int(it.ID), ContentLimit: cl})
	}
	return out, nil
}

