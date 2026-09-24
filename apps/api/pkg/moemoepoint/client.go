package moemoepoint

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"kun-galgame-patch-api/pkg/upstream"
)

const defaultTimeout = 5 * time.Second

// The ledger's house codes (nextmoe-infra pkg/errors/codes.go), as the s2s
// faces in apps/api/internal/platform/ledger/handler/handler.go answer them.
const (
	codeUserNotFound = 10005
	codeIdemConflict = 16004
	codeInsufficient = 16006
)

type Config struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client
}

type Client struct {
	baseURL    string
	authHeader string
	clientID   string
	http       *http.Client
}

func New(cfg Config) *Client {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: defaultTimeout}
	}
	creds := cfg.ClientID + ":" + cfg.ClientSecret
	return &Client{
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		authHeader: "Basic " + base64.StdEncoding.EncodeToString([]byte(creds)),
		clientID:   cfg.ClientID,
		http:       cfg.HTTPClient,
	}
}

type AdjustRequest struct {
	Delta          int    `json:"delta"`
	Reason         string `json:"reason"`
	Ref            string `json:"ref,omitempty"`
	ActorUserID    int    `json:"actor_user_id"`
	IdempotencyKey string `json:"idempotency_key"`
	Note           string `json:"note,omitempty"`
}

type AdjustResult struct {
	UserID  int  `json:"user_id"`
	Balance int  `json:"balance"`
	Applied bool `json:"applied"`
}

func (c *Client) Adjust(ctx context.Context, userID int, r AdjustRequest) (*AdjustResult, error) {
	body, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/users/%d/moemoepoint", c.baseURL, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	var out AdjustResult
	if err := c.do(req, "moemoepoint adjust", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Balance(ctx context.Context, userID int) (int, error) {
	u := fmt.Sprintf("%s/users/%d/moemoepoint", c.baseURL, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, err
	}
	var out struct {
		Balance int `json:"balance"`
	}
	if err := c.do(req, "moemoepoint balance", &out); err != nil {
		return 0, err
	}
	return out.Balance, nil
}

type LogEntry struct {
	ID        int64  `json:"id"`
	Delta     int    `json:"delta"`
	Reason    string `json:"reason"`
	SourceApp string `json:"source_app"`
	Ref       string `json:"ref"`
	CreatedAt string `json:"created_at"`
	IsLocal   bool   `json:"is_local"`
	Link      string `json:"link"`
}

func (c *Client) Log(ctx context.Context, userID, limit int, beforeID int64, reason string) ([]LogEntry, bool, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if beforeID > 0 {
		q.Set("before_id", strconv.FormatInt(beforeID, 10))
	}
	if reason != "" {
		q.Set("reason", reason)
	}
	u := fmt.Sprintf("%s/users/%d/moemoepoint/log", c.baseURL, userID)
	if enc := q.Encode(); enc != "" {
		u += "?" + enc
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, false, err
	}

	var out struct {
		Items   []LogEntry `json:"items"`
		HasMore bool       `json:"has_more"`
	}
	if err := c.do(req, "moemoepoint log", &out); err != nil {
		return nil, false, err
	}
	if out.Items == nil {
		out.Items = []LogEntry{}
	}
	for i := range out.Items {
		out.Items[i].IsLocal = out.Items[i].SourceApp == c.clientID
	}
	return out.Items, out.HasMore, nil
}

func (c *Client) do(req *http.Request, op string, out any) error {
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return upstream.Transport("oauth", op, err)
	}
	defer resp.Body.Close()
	body, err := upstream.ReadBody(resp.Body)
	if err != nil {
		return upstream.Transport("oauth", op, err)
	}

	fail := func(code int, detail string, cause error) error {
		return &upstream.Error{
			Service:    "oauth",
			Op:         op,
			Kind:       ledgerKind(resp.StatusCode, code),
			Status:     resp.StatusCode,
			Code:       strconv.Itoa(code),
			Detail:     detail,
			RequestID:  resp.Header.Get("X-Request-ID"),
			RetryAfter: upstream.RetryAfter(resp.Header),
			Cause:      cause,
		}
	}
	var env struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return fail(0, "", fmt.Errorf("decode envelope: %w", err))
	}
	if resp.StatusCode != http.StatusOK || env.Code != 0 {
		return fail(env.Code, env.Message, nil)
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fail(0, "", fmt.Errorf("decode data: %w", err))
	}
	return nil
}

// ledgerKind keeps a transient failure (Unavailable, RateLimited) apart from a
// permanent one. Of the permanent ones only an unknown user is nobody's bug;
// the rest are moyu's: a 401 is its Basic credential, 16005 an awarder flag
// its client lacks, 16004 a key that already names a different award.
func ledgerKind(status, code int) upstream.Kind {
	switch {
	case status >= 500:
		return upstream.Unavailable
	case status == http.StatusTooManyRequests:
		return upstream.RateLimited
	case code == codeUserNotFound:
		return upstream.NotFound
	case code == codeIdemConflict:
		return upstream.Conflict
	case code == codeInsufficient:
		return upstream.Rejected
	default:
		return upstream.Internal
	}
}
