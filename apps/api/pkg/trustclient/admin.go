package trustclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Admin inbox proxy (Phase 3). These call the trust service's
// /api/v1/admin/trust/* face, which is gated by a USER JWT + the
// `trust.queue_access` permission — so the BFF forwards the moderator's own
// OAuth access token as Bearer (NOT the S2S Basic client creds). The trust
// service lifts the operator id from the token, so decisions are attributed to
// the real moderator. Responses are returned as the raw `data` payload
// (passthrough) — the browser owns the review-item / report shapes.

// AdminError carries the trust service's house code + HTTP status so the BFF can
// map a claim conflict (409) etc. to the right response.
type AdminError struct {
	Status  int
	Code    int
	Message string
}

func (e *AdminError) Error() string {
	return fmt.Sprintf("trustclient admin: status %d code %d: %s", e.Status, e.Code, e.Message)
}

// doAdmin performs a Bearer-authed admin request and returns the raw `data`
// bytes of the house envelope on success.
func (c *Client) doAdmin(
	ctx context.Context, method, token, path string, query url.Values, body []byte,
) (json.RawMessage, error) {
	if c.baseURL == "" {
		return nil, ErrNotConfigured
	}
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var env struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	_ = json.Unmarshal(raw, &env)

	if resp.StatusCode == http.StatusOK && env.Code == 0 {
		return env.Data, nil
	}
	return nil, &AdminError{Status: resp.StatusCode, Code: env.Code, Message: env.Message}
}

const adminBase = "/api/v1/admin/trust"

// ListReviewItems returns the moderation inbox page (raw data passthrough).
func (c *Client) ListReviewItems(ctx context.Context, token string, query url.Values) (json.RawMessage, error) {
	return c.doAdmin(ctx, http.MethodGet, token, adminBase+"/review-items", query, nil)
}

// GetReviewItem returns one review item + its reports (raw data passthrough).
func (c *Client) GetReviewItem(ctx context.Context, token string, id int64) (json.RawMessage, error) {
	return c.doAdmin(ctx, http.MethodGet, token, fmt.Sprintf("%s/review-items/%d", adminBase, id), nil, nil)
}

// ClaimReviewItem claims a pending item for the calling moderator (409 if taken).
func (c *Client) ClaimReviewItem(ctx context.Context, token string, id int64) (json.RawMessage, error) {
	return c.doAdmin(ctx, http.MethodPost, token, fmt.Sprintf("%s/review-items/%d/claim", adminBase, id), nil, nil)
}

// DecideReviewItem records a dismiss/action decision (body = DecideRequest).
func (c *Client) DecideReviewItem(ctx context.Context, token string, id int64, body []byte) (json.RawMessage, error) {
	return c.doAdmin(ctx, http.MethodPost, token, fmt.Sprintf("%s/review-items/%d/decide", adminBase, id), nil, body)
}
