// Package moemoepoint is a thin client for OAuth's unified-currency
// (moemoepoint) service-to-service API, plus an Awarder that mirrors the
// authoritative balance into the local read-cache.
//
// OAuth is the single source of truth for moemoepoint across all sites
// (kungal / moyu / …). moyu no longer mutates its local balance directly; it
// calls POST /users/:id/moemoepoint (idempotent, OAuth Client Basic Auth — the
// same auth as /users/batch) and keeps user.moemoepoint only as a cache updated
// from each adjust response. See
// kun-galgame-infra/docs/integration/oauth/06-moemoepoint.md.
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
)

const defaultTimeout = 5 * time.Second

// Config configures a Client. BaseURL is the OAuth server base (same as the
// userclient — cfg.OAuth.ServerURL).
type Config struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client
}

// Client calls OAuth's moemoepoint s2s endpoints.
type Client struct {
	baseURL    string
	authHeader string
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
		http:       cfg.HTTPClient,
	}
}

// AdjustRequest is the POST /users/:id/moemoepoint body. source_app is derived
// server-side from the authenticated client, so it's intentionally absent.
type AdjustRequest struct {
	Delta          int    `json:"delta"`            // signed, non-zero, |delta| ≤ 1e6
	Reason         string `json:"reason"`           // content_approved|content_removed|daily_checkin|liked
	Ref            string `json:"ref,omitempty"`    // e.g. "resource:42"
	ActorUserID    int    `json:"actor_user_id"`    // 0 = system
	IdempotencyKey string `json:"idempotency_key"`  // stable per business event
	Note           string `json:"note,omitempty"`
}

// AdjustResult is the data payload of a successful adjust. Applied=false means
// the idempotency key already existed (no double-apply).
type AdjustResult struct {
	UserID  int  `json:"user_id"`
	Balance int  `json:"balance"`
	Applied bool `json:"applied"`
}

// Adjust applies a signed moemoepoint delta to a user. Returns the resulting
// authoritative balance. A non-zero business code (16002/16003/16004/…) or any
// transport error is returned as an error.
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
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth moemoepoint adjust: %w", err)
	}
	defer resp.Body.Close()

	var env struct {
		Code    int          `json:"code"`
		Message string       `json:"message"`
		Data    AdjustResult `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("oauth moemoepoint adjust decode (status=%d): %w", resp.StatusCode, err)
	}
	if env.Code != 0 {
		return nil, fmt.Errorf("oauth moemoepoint adjust: code=%d msg=%s", env.Code, env.Message)
	}
	return &env.Data, nil
}

// Balance reads a user's current authoritative balance
// (GET /users/:id/moemoepoint). Used by the one-time cache-seed cmd.
func (c *Client) Balance(ctx context.Context, userID int) (int, error) {
	u := fmt.Sprintf("%s/users/%d/moemoepoint", c.baseURL, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("oauth moemoepoint balance: %w", err)
	}
	defer resp.Body.Close()

	var env struct {
		Code int `json:"code"`
		Data struct {
			Balance int `json:"balance"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return 0, fmt.Errorf("oauth moemoepoint balance decode (status=%d): %w", resp.StatusCode, err)
	}
	if env.Code != 0 {
		return 0, fmt.Errorf("oauth moemoepoint balance: code=%d", env.Code)
	}
	return env.Data.Balance, nil
}

// LogEntry is one row of the REDUCED (end-user-facing) moemoepoint ledger from
// GET /users/:id/moemoepoint/log. Admin-only fields (note / actor_user_id) are
// deliberately absent — OAuth's s2s view omits them so moderation notes never
// leak to the user.
type LogEntry struct {
	ID        int64  `json:"id"`
	Delta     int    `json:"delta"`
	Reason    string `json:"reason"`
	SourceApp string `json:"source_app"`
	Ref       string `json:"ref"`
	CreatedAt string `json:"created_at"`
}

// Log reads a page of a user's moemoepoint ledger
// (GET /users/:id/moemoepoint/log). Cursor pagination: pass beforeID=0 for the
// newest page, then the last returned entry's ID to fetch older rows. reason is
// an optional filter. Returns the page plus hasMore. The slice is never nil.
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
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("oauth moemoepoint log: %w", err)
	}
	defer resp.Body.Close()

	var env struct {
		Code int `json:"code"`
		Data struct {
			Items   []LogEntry `json:"items"`
			HasMore bool       `json:"has_more"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, false, fmt.Errorf("oauth moemoepoint log decode (status=%d): %w", resp.StatusCode, err)
	}
	if env.Code != 0 {
		return nil, false, fmt.Errorf("oauth moemoepoint log: code=%d", env.Code)
	}
	if env.Data.Items == nil {
		env.Data.Items = []LogEntry{}
	}
	return env.Data.Items, env.Data.HasMore, nil
}
