package trustclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

const adminBase = "/api/v1/admin/trust"

func (c *Client) doAdmin(
	ctx context.Context, op, method, token, path string, query url.Values, body []byte,
) (json.RawMessage, error) {
	if c.baseURL == "" {
		return nil, ErrNotConfigured
	}
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	return c.do(ctx, op, method, path, "Bearer "+token, body, adminKind)
}

func (c *Client) ListReviewItems(ctx context.Context, token string, query url.Values) (json.RawMessage, error) {
	return c.doAdmin(ctx, "list review items", http.MethodGet, token, adminBase+"/review-items", query, nil)
}

func (c *Client) GetReviewItem(ctx context.Context, token string, id int64) (json.RawMessage, error) {
	return c.doAdmin(ctx, "get review item", http.MethodGet, token, fmt.Sprintf("%s/review-items/%d", adminBase, id), nil, nil)
}

func (c *Client) ClaimReviewItem(ctx context.Context, token string, id int64) (json.RawMessage, error) {
	return c.doAdmin(ctx, "claim review item", http.MethodPost, token, fmt.Sprintf("%s/review-items/%d/claim", adminBase, id), nil, nil)
}

func (c *Client) DecideReviewItem(ctx context.Context, token string, id int64, body []byte) (json.RawMessage, error) {
	return c.doAdmin(ctx, "decide review item", http.MethodPost, token, fmt.Sprintf("%s/review-items/%d/decide", adminBase, id), nil, body)
}
