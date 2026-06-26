// Package imageclient is a thin SDK for the centralized image_service.
//
// Background: image_service is a hash-addressed blob store hosted by
// kun-galgame-infra (port 9278). Callers POST an image multipart body and get
// back a content hash + a set of variant URLs. The hash is the only thing
// downstream stores; URLs are derived deterministically from
// `{CDN_BASE}/<hash[:2]>/<hash[2:4]>/<hash>[_variant].webp` (the image_service
// object-key layout — no `/img/` segment; matches infra/kungal imageclient).
//
// Authentication is HTTP Basic with an OAuth client_id/secret — the
// image_service shares the OAuth `oauth_client` table as its "site" registry,
// so the project's existing OAuth credentials work as-is (provided the admin
// flipped `image_enabled=true` and listed the desired presets in
// `image_allowed_presets` on the kun-galgame-infra side).
//
// See docs at:
//   - kun-galgame-infra/docs/image_service/03-api-design.md (endpoints)
//   - kun-galgame-infra/docs/image_service/06-integration-guide.md (SDK contract)
//
// This file intentionally stays small (single Upload call + URL helpers) and
// stdlib-only — the screenshot editor is currently the only consumer; if more
// surfaces show up (e.g. moyu also wants to manage avatars through this SDK)
// we can grow it then.
package imageclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"strings"
	"time"
)

// Sentinel errors callers can `errors.Is` against.
var (
	ErrQuotaExceeded      = errors.New("image_service: daily upload quota exceeded")
	ErrModerationRejected = errors.New("image_service: image rejected by moderation")
	ErrUnauthorized       = errors.New("image_service: unauthorized (check client_id/secret + image_enabled)")
)

// Config bundles connection settings. Created in app.go from
// config.ImageServiceConfig; defaults for ClientID/Secret fall back to the
// project's OAuth credentials there.
type Config struct {
	BaseURL      string // e.g. http://127.0.0.1:9278 (no trailing slash)
	CDNBase      string // e.g. http://127.0.0.1:9000/kun-images-dev
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client // optional; defaults to 30s timeout client
}

// Client is a thin singleton-friendly wrapper. Reuse one across the process.
type Client struct {
	baseURL    string
	cdnBase    string
	basicAuth  string
	httpClient *http.Client
}

// New constructs a Client. Empty BaseURL = no-op client (Upload returns error
// quickly so the caller can degrade gracefully when image_service is
// intentionally disabled in dev).
func New(cfg Config) *Client {
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	var ba string
	if cfg.ClientID != "" && cfg.ClientSecret != "" {
		ba = "Basic " + base64.StdEncoding.EncodeToString(
			[]byte(cfg.ClientID+":"+cfg.ClientSecret),
		)
	}
	return &Client{
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		cdnBase:    strings.TrimRight(cfg.CDNBase, "/"),
		basicAuth:  ba,
		httpClient: hc,
	}
}

// UploadResult mirrors POST /image/upload's success response.
type UploadResult struct {
	Hash          string            `json:"hash"`
	URL           string            `json:"url"` // main image URL
	VariantURLs   map[string]string `json:"variant_urls"`
	Width         int               `json:"width"`
	Height        int               `json:"height"`
	// Thumbhash is the base64 ThumbHash placeholder the image service now returns
	// on upload (omitempty for rows predating the column). Parsed so callers that
	// persist upload results can ship a blur-up placeholder + reserve the aspect
	// ratio without a second roundtrip; matches the infra SDK's UploadResult.
	Thumbhash     string            `json:"thumbhash,omitempty"`
	SizeBytes     int64             `json:"size_bytes"`
	Deduplicated  bool              `json:"deduplicated"`
}

// Upload sends one image to image_service.
//
//   - body / filename / mime describe the file (mime may be "" — the server
//     sniffs by magic number anyway).
//   - preset must be one of the presets enabled for our client (e.g. "topic"
//     for free-form gallery / editor-inline images, "galgame_screenshot" for
//     galgame screenshots; "galgame_banner"/"avatar" exist but moyu uploads
//     those via other paths — see UploadImageService handler doc).
//
// On non-2xx the response body's error code is mapped to one of the sentinels
// where it makes sense, or wrapped with the raw status for unrecognized codes.
func (c *Client) Upload(
	ctx context.Context,
	body io.Reader, filename, mime, preset string,
) (*UploadResult, error) {
	if c.baseURL == "" {
		return nil, errors.New("image_service: client not configured (KUN_IMAGE_SERVICE_BASE_URL unset)")
	}
	if c.basicAuth == "" {
		return nil, ErrUnauthorized
	}
	if filename == "" {
		filename = "upload.bin"
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("preset", preset); err != nil {
		return nil, fmt.Errorf("write preset field: %w", err)
	}
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="file"; filename=%q`, filepath.Base(filename)))
	if mime != "" {
		h.Set("Content-Type", mime)
	}
	fw, err := w.CreatePart(h)
	if err != nil {
		return nil, fmt.Errorf("create file part: %w", err)
	}
	if _, err := io.Copy(fw, body); err != nil {
		return nil, fmt.Errorf("copy file body: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close multipart: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/image/upload", &buf)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", c.basicAuth)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("image_service POST /image/upload: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	// image_service wraps EVERY response in {code,message,data} (api/pkg/response).
	// A 2xx success body is {code:0,message:"成功",data:{hash,url,variant_urls,...}}.
	// (The previous code unmarshalled the bare body into UploadResult, so hash/url
	// came back empty on every successful upload — silently breaking the screenshot
	// editor. The image_service's own handler_http_test.go reads `env.Data`.)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		var env struct {
			Code int          `json:"code"`
			Data UploadResult `json:"data"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, fmt.Errorf("decode upload response: %w (body=%s)", err, truncate(string(raw), 200))
		}
		out := env.Data
		if out.VariantURLs == nil {
			out.VariantURLs = map[string]string{}
		}
		return &out, nil
	}

	// Error body is the same flat envelope {code:<int>,message:<string>} (NOT a
	// nested {error:{...}} — that was a stale-doc assumption). Map the known
	// integer business codes (kun-galgame-infra/pkg/errors/codes.go) to sentinels;
	// otherwise wrap with code+message.
	var env struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(raw, &env)

	switch env.Code {
	case 80008: // ErrImageQuotaExceeded
		return nil, fmt.Errorf("%w: %s", ErrQuotaExceeded, env.Message)
	case 60002: // ErrModerationRejected
		return nil, fmt.Errorf("%w: %s", ErrModerationRejected, env.Message)
	case 80001, 80002, 80003, 80004, 80005, 80006, 80015:
		// unauthorized / bad client / bad secret / site disabled / site
		// unconfigured / preset denied / upload disabled — all "you can't" class.
		return nil, fmt.Errorf("%w: %s", ErrUnauthorized, env.Message)
	}
	return nil, fmt.Errorf("image_service upload failed: status=%d code=%d msg=%q",
		resp.StatusCode, env.Code, env.Message)
}

// Configured reports whether the client has both a base URL and credentials —
// i.e. Upload / ReferencePing can actually reach image_service. Lets callers
// skip work (e.g. the ref-ping cron) when image_service is intentionally
// disabled in dev.
func (c *Client) Configured() bool { return c.baseURL != "" && c.basicAuth != "" }

// ReferencePingResult mirrors POST /image/reference-ping's success response.
type ReferencePingResult struct {
	Updated  int64    `json:"updated"`
	NotFound []string `json:"not_found"`
}

// ReferencePing refreshes last_referenced_at for a batch of hashes so the
// image_service GC doesn't reclaim images this site still references (cold-
// storage TTL ~60 days). Call it from a daily cron with EVERY hash the site
// still points at — entity columns AND content-embedded `/image/<hash>` tokens
// (see internal/infrastructure/cron). The server caps a batch at 1000; the
// caller is responsible for chunking larger sets.
func (c *Client) ReferencePing(ctx context.Context, hashes []string) (*ReferencePingResult, error) {
	if len(hashes) == 0 {
		return &ReferencePingResult{}, nil
	}
	if c.baseURL == "" {
		return nil, errors.New("image_service: client not configured (KUN_IMAGE_SERVICE_BASE_URL unset)")
	}
	if c.basicAuth == "" {
		return nil, ErrUnauthorized
	}
	if len(hashes) > 1000 {
		return nil, fmt.Errorf("imageclient: batch size %d exceeds limit 1000", len(hashes))
	}

	body, _ := json.Marshal(struct {
		Hashes []string `json:"hashes"`
	}{Hashes: hashes})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/image/reference-ping", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", c.basicAuth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("image_service POST /image/reference-ping: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var env struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &env)
		switch env.Code {
		case 80001, 80002, 80003, 80004, 80005:
			return nil, fmt.Errorf("%w: %s", ErrUnauthorized, env.Message)
		}
		return nil, fmt.Errorf("image_service reference-ping failed: status=%d code=%d msg=%q",
			resp.StatusCode, env.Code, env.Message)
	}

	var env struct {
		Data ReferencePingResult `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode reference-ping response: %w (body=%s)", err, truncate(string(raw), 200))
	}
	return &env.Data, nil
}

// MainURL builds the canonical CDN URL for the full image (`.webp`).
// Returns "" for empty / malformed hashes — caller can fall back to a
// placeholder.
func (c *Client) MainURL(hash string) string {
	return c.variantPath(hash, "")
}

// VariantURL builds the CDN URL for a pre-generated variant (e.g. "mini",
// "100", "256"). Returns "" if the hash is invalid.
func (c *Client) VariantURL(hash, variant string) string {
	return c.variantPath(hash, variant)
}

func (c *Client) variantPath(hash, variant string) string {
	if len(hash) < 4 || !isHex(hash) {
		return ""
	}
	suffix := ""
	if variant != "" {
		suffix = "_" + variant
	}
	return fmt.Sprintf("%s/%s/%s/%s%s.webp",
		c.cdnBase, hash[:2], hash[2:4], hash, suffix)
}

func isHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
