// S2S client for the NextMoe community primitive (cmd/community, port 9282).
//
// Tenancy is NOT on the wire: community derives the tenant from the calling
// OAuth client's `oauth_clients.community_site`, which is `moyu` here — not
// from `catalog_site`, which stays `kungal` for the catalog claims and is the
// forum's tenant too (see internal/community/anchor).
package communityclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client
}

var (
	ErrNotConfigured = errors.New("communityclient: not configured (empty base URL or credentials)")
	ErrRateLimited   = errors.New("communityclient: rate limited (TL0 sandbox cap)")
	ErrForbidden     = errors.New("communityclient: forbidden (client not bound to a site)")
)

type APIError struct {
	Status int
	Code   int
	Msg    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("communityclient: status=%d code=%d msg=%s", e.Status, e.Code, e.Msg)
}

type Client struct {
	http    *http.Client
	baseURL string
	authHdr string
}

func New(cfg Config) *Client {
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 8 * time.Second}
	}
	c := &Client{
		http:    hc,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
	}
	if cfg.ClientID != "" && cfg.ClientSecret != "" {
		c.authHdr = "Basic " + base64.StdEncoding.EncodeToString([]byte(cfg.ClientID+":"+cfg.ClientSecret))
	}
	return c
}

func (c *Client) Configured() bool { return c != nil && c.baseURL != "" && c.authHdr != "" }

type envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	if !c.Configured() {
		return ErrNotConfigured
	}
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("community request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusTooManyRequests:
		return ErrRateLimited
	}

	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("community decode: %w", err)
	}
	if resp.StatusCode != http.StatusOK || env.Code != 0 {
		return &APIError{Status: resp.StatusCode, Code: env.Code, Msg: env.Message}
	}
	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("community decode data: %w", err)
		}
	}
	return nil
}

func query(pairs map[string]string) string {
	q := url.Values{}
	for k, v := range pairs {
		if v != "" {
			q.Set(k, v)
		}
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func joinInt64(ids []int64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(parts, ",")
}

// viewer renders a viewer_id, and nothing at all for a signed-out reader: the
// upstream faces read 0 as "no viewer" and query() drops an empty value.
func viewer(userID int64) string {
	if userID <= 0 {
		return ""
	}
	return itoa(userID)
}
