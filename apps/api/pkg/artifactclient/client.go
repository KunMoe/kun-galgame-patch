// Package artifactclient is a thin SDK for the centralized artifact service
// (kun-galgame-infra, port 9279): a private-bucket large-file upload/download
// platform. The browser PUTs bytes straight to B2 via presigned URLs; this
// client only drives the small init/complete/download/delete JSON calls (backend
// S2S, HTTP Basic with an OAuth client_id/secret — the artifact service shares
// the OAuth `oauth_client` table as its "site" registry, gated by
// artifact_enabled + artifact_site_key on the infra side).
//
// The low-level typed client (sub-package gen) is generated from the committed
// OpenAPI contract; this file wraps it with auth, no-op-when-disabled behaviour,
// re-exported contract types, and sentinel errors. See
// kun-galgame-infra/docs/artifact/06,10.
package artifactclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"kun-galgame-patch-api/pkg/artifactclient/gen"
)

// Re-exported contract types (generated in the gen sub-package) so callers
// depend only on this package.
type (
	InitUploadRequest     = gen.InitUploadRequest
	InitUploadResponse    = gen.InitUploadResponse
	CompleteUploadRequest = gen.CompleteUploadRequest
	ArtifactResponse      = gen.ArtifactResponse
	DownloadResponse      = gen.DownloadResponse
	CompletedPart         = gen.CompletedPart
	PartURL               = gen.PartURL
	ManifestInput         = gen.ManifestInput
)

// UploadedPart is a part already stored in B2 for an in-progress upload. Resume
// returns these so the client can skip re-uploading them (reuse the etag).
type UploadedPart struct {
	PartNumber int32  `json:"part_number"`
	Etag       string `json:"etag"`
	Size       int64  `json:"size"`
}

// ResumeUploadResponse mirrors the artifact service's resume response: the parts
// already stored (UploadedParts — skip them) plus fresh presigned URLs for the
// parts still missing (PartUrls). A single-part upload comes back with UploadUrl
// to re-PUT the whole file. Field shapes match gen.InitUploadResponse where they
// overlap. Hand-written (not generated) until the contract is re-synced; see the
// note in Resume below.
type ResumeUploadResponse struct {
	Uuid          string          `json:"uuid"`
	Multipart     bool            `json:"multipart"`
	ExpiresAt     string          `json:"expires_at"`
	PartSize      *int64          `json:"part_size,omitempty"`
	PartUrls      *[]gen.PartURL  `json:"part_urls,omitempty"`
	UploadUrl     *string         `json:"upload_url,omitempty"`
	UploadedParts *[]UploadedPart `json:"uploaded_parts,omitempty"`
}

// House error codes returned by the artifact service (see infra pkg/errors).
const (
	codeArtifactNotFound       = 50001
	codeArtifactTooBig         = 50004
	codeArtifactUnauthorized   = 50006
	codeArtifactQuotaExceeded  = 50012
	codeArtifactUploadDisabled = 50014
	codeArtifactSizeMismatch   = 50015
	codeArtifactMIMEDenied     = 50017
)

// Sentinel errors callers can errors.Is against.
var (
	ErrNotConfigured  = errors.New("artifactclient: not configured (empty base URL or credentials)")
	ErrUnauthorized   = errors.New("artifactclient: unauthorized (check client_id/secret + artifact_enabled)")
	ErrTooBig         = errors.New("artifactclient: file exceeds the per-site max size")
	ErrQuotaExceeded  = errors.New("artifactclient: daily quota exceeded")
	ErrMIMEDenied     = errors.New("artifactclient: file type not allowed for this site")
	ErrNotFound       = errors.New("artifactclient: artifact not found")
	ErrUploadDisabled = errors.New("artifactclient: upload disabled")
	ErrSizeMismatch   = errors.New("artifactclient: uploaded size does not match declared size")
)

// Config bundles connection settings (created in app.go from config).
type Config struct {
	BaseURL      string // e.g. http://127.0.0.1:9279 (no trailing slash)
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client // optional; defaults to a 30s-timeout client
}

// Client is a thin singleton-friendly wrapper around the generated client.
type Client struct {
	inner     *gen.ClientWithResponses
	basicAuth string
	// baseURL + httpClient back the hand-written Resume call (the generated
	// client predates the /resume endpoint); identical to what `inner` uses.
	baseURL    string
	httpClient *http.Client
}

// New constructs a Client. Empty BaseURL/credentials = no-op client whose calls
// return ErrNotConfigured, so the caller can degrade gracefully in dev.
func New(cfg Config) *Client {
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	base := strings.TrimRight(cfg.BaseURL, "/")

	var ba string
	if cfg.ClientID != "" && cfg.ClientSecret != "" {
		ba = "Basic " + base64.StdEncoding.EncodeToString([]byte(cfg.ClientID+":"+cfg.ClientSecret))
	}

	c := &Client{basicAuth: ba, baseURL: base, httpClient: hc}
	if base != "" && ba != "" {
		inner, err := gen.NewClientWithResponses(base,
			gen.WithHTTPClient(hc),
			gen.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
				req.Header.Set("Authorization", ba)
				return nil
			}),
		)
		if err == nil {
			c.inner = inner
		}
	}
	return c
}

// Configured reports whether the client can talk to the artifact service.
func (c *Client) Configured() bool { return c.inner != nil && c.basicAuth != "" }

// InitUpload reserves quota + creates the artifact row, returning the presigned
// upload URL(s) for the browser to PUT directly to B2.
func (c *Client) InitUpload(ctx context.Context, req InitUploadRequest) (*InitUploadResponse, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	resp, err := c.inner.InitUploadWithResponse(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil && resp.JSON200.Code == 0 && resp.JSON200.Data != nil {
		return resp.JSON200.Data, nil
	}
	return nil, mapErr(resp.StatusCode(), resp.JSONDefault)
}

// CompleteUpload finalizes an upload (HeadObject size verify + optional manifest)
// and returns the ready artifact's metadata.
func (c *Client) CompleteUpload(ctx context.Context, uuid string, req CompleteUploadRequest) (*ArtifactResponse, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	resp, err := c.inner.CompleteUploadWithResponse(ctx, uuid, req)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil && resp.JSON200.Code == 0 && resp.JSON200.Data != nil {
		return resp.JSON200.Data, nil
	}
	return nil, mapErr(resp.StatusCode(), resp.JSONDefault)
}

// Resume restarts an interrupted upload. It asks the artifact service which
// parts are already stored in B2 (UploadedParts — skip them, reuse their etags)
// and returns fresh presigned URLs for the parts still missing (PartUrls); a
// single-part upload comes back with a fresh UploadUrl to re-PUT the whole file.
// Calling it also refreshes the upload's activity timestamp so artifact-gc won't
// reap an upload that's still being resumed. See infra docs/artifact/06 §2.3.
//
// Hand-written rather than generated: the committed gen client predates the
// /resume endpoint. It reuses the same base URL, HTTP client, Basic auth, and
// {code,message,data} envelope / error mapping as the generated calls — a future
// full re-gen from the synced contract can fold it back in.
func (c *Client) Resume(ctx context.Context, uuid string) (*ResumeUploadResponse, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	endpoint := c.baseURL + "/api/v1/artifacts/" + url.PathEscape(uuid) + "/resume"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.basicAuth)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		var env struct {
			Code int                   `json:"code"`
			Data *ResumeUploadResponse `json:"data"`
		}
		if json.Unmarshal(body, &env) == nil && env.Code == 0 && env.Data != nil {
			return env.Data, nil
		}
	}
	var he gen.HouseError
	_ = json.Unmarshal(body, &he)
	return nil, mapErr(resp.StatusCode, &he)
}

// Download returns a download URL (presigned GET, or a Worker URL for public
// artifacts on a site with a CDN base configured).
func (c *Client) Download(ctx context.Context, uuid string) (*DownloadResponse, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	resp, err := c.inner.DownloadArtifactWithResponse(ctx, uuid)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil && resp.JSON200.Code == 0 && resp.JSON200.Data != nil {
		return resp.JSON200.Data, nil
	}
	return nil, mapErr(resp.StatusCode(), resp.JSONDefault)
}

// Delete soft-deletes an artifact (GC physically reclaims it after the TTL).
func (c *Client) Delete(ctx context.Context, uuid string) error {
	if !c.Configured() {
		return ErrNotConfigured
	}
	resp, err := c.inner.DeleteArtifactWithResponse(ctx, uuid)
	if err != nil {
		return err
	}
	if resp.JSON200 != nil && resp.JSON200.Code == 0 {
		return nil
	}
	return mapErr(resp.StatusCode(), resp.JSONDefault)
}

// mapErr turns a non-success artifact response into a sentinel (where callers
// branch) or a descriptive error.
func mapErr(status int, he *gen.HouseError) error {
	code := 0
	msg := ""
	if he != nil {
		code = int(he.Code)
		msg = he.Message
	}
	switch code {
	case codeArtifactNotFound:
		return ErrNotFound
	case codeArtifactTooBig:
		return ErrTooBig
	case codeArtifactUnauthorized:
		return ErrUnauthorized
	case codeArtifactQuotaExceeded:
		return ErrQuotaExceeded
	case codeArtifactUploadDisabled:
		return ErrUploadDisabled
	case codeArtifactSizeMismatch:
		return ErrSizeMismatch
	case codeArtifactMIMEDenied:
		return ErrMIMEDenied
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return ErrUnauthorized
	}
	if msg == "" {
		msg = http.StatusText(status)
	}
	return fmt.Errorf("artifactclient: request failed (code %d, http %d): %s", code, status, msg)
}
