package trustclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"kun-galgame-patch-api/pkg/upstream"
)

const service = "trust"

type Config struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client
}

type Client struct {
	basicAuth  string
	baseURL    string
	httpClient *http.Client
}

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

func (c *Client) Configured() bool { return c.baseURL != "" && c.basicAuth != "" }

var ErrNotConfigured = errors.New("trustclient: not configured (empty base URL or credentials)")

type ReportRequest struct {
	SubjectKind string `json:"subject_kind"`
	SubjectID   string `json:"subject_id"`
	ReasonKey   string `json:"reason_key"`
	ReporterID  int64  `json:"reporter_id"`
	Note        string `json:"note,omitempty"`
	Snapshot    string `json:"snapshot,omitempty"`
	SubjectURL  string `json:"subject_url,omitempty"`
}

type ReportResult struct {
	ReportID     int64 `json:"report_id"`
	ReviewItemID int64 `json:"review_item_id,omitempty"`
}

type ReasonView struct {
	ID           int64  `json:"id"`
	Key          string `json:"key"`
	NameCN       string `json:"name_cn"`
	Severity     int    `json:"severity"`
	IsDeprecated bool   `json:"is_deprecated"`
	Site         string `json:"site,omitempty"`
}

func (c *Client) ListReportReasons(ctx context.Context) ([]ReasonView, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	const op = "list report reasons"
	raw, err := c.do(ctx, op, http.MethodGet, "/api/v1/trust/report-reasons", c.basicAuth, nil, s2sKind)
	if err != nil {
		return nil, err
	}
	var data struct {
		Reasons []ReasonView `json:"reasons"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, &upstream.Error{Service: service, Op: op, Kind: upstream.Internal, Cause: err}
	}
	return data.Reasons, nil
}

func (c *Client) SubmitReport(ctx context.Context, req ReportRequest) (*ReportResult, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	const op = "submit report"
	raw, err := c.do(ctx, op, http.MethodPost, "/api/v1/trust/reports", c.basicAuth, body, s2sKind)
	if err != nil {
		return nil, err
	}
	var data ReportResult
	if err := json.Unmarshal(raw, &data); err != nil || data.ReportID == 0 {
		return nil, &upstream.Error{Service: service, Op: op, Kind: upstream.Internal, Detail: "undecodable report result", Cause: err}
	}
	return &data, nil
}

// SubjectKind is one entry of the declarative registry face. A nil field is
// left as it is upstream (unset when the kind is created).
type SubjectKind struct {
	Key             string  `json:"key"`
	CallbackURL     *string `json:"callback_url,omitempty"`
	CallbackSecret  *string `json:"callback_secret,omitempty"`
	NotifyOnDismiss *bool   `json:"notify_on_dismiss,omitempty"`
}

type EnsureResult struct {
	Key    string `json:"key"`
	Result string `json:"result"`
}

func (c *Client) EnsureSubjectKinds(ctx context.Context, kinds []SubjectKind) ([]EnsureResult, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	body, err := json.Marshal(struct {
		Kinds []SubjectKind `json:"kinds"`
	}{kinds})
	if err != nil {
		return nil, err
	}
	const op = "ensure subject kinds"
	raw, err := c.do(ctx, op, http.MethodPost, "/api/v1/trust/subject-kinds/ensure", c.basicAuth, body, s2sKind)
	if err != nil {
		return nil, err
	}
	var data struct {
		Results []EnsureResult `json:"results"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, &upstream.Error{Service: service, Op: op, Kind: upstream.Internal, Cause: err}
	}
	return data.Results, nil
}

type kindFunc func(status int, message string) upstream.Kind

// s2sKind reads infra's trust s2s handler (handler/s2s.go mapIntakeErr,
// auth.go siteBinding). One 422 code covers three causes, so the message
// separates them: a subject kind the site never registered is moyu's
// onboarding, while an unknown reason or a bad link is what the reader sent.
func s2sKind(status int, message string) upstream.Kind {
	if status == http.StatusUnprocessableEntity && strings.Contains(message, "subject_kind is not registered") {
		return upstream.Internal
	}
	return upstream.ByStatus(status)
}

// adminKind reads infra's trust admin face (handler/admin.go), which runs on
// the moderator's own token behind JWTAuth + RequirePermission. A 401 there is
// trust failing to verify a token moyu's session still holds, not the session
// ending: mapped to 40100, a verification fault on the trust side logged every
// moderator out of moyu. A 403 is the moderator lacking the queue permission,
// unless infra says the client or token is not bound, which is moyu's setup.
func adminKind(status int, message string) upstream.Kind {
	switch {
	case status == http.StatusForbidden && strings.Contains(message, "not bound"):
		return upstream.Internal
	case status == http.StatusForbidden, status == http.StatusBadRequest:
		return upstream.Rejected
	}
	return upstream.ByStatus(status)
}

func (c *Client) do(
	ctx context.Context, op, method, path, auth string, body []byte, kind kindFunc,
) (json.RawMessage, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", auth)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, upstream.Transport(service, op, err)
	}
	defer resp.Body.Close()
	raw, err := upstream.ReadBody(resp.Body)
	if err != nil {
		return nil, upstream.Transport(service, op, err)
	}

	var env struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	decodeErr := json.Unmarshal(raw, &env)
	e := &upstream.Error{
		Service:    service,
		Op:         op,
		Status:     resp.StatusCode,
		RequestID:  resp.Header.Get("X-Request-ID"),
		RetryAfter: upstream.RetryAfter(resp.Header),
	}
	if resp.StatusCode == http.StatusOK {
		if decodeErr == nil && env.Code == 0 {
			return env.Data, nil
		}
		e.Kind, e.Detail = upstream.Internal, "undecodable success body"
		return nil, e
	}
	if decodeErr != nil || env.Code == 0 {
		e.Kind = upstream.Internal
		if resp.StatusCode >= 500 {
			e.Kind = upstream.Unavailable
		}
		return nil, e
	}
	e.Code, e.Detail = strconv.Itoa(env.Code), env.Message
	e.Kind = kind(resp.StatusCode, env.Message)
	return nil, e
}
