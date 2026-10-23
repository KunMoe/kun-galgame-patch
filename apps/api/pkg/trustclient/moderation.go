package trustclient

import (
	"context"
	"encoding/json"
	"net/http"

	"kun-galgame-patch-api/pkg/upstream"
)

type CheckRequest struct {
	Text     string `json:"text"`
	AuthorID *int64 `json:"author_id,omitempty"`
}

type CheckResult struct {
	Decision string   `json:"decision"`
	Matched  []string `json:"matched"`
}

func (c *Client) Check(ctx context.Context, req CheckRequest) (*CheckResult, error) {
	var out CheckResult
	if err := c.postS2S(ctx, "check text", "/api/v1/trust/check", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type ScanRequest struct {
	SubjectKind string `json:"subject_kind"`
	SubjectID   string `json:"subject_id"`
	Text        string `json:"text"`
	AuthorID    *int64 `json:"author_id,omitempty"`
	// Needs trust at or past infra #460: an older trust refuses the unknown
	// field with a 422 on every scan.
	CheckSkipped bool `json:"check_skipped,omitempty"`
}

type ScanResult struct {
	ScanID    int64 `json:"scan_id"`
	Truncated bool  `json:"truncated"`
}

func (c *Client) Scan(ctx context.Context, req ScanRequest) (*ScanResult, error) {
	var out ScanResult
	if err := c.postS2S(ctx, "submit scan", "/api/v1/trust/scan", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) postS2S(ctx context.Context, op, path string, in, out any) error {
	if !c.Configured() {
		return ErrNotConfigured
	}
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	raw, err := c.do(ctx, op, http.MethodPost, path, c.basicAuth, body, s2sKind)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return &upstream.Error{Service: service, Op: op, Kind: upstream.Internal, Cause: err}
	}
	return nil
}
