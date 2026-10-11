package catalogv2

import (
	"context"
	"net/http"
	"strconv"
)

const (
	TagBlockTree  = "tree"
	TagBlockExact = "exact"
)

type TagBlock struct {
	TagID     int64  `json:"tag_id"`
	Mode      string `json:"mode"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type tagBlockWire struct {
	TagID     string `json:"tag_id"`
	Mode      string `json:"mode"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func (w tagBlockWire) view() TagBlock {
	return TagBlock{TagID: parseIDOrZero(w.TagID), Mode: w.Mode, CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt}
}

func tagBlockPath(tagID int64) string {
	return "/v2/me/blocked-tags/" + strconv.FormatInt(tagID, 10)
}

// One page holds the whole list: catalog caps an account at TagExclusionMax
// blocks, the same cap as the exclusion parameters.
func (c *Client) MyTagBlocks(ctx context.Context, accessToken string) ([]TagBlock, error) {
	var page List[tagBlockWire]
	path := "/v2/me/blocked-tags?limit=" + strconv.Itoa(TagExclusionMax)
	if _, err := c.userDo(ctx, http.MethodGet, path, accessToken, nil, &page); err != nil {
		return nil, err
	}
	out := make([]TagBlock, 0, len(page.Items))
	for _, row := range page.Items {
		if b := row.view(); b.TagID > 0 {
			out = append(out, b)
		}
	}
	return out, nil
}

func (c *Client) MyTagBlock(ctx context.Context, accessToken string, tagID int64) (*TagBlock, error) {
	var row tagBlockWire
	if _, err := c.userDo(ctx, http.MethodGet, tagBlockPath(tagID), accessToken, nil, &row); err != nil {
		return nil, err
	}
	b := row.view()
	return &b, nil
}

func (c *Client) PutTagBlock(ctx context.Context, accessToken string, tagID int64, mode string) (*TagBlock, error) {
	body := map[string]string{"mode": mode}
	var row tagBlockWire
	if _, err := c.userDo(ctx, http.MethodPut, tagBlockPath(tagID), accessToken, body, &row); err != nil {
		return nil, err
	}
	b := row.view()
	return &b, nil
}

func (c *Client) DeleteTagBlock(ctx context.Context, accessToken string, tagID int64) error {
	_, err := c.userDo(ctx, http.MethodDelete, tagBlockPath(tagID), accessToken, nil, nil)
	return err
}
