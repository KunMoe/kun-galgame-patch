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
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"kun-galgame-patch-api/pkg/upstream"
)

const service = "community"

type Config struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client
}

var ErrNotConfigured = errors.New("communityclient: not configured (empty base URL or credentials)")

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

// Community answers every refusal with a generic code — each 403 is 5, each 422
// is 7, each 409 and 429 is 10 (infra community/handler/envelope.go and
// errors.go) — so the message is the only thing that says which rule refused.
const (
	codeNotFound = 4

	// handler/auth.go siteBinding: moyu's OAuth client names no site to act in.
	msgUnbound = "client is not bound to a site"
	// handler/errors.go mapErr: ErrNotAuthor, ErrContentBlocked, SandboxError,
	// and service/writekey.go's ConflictError for a key reused on another body.
	msgNotAuthor      = "not the post author"
	msgContentBlocked = "content blocked by word list"
	msgSandboxContent = "sandbox limit: too many "
	msgKeyReused      = "Idempotency-Key was reused with a different request"
)

// Refusal names the community rule behind a refusal, so a caller can word it
// for the reader.
type Refusal uint8

const (
	RefusalOther Refusal = iota
	RefusalNotAuthor
	RefusalContentBlocked
	RefusalSandbox
	RefusalKeyReused
)

func RefusalOf(err error) Refusal {
	e, ok := upstream.As(err)
	if !ok || e.Service != service {
		return RefusalOther
	}
	switch {
	case e.Detail == msgNotAuthor:
		return RefusalNotAuthor
	case e.Detail == msgContentBlocked:
		return RefusalContentBlocked
	case strings.HasPrefix(e.Detail, msgSandboxContent):
		return RefusalSandbox
	case e.Detail == msgKeyReused:
		return RefusalKeyReused
	}
	return RefusalOther
}

func classify(status int, env envelope) upstream.Kind {
	switch status {
	case http.StatusForbidden:
		if strings.HasPrefix(env.Message, msgUnbound) {
			return upstream.Internal
		}
		return upstream.Rejected
	case http.StatusNotFound:
		// Fiber's route miss is {"code":404,"message":"Cannot GET …"}: moyu is
		// calling a face this community does not have.
		if env.Code == codeNotFound {
			return upstream.NotFound
		}
		return upstream.Internal
	case http.StatusConflict:
		return upstream.Conflict
	case http.StatusUnprocessableEntity:
		// Every other 422 is a field of moyu's own request failing validation.
		if env.Message == msgContentBlocked {
			return upstream.Rejected
		}
		return upstream.Internal
	case http.StatusTooManyRequests:
		// The TL0 sandbox answers 429 for a post with too many links as well as
		// for the daily cap, and waiting fixes only the second.
		if strings.HasPrefix(env.Message, msgSandboxContent) {
			return upstream.Rejected
		}
		return upstream.RateLimited
	}
	return upstream.ByStatus(status)
}

func (c *Client) do(ctx context.Context, op, method, path string, body, out any) error {
	_, err := c.send(ctx, op, method, path, "", body, out)
	return err
}

func (c *Client) send(ctx context.Context, op, method, path, idempotencyKey string, body, out any) (replayed bool, err error) {
	if !c.Configured() {
		return false, &upstream.Error{Service: service, Op: op, Kind: upstream.Unavailable, Cause: ErrNotConfigured}
	}
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return false, &upstream.Error{Service: service, Op: op, Kind: upstream.Internal, Cause: err}
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return false, &upstream.Error{Service: service, Op: op, Kind: upstream.Internal, Cause: err}
	}
	req.Header.Set("Authorization", c.authHdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return false, upstream.Transport(service, op, err)
	}
	defer resp.Body.Close()
	raw, err := upstream.ReadBody(resp.Body)
	if err != nil {
		return false, upstream.Transport(service, op, err)
	}

	var env envelope
	decodeErr := json.Unmarshal(raw, &env)
	fail := &upstream.Error{
		Service: service, Op: op, Status: resp.StatusCode,
		RequestID: resp.Header.Get("X-Request-ID"), RetryAfter: upstream.RetryAfter(resp.Header),
	}
	switch {
	case resp.StatusCode >= 500:
		fail.Kind = upstream.Unavailable
		if decodeErr == nil {
			fail.Code, fail.Detail = strconv.Itoa(env.Code), env.Message
		}
		return false, fail
	case decodeErr != nil:
		fail.Kind, fail.Cause = upstream.Internal, fmt.Errorf("decode envelope: %w", decodeErr)
		return false, fail
	case resp.StatusCode != http.StatusOK || env.Code != 0:
		fail.Code, fail.Detail = strconv.Itoa(env.Code), env.Message
		fail.Kind = classify(resp.StatusCode, env)
		return false, fail
	}
	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			fail.Kind, fail.Cause = upstream.Internal, fmt.Errorf("decode data: %w", err)
			return false, fail
		}
	}
	return resp.Header.Get("Idempotency-Replayed") == "true", nil
}

// LogDegraded records a failure its caller answers without: Internal (moyu's
// own request or credential) at ERROR, anything else at WARN.
func LogDegraded(ctx context.Context, msg string, err error, attrs ...any) {
	level := slog.LevelWarn
	if upstream.KindOf(err) == upstream.Internal {
		level = slog.LevelError
	}
	slog.Log(ctx, level, msg, append(attrs, "error", err)...)
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
