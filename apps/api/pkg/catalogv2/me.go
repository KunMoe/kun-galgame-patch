package catalogv2

import (
	"context"
	"net/url"
	"strconv"
)

type proposalRow struct {
	ID string `json:"id"`
}

func (c *Client) MergedProposalTotal(ctx context.Context, uid int) (int64, error) {
	if uid <= 0 {
		return 0, nil
	}
	q := url.Values{
		"proposer_uid":  {strconv.Itoa(uid)},
		"state":         {"merged"},
		"include_total": {"true"},
		"limit":         {"1"},
	}
	var out List[proposalRow]
	if err := c.get(ctx, "/v2/catalog/proposals?"+q.Encode(), &out); err != nil {
		return 0, err
	}
	if out.Total != nil {
		return *out.Total, nil
	}
	return int64(len(out.Items)), nil
}
