package catalogv2

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

type ClaimEvent struct {
	ID            int64
	WorkID        int64
	FromState     *string
	ToState       string
	Reason        *string
	ActorUID      int64
	Site          string
	ProductWorkID *int64
}

type claimEventWire struct {
	ID            string  `json:"id"`
	WorkID        string  `json:"work_id"`
	FromState     *string `json:"from_state"`
	ToState       string  `json:"to_state"`
	Reason        *string `json:"reason"`
	ActorUID      string  `json:"actor_uid"`
	Site          string  `json:"site"`
	ProductWorkID *string `json:"product_work_id"`
}

// A consumer resumes from the event id it last committed, so the cursor is
// minted from that watermark instead of echoing a previous next_cursor: the
// watermark advances inside the per-event transaction and no page boundary
// survives a restart. sort=recorded_asc is the walk catalog documents for it.
// The application key needs claim_events:read on top of catalog:read; an
// operator grants it, and without it this answers 403, not an empty page.
//
// A row whose id or work_id does not parse ends the page with an error, after
// the rows before it. Skipping it lost it for good: the next event's id moved
// the watermark past it, and nothing reads an event twice.
func (c *Client) ClaimEvents(ctx context.Context, since int64, limit int, site string) ([]ClaimEvent, error) {
	q := url.Values{"sort": {"recorded_asc"}, "limit": {strconv.Itoa(limit)}}
	if since > 0 {
		q.Set("cursor", EncodeCursor(strconv.FormatInt(since, 10)))
	}
	return c.claimEvents(ctx, q, site)
}

// The newest event id, for a consumer seeding a watermark it will not backfill
// behind.
func (c *Client) ClaimEventHead(ctx context.Context, site string) (int64, error) {
	rows, err := c.claimEvents(ctx, url.Values{"sort": {"recorded_desc"}, "limit": {"1"}}, site)
	if err != nil || len(rows) == 0 {
		return 0, err
	}
	return rows[0].ID, nil
}

func (c *Client) claimEvents(ctx context.Context, q url.Values, site string) ([]ClaimEvent, error) {
	if site != "" {
		q.Set("site", site)
	}
	var page List[claimEventWire]
	if err := c.get(ctx, "/v2/catalog/claim-events?"+q.Encode(), &page); err != nil {
		return nil, err
	}
	out := make([]ClaimEvent, 0, len(page.Items))
	for i := range page.Items {
		w := &page.Items[i]
		id, ok := ParseID(w.ID)
		if !ok {
			return out, fmt.Errorf("claim event id %q is not a catalog id; the watermark holds before it", w.ID)
		}
		workID, ok := ParseID(w.WorkID)
		if !ok {
			return out, fmt.Errorf("claim event %d names work_id %q, which is not a catalog id; the watermark holds before it", id, w.WorkID)
		}
		actor, _ := ParseID(w.ActorUID)
		ev := ClaimEvent{
			ID: id, WorkID: workID, FromState: w.FromState, ToState: w.ToState,
			Reason: w.Reason, ActorUID: actor, Site: w.Site,
		}
		if w.ProductWorkID != nil {
			if gid, valid := ParseID(*w.ProductWorkID); valid {
				ev.ProductWorkID = &gid
			}
		}
		out = append(out, ev)
	}
	return out, nil
}
