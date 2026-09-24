package artifactclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"kun-galgame-patch-api/pkg/artifactclient/gen"
	"kun-galgame-patch-api/pkg/upstream"
)

type (
	InitUploadRequest     = gen.InitUploadRequest
	InitUploadResponse    = gen.InitUploadResponse
	CompleteUploadRequest = gen.CompleteUploadRequest
	ArtifactResponse      = gen.ArtifactResponse
	DownloadResponse      = gen.DownloadResponse
	ResumeUploadResponse  = gen.ResumeUploadResponse
	CompletedPart         = gen.CompletedPart
	UploadedPart          = gen.UploadedPart
	PartURL               = gen.PartURL
	ManifestInput         = gen.ManifestInput
)

const service = "artifact"

// The artifact service's house codes (infra pkg/errors/codes.go 50001-50017).
// Everything not named here is classified by status: its credential and site
// codes arrive as 401/403 and are moyu's own configuration.
const (
	codeNotFound       = 50001
	codeTooBig         = 50004
	codeBadRequest     = 50011
	codeQuotaExceeded  = 50012
	codeUploadDisabled = 50014
	codeSizeMismatch   = 50015
	codeMIMEDenied     = 50017
)

var (
	ErrNotConfigured  = errors.New("artifactclient: not configured (empty base URL or credentials)")
	ErrNotFound       = errors.New("artifactclient: artifact not found")
	ErrTooBig         = errors.New("artifactclient: file exceeds the per-site max size")
	ErrQuotaExceeded  = errors.New("artifactclient: site daily quota exceeded")
	ErrMIMEDenied     = errors.New("artifactclient: file type not allowed for this site")
	ErrUploadDisabled = errors.New("artifactclient: upload disabled")
	ErrSizeMismatch   = errors.New("artifactclient: uploaded size does not match declared size")
)

type Config struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client
}

const (
	callTimeout         = 30 * time.Second
	completeCallTimeout = 90 * time.Second
	// Download runs on the resource page render, which must not wait out a
	// write-sized timeout when the service hangs.
	downloadTimeout = 5 * time.Second
	StatusReady     = 1
)

type Client struct {
	inner *gen.ClientWithResponses
}

func New(cfg Config) *Client {
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{}
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" || cfg.ClientID == "" || cfg.ClientSecret == "" {
		return &Client{}
	}
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(cfg.ClientID+":"+cfg.ClientSecret))
	inner, err := gen.NewClientWithResponses(base,
		gen.WithHTTPClient(hc),
		gen.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Set("Authorization", auth)
			return nil
		}),
	)
	if err != nil {
		slog.Error("artifact client disabled: invalid base URL", "base_url", base, "error", err)
		return &Client{}
	}
	return &Client{inner: inner}
}

func (c *Client) Configured() bool { return c.inner != nil }

func (c *Client) Get(ctx context.Context, uuid string) (*ArtifactResponse, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	resp, err := c.inner.GetArtifactWithResponse(ctx, uuid)
	if err != nil {
		return nil, callErr("get", err)
	}
	if resp.JSON200 != nil && resp.JSON200.Code == 0 && resp.JSON200.Data != nil {
		return resp.JSON200.Data, nil
	}
	return nil, failure("get", resp.HTTPResponse, resp.JSONDefault)
}

func (c *Client) InitUpload(ctx context.Context, req InitUploadRequest) (*InitUploadResponse, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	resp, err := c.inner.InitUploadWithResponse(ctx, req)
	if err != nil {
		return nil, callErr("init upload", err)
	}
	if resp.JSON200 != nil && resp.JSON200.Code == 0 && resp.JSON200.Data != nil {
		return resp.JSON200.Data, nil
	}
	return nil, failure("init upload", resp.HTTPResponse, resp.JSONDefault)
}

func (c *Client) CompleteUpload(ctx context.Context, uuid string, req CompleteUploadRequest) (*ArtifactResponse, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	ctx, cancel := context.WithTimeout(ctx, completeCallTimeout)
	defer cancel()
	resp, err := c.inner.CompleteUploadWithResponse(ctx, uuid, req)
	if err != nil {
		return nil, callErr("complete upload", err)
	}
	if resp.JSON200 != nil && resp.JSON200.Code == 0 && resp.JSON200.Data != nil {
		return resp.JSON200.Data, nil
	}
	return nil, failure("complete upload", resp.HTTPResponse, resp.JSONDefault)
}

func (c *Client) Resume(ctx context.Context, uuid string) (*ResumeUploadResponse, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	resp, err := c.inner.ResumeUploadWithResponse(ctx, uuid)
	if err != nil {
		return nil, callErr("resume upload", err)
	}
	if resp.JSON200 != nil && resp.JSON200.Code == 0 && resp.JSON200.Data != nil {
		return resp.JSON200.Data, nil
	}
	return nil, failure("resume upload", resp.HTTPResponse, resp.JSONDefault)
}

func (c *Client) Download(ctx context.Context, uuid string) (*DownloadResponse, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()
	resp, err := c.inner.DownloadArtifactWithResponse(ctx, uuid)
	if err != nil {
		return nil, callErr("download", err)
	}
	if resp.JSON200 != nil && resp.JSON200.Code == 0 && resp.JSON200.Data != nil {
		return resp.JSON200.Data, nil
	}
	return nil, failure("download", resp.HTTPResponse, resp.JSONDefault)
}

func (c *Client) Delete(ctx context.Context, uuid string) error {
	if !c.Configured() {
		return ErrNotConfigured
	}
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	resp, err := c.inner.DeleteArtifactWithResponse(ctx, uuid)
	if err != nil {
		return callErr("delete", err)
	}
	if resp.JSON200 != nil && resp.JSON200.Code == 0 {
		return nil
	}
	return failure("delete", resp.HTTPResponse, resp.JSONDefault)
}

// callErr separates a body the generated client could not decode, which is a
// contract break on moyu's side, from the call never getting an answer.
func callErr(op string, err error) error {
	var syntax *json.SyntaxError
	var typ *json.UnmarshalTypeError
	if errors.As(err, &syntax) || errors.As(err, &typ) {
		return &upstream.Error{Service: service, Op: op, Kind: upstream.Internal, Cause: err}
	}
	return upstream.Transport(service, op, err)
}

func failure(op string, resp *http.Response, he *gen.HouseError) error {
	e := &upstream.Error{Service: service, Op: op}
	if resp != nil {
		e.Status = resp.StatusCode
		e.RequestID = resp.Header.Get("X-Request-ID")
		e.RetryAfter = upstream.RetryAfter(resp.Header)
	}
	if he == nil || he.Code == 0 {
		e.Kind = upstream.Internal
		if e.Status >= 500 {
			e.Kind = upstream.Unavailable
		}
		return e
	}
	e.Code = strconv.FormatInt(he.Code, 10)
	e.Detail = he.Message
	switch he.Code {
	case codeNotFound:
		e.Kind, e.Cause = upstream.NotFound, ErrNotFound
	case codeTooBig:
		e.Kind, e.Cause = upstream.Rejected, ErrTooBig
	case codeMIMEDenied:
		e.Kind, e.Cause = upstream.Rejected, ErrMIMEDenied
	case codeSizeMismatch:
		e.Kind, e.Cause = upstream.Rejected, ErrSizeMismatch
	case codeQuotaExceeded:
		e.Kind, e.Cause = upstream.RateLimited, ErrQuotaExceeded
	case codeUploadDisabled:
		e.Kind, e.Cause = upstream.Unavailable, ErrUploadDisabled
	case codeBadRequest:
		// 409 is resume on an upload that already completed or failed; a 400 is
		// the part list or file metadata the reader's browser sent.
		e.Kind = upstream.Rejected
		if e.Status == http.StatusConflict {
			e.Kind = upstream.Conflict
		}
	default:
		e.Kind = upstream.ByStatus(e.Status)
	}
	return e
}
