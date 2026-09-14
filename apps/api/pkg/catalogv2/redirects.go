package catalogv2

import (
	"context"
	"net/url"
	"strconv"
)

// RedirectsMaxLimit is the feed's ceiling. Above it the call is 400
// LIMIT_TOO_LARGE -- the parameter is not clamped.
const RedirectsMaxLimit = 100

type Redirect struct {
	OldID     int64
	CurrentID int64
	MergedAt  string
}

type RedirectPage struct {
	Items []Redirect
	// Empty once the drain has caught up. Opaque: hand it back verbatim.
	NextCursor string
}

type redirectWire struct {
	TargetObject string `json:"target_object"`
	OldID        string `json:"old_id"`
	CurrentID    string `json:"current_id"`
	MergedAt     string `json:"merged_at"`
}

// Redirects drains catalog's merge feed: every id catalog merged away, oldest
// first, beside the id that replaced it. Merging erases the claim naming the
// work in the same transaction, so this feed is the only place the successor is
// written down -- a page whose work was merged has nothing else to follow.
//
// object=work is pinned, and a non-work row is dropped again on arrival rather
// than trusted to stay filtered: the feed answers eight entity families and a
// person id read as a work id folds one page into an unrelated other.
//
// current_id arrives already flattened -- catalog rewrites older rows when the
// target of a merge is itself merged, so no chain reaches a consumer. It
// rewrites them in place without touching merged_at, which means the rewrite
// does not resurface here: a consumer that stores the mapping has to flatten
// its own copy at the same moment or keep a hop that has since gone stale.
func (c *Client) Redirects(ctx context.Context, cursor string, limit int) (RedirectPage, error) {
	if limit <= 0 || limit > RedirectsMaxLimit {
		limit = RedirectsMaxLimit
	}
	q := url.Values{"object": {"work"}, "limit": {strconv.Itoa(limit)}}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	var page List[redirectWire]
	if err := c.get(ctx, "/v2/catalog/redirects?"+q.Encode(), &page); err != nil {
		return RedirectPage{}, err
	}
	out := RedirectPage{Items: make([]Redirect, 0, len(page.Items)), NextCursor: page.Next()}
	for i := range page.Items {
		w := &page.Items[i]
		if w.TargetObject != "" && w.TargetObject != "work" {
			continue
		}
		oldID, ok := ParseID(w.OldID)
		if !ok {
			continue
		}
		currentID, ok := ParseID(w.CurrentID)
		if !ok {
			continue
		}
		out.Items = append(out.Items, Redirect{
			OldID: oldID, CurrentID: currentID, MergedAt: w.MergedAt,
		})
	}
	return out, nil
}
