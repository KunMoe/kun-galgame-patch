// Package trustclient is a thin SDK for the infra Trust & Safety service
// (kun-galgame-infra, port 9283): report intake (Phase 1) + the moderation admin
// inbox (Phase 3). S2S report submission uses HTTP Basic with an OAuth
// client_id/secret — the trust service reads oauth_clients.catalog_site to
// derive moyu's site, so the site is never on the wire. There is no generated
// client for trust, so the calls are hand-written against the committed contract
// (kun-galgame-infra/docs/trust/openapi.yaml), reusing the same
// {code,message,data} house envelope as the other infra services.
package trustclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Config bundles connection settings (created in app.go from config). The Basic
// credentials are moyu's OAuth client_id/secret.
type Config struct {
	BaseURL      string // trust service base, e.g. http://127.0.0.1:9283 (no trailing slash)
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client // optional; defaults to a 15s-timeout client
}

// Client is a thin wrapper over the trust HTTP API.
type Client struct {
	basicAuth  string
	baseURL    string
	httpClient *http.Client
}

// New constructs a Client. Empty BaseURL/credentials = a no-op client whose
// calls return ErrNotConfigured, so moyu degrades gracefully when the trust
// service isn't wired (dev, or before infra onboarding).
func New(cfg Config) *Client {
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	var ba string
	if cfg.ClientID != "" && cfg.ClientSecret != "" {
		ba = "Basic " + base64.StdEncoding.EncodeToString([]byte(cfg.ClientID+":"+cfg.ClientSecret))
	}
	return &Client{
		basicAuth:  ba,
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		httpClient: hc,
	}
}

// Configured reports whether the client can reach the trust service S2S.
func (c *Client) Configured() bool { return c.baseURL != "" && c.basicAuth != "" }

// Sentinel errors callers can errors.Is against, so the BFF can map them to the
// right HTTP status for the browser.
var (
	ErrNotConfigured = errors.New("trustclient: not configured (empty base URL or credentials)")
	ErrValidation    = errors.New("trustclient: report rejected (unregistered subject kind or unknown reason)")
	ErrRateLimited   = errors.New("trustclient: reporter rate limit exceeded")
	ErrForbidden     = errors.New("trustclient: client not bound to a site")
	ErrUnauthorized  = errors.New("trustclient: unauthorized (check client_id/secret)")
)

// ReportRequest mirrors the trust service's dto.ReportRequest. subject_id is a
// STRING (stringify numeric ids); site is derived server-side, never sent.
type ReportRequest struct {
	SubjectKind string `json:"subject_kind"`
	SubjectID   string `json:"subject_id"`
	ReasonKey   string `json:"reason_key"`
	ReporterID  int64  `json:"reporter_id"`
	Note        string `json:"note,omitempty"`
	Snapshot    string `json:"snapshot,omitempty"`
	// SubjectURL is the absolute deep-link to the reported content, so the
	// moderator console can jump straight into context. Must be absolute
	// http(s), ≤512 chars, or trust rejects the report (422).
	SubjectURL string `json:"subject_url,omitempty"`
}

// ReportResult mirrors dto.ReportResponse. ReviewItemID is 0 when the report
// stayed below the aggregate threshold (no review item opened).
type ReportResult struct {
	ReportID     int64 `json:"report_id"`
	ReviewItemID int64 `json:"review_item_id,omitempty"`
}

// ReasonView mirrors the trust service's ReasonView (a usable report reason for
// the calling site — global base + this site's extensions, non-deprecated).
type ReasonView struct {
	ID           int64  `json:"id"`
	Key          string `json:"key"`
	NameCN       string `json:"name_cn"`
	Severity     int    `json:"severity"`
	IsDeprecated bool   `json:"is_deprecated"`
	Site         string `json:"site,omitempty"`
}

// ListReportReasons returns the reasons moyu's report UI may offer, resolved
// server-side from the client's site binding (S2S Basic auth).
func (c *Client) ListReportReasons(ctx context.Context) ([]ReasonView, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, c.baseURL+"/api/v1/trust/report-reasons", nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.basicAuth)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var env struct {
		Code int `json:"code"`
		Data struct {
			Reasons []ReasonView `json:"reasons"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK || env.Code != 0 {
		return nil, fmt.Errorf("trustclient: list reasons failed (status %d, code %d)", resp.StatusCode, env.Code)
	}
	return env.Data.Reasons, nil
}

// SubmitReport files a report against (subject_kind, subject_id) on moyu's site.
// Dedup + rate-limit + weighting happen server-side.
func (c *Client) SubmitReport(ctx context.Context, req ReportRequest) (*ReportResult, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL+"/api/v1/trust/reports", bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", c.basicAuth)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var env struct {
		Code    int           `json:"code"`
		Message string        `json:"message"`
		Data    *ReportResult `json:"data"`
	}
	_ = json.Unmarshal(raw, &env)

	if resp.StatusCode == http.StatusOK && env.Code == 0 && env.Data != nil {
		return env.Data, nil
	}
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return nil, fmt.Errorf("%w: %s", ErrRateLimited, env.Message)
	case http.StatusUnprocessableEntity:
		return nil, fmt.Errorf("%w: %s", ErrValidation, env.Message)
	case http.StatusForbidden:
		return nil, ErrForbidden
	case http.StatusUnauthorized:
		return nil, ErrUnauthorized
	}
	return nil, fmt.Errorf(
		"trustclient: report failed (status %d, code %d): %s", resp.StatusCode, env.Code, env.Message,
	)
}
