package catalogv2

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"kun-galgame-patch-api/pkg/upstream"

	"github.com/redis/go-redis/v9"
)

var (
	ErrNotConfigured = errors.New("catalogv2: not configured")
	ErrNoAccessToken = errors.New("catalogv2: no access token on the session")
)

type Client struct {
	http   *http.Client
	origin string
	apiKey string
	rdb    *redis.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		http: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		origin: origin(baseURL),
		apiKey: strings.TrimSpace(apiKey),
	}
}

func origin(raw string) string {
	u := strings.TrimRight(strings.TrimSpace(raw), "/")
	for _, suf := range []string{"/api/v1", "/v2", "/v1"} {
		if strings.HasSuffix(u, suf) {
			return strings.TrimSuffix(u, suf)
		}
	}
	return u
}

func (c *Client) Configured() bool {
	return c != nil && c.origin != "" && c.apiKey != ""
}

// Every S2S read goes through here, so this is where the shared read cache
// belongs; user-token reads take userDo and are never cached. Decoding into a
// json.RawMessage is what makes one body serve both the cache and the caller:
// send() hands the verbatim bytes back without a second request, and a 204 or
// an empty body leaves raw nil, exactly as before.
func (c *Client) get(ctx context.Context, path string, out any) error {
	if body, ok := c.cacheGet(ctx, path); ok {
		return json.Unmarshal(body, out)
	}
	var raw json.RawMessage
	if _, err := c.do(ctx, http.MethodGet, path, "", "", nil, &raw); err != nil {
		return err
	}
	if len(raw) == 0 {
		return nil
	}
	c.cacheSet(ctx, path, raw)
	return json.Unmarshal(raw, out)
}

type call struct {
	method  string
	path    string
	token   string
	ifMatch string
	idemKey string
	body    []byte
}

func (c *Client) do(ctx context.Context, method, path, userToken, ifMatch string, body any, out any) (string, error) {
	var raw []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return "", err
		}
		raw = b
	}
	return c.send(ctx, call{method: method, path: path, token: userToken, ifMatch: ifMatch, body: raw}, out)
}

func (c *Client) send(ctx context.Context, in call, out any) (string, error) {
	if !c.Configured() {
		return "", ErrNotConfigured
	}
	route, _, _ := strings.Cut(in.path, "?")
	op := in.method + " " + route
	var rdr io.Reader
	if in.body != nil {
		rdr = bytes.NewReader(in.body)
	}
	req, err := http.NewRequestWithContext(ctx, in.method, c.origin+in.path, rdr)
	if err != nil {
		return "", err
	}
	bearer := c.apiKey
	if in.token != "" {
		bearer = in.token
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Accept", "application/json, application/problem+json")
	if in.ifMatch != "" {
		req.Header.Set("If-Match", in.ifMatch)
	}
	if in.idemKey != "" {
		req.Header.Set("Idempotency-Key", in.idemKey)
	}
	if in.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", upstream.Transport(service, op, err)
	}
	defer resp.Body.Close()
	raw, err := upstream.ReadBody(resp.Body)
	if err != nil {
		return "", upstream.Transport(service, op, err)
	}
	etag := strings.TrimSpace(resp.Header.Get("ETag"))
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotModified {
		return etag, nil
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if out == nil || len(raw) == 0 || string(raw) == "null" {
			return etag, nil
		}
		if err := json.Unmarshal(raw, out); err != nil {
			return etag, &upstream.Error{Service: service, Op: op, Kind: upstream.Internal,
				Status: resp.StatusCode, RequestID: resp.Header.Get("X-Request-ID"), Cause: err}
		}
		return etag, nil
	}
	return etag, failure(op, in.method, in.token != "", resp, raw)
}

func (c *Client) userDo(ctx context.Context, method, path, accessToken string, body, out any) (string, error) {
	if accessToken == "" {
		return "", ErrNoAccessToken
	}
	return c.do(ctx, method, path, accessToken, "", body, out)
}

// userPost is every POST that creates something. Catalog replays its first
// answer to a repeated key for 24h, refusals included, so the key has to name
// one attempt at the write rather than its body: keyed on the body, a folder
// deleted and made again came back as the deleted one.
func (c *Client) userPost(ctx context.Context, path, accessToken, idemKey string, body, out any) error {
	if accessToken == "" {
		return ErrNoAccessToken
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	_, err = c.send(ctx, call{
		method: http.MethodPost, path: path, token: accessToken, idemKey: idemKey, body: raw,
	}, out)
	return err
}
