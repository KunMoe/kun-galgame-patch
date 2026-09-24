package client

import (
	"context"

	"kun-galgame-patch-api/pkg/catalogv2"
)

// SearchGalgameIDs answers "which games does this keyword name". The site
// search's resource lane needs it because a keyword reaches a patch resource
// two ways — through the note its uploader wrote, and through the title of the
// game it hangs off — and moyu stores no titles of its own.
//
// No include=: the caller reads nothing but the id and the claim, so the bare
// id+name rows a v2 read answers by default are the whole payload. Adding a
// block here buys nothing and makes the heaviest face on catalog answer twice
// per search.
func (c *Client) SearchGalgameIDs(ctx context.Context, q, contentLimit string, limit int) ([]int, error) {
	page, err := c.v2.ListWorks(ctx, catalogv2.WorksQuery{
		Q:            q,
		Limit:        limit,
		NSFW:         true,
		ContentLimit: gateFor(contentLimit).contentLimit,
	})
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(page.Items))
	for i := range page.Items {
		it := workToListItem(page.Items[i])
		if !it.ClaimedBy.renderable() {
			continue
		}
		if gid := it.publicGID(); gid > 0 {
			ids = append(ids, gid)
		}
	}
	return ids, nil
}
