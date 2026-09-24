package catalogv2

import (
	"context"
	"log/slog"
	"net/url"
	"strconv"
)

// ChangesMaxLimit is the feed's ceiling. Above it the call is 400
// LIMIT_TOO_LARGE — the parameter is not clamped.
const ChangesMaxLimit = 100

type Change struct {
	ID        int64
	UpdatedAt string
	Gone      bool
}

type ChangePage struct {
	Items []Change
	// Empty once the drain has caught up. Opaque: hand it back verbatim.
	NextCursor string
}

type changeWire struct {
	TargetObject string `json:"target_object"`
	ID           string `json:"id"`
	UpdatedAt    string `json:"updated_at"`
	Gone         *bool  `json:"gone"`
}

// Changes drains the catalog mirror channel: every write that changes a work's
// claim state, its display axis (display_nsfw / content_rating) or its
// existence bumps updated_at and surfaces the id here, oldest first. An empty
// cursor starts at the beginning of the whole population, so the first drain is
// itself a full inventory — there is no separate "list every id" face and no
// reason to sweep. See docs/catalog/01 §8 at the infra source.
//
// The feed answers galgame works today, but target_object is an enum over
// eight entity families and a person id read as a work id would write another
// row's verdict onto a patch. Non-work rows are dropped here rather than
// trusted to stay absent.
func (c *Client) Changes(ctx context.Context, cursor string, limit int) (ChangePage, error) {
	if limit <= 0 || limit > ChangesMaxLimit {
		limit = ChangesMaxLimit
	}
	q := url.Values{"limit": {strconv.Itoa(limit)}}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	var page List[changeWire]
	if err := c.get(ctx, "/v2/catalog/changes?"+q.Encode(), &page); err != nil {
		return ChangePage{}, err
	}
	out := ChangePage{Items: make([]Change, 0, len(page.Items)), NextCursor: page.Next()}
	var foreign []string
	for i := range page.Items {
		w := &page.Items[i]
		if w.TargetObject != "" && w.TargetObject != "work" {
			foreign = append(foreign, w.TargetObject+":"+w.ID)
			continue
		}
		id, ok := ParseID(w.ID)
		if !ok {
			slog.Warn("catalog changes feed: dropped a row whose id is not a catalog id",
				"id", w.ID, "updated_at", w.UpdatedAt, "cursor", cursor)
			continue
		}
		out.Items = append(out.Items, Change{
			ID: id, UpdatedAt: w.UpdatedAt, Gone: w.Gone != nil && *w.Gone,
		})
	}
	if len(foreign) > 0 {
		slog.Warn("catalog changes feed: dropped rows of another family",
			"count", len(foreign), "rows", foreign, "cursor", cursor)
	}
	return out, nil
}
