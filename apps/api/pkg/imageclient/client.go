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
	"strconv"
	"strings"
	"time"

	"kun-galgame-patch-api/pkg/upstream"
)

const service = "image"

// The image service's house codes (infra pkg/errors/codes.go 80001-80015,
// 60002). Only the ones a reader's file can cause are named: the credential,
// site and preset codes arrive as 400/401/403 and are moyu's own configuration.
const (
	codeFileTooLarge       = 80007
	codeQuotaExceeded      = 80008
	codeMIMEDenied         = 80009
	codeDecodeFailed       = 80010
	codeNotFound           = 80013
	codeUploadDisabled     = 80015
	codeModerationRejected = 60002
)

var (
	ErrNotConfigured      = errors.New("imageclient: not configured (empty base URL or credentials)")
	ErrFileTooLarge       = errors.New("imageclient: file exceeds the preset's size limit")
	ErrQuotaExceeded      = errors.New("imageclient: site daily upload quota exceeded")
	ErrMIMEDenied         = errors.New("imageclient: preset does not accept this format")
	ErrDecodeFailed       = errors.New("imageclient: image could not be decoded")
	ErrModerationRejected = errors.New("imageclient: image rejected by moderation")
	ErrUploadDisabled     = errors.New("imageclient: upload disabled")
)

type Config struct {
	BaseURL      string
	CDNBase      string
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client
}

// This client addresses OAuth's content-addressed image store; moyu has no
// local object-key namespace.
type Client struct {
	baseURL    string
	cdnBase    string
	basicAuth  string
	httpClient *http.Client
}

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

func (c *Client) Configured() bool { return c.baseURL != "" && c.basicAuth != "" }

type UploadResult struct {
	Hash         string            `json:"hash"`
	URL          string            `json:"url"`
	VariantURLs  map[string]string `json:"variant_urls"`
	Width        int               `json:"width"`
	Height       int               `json:"height"`
	Thumbhash    string            `json:"thumbhash,omitempty"`
	SizeBytes    int64             `json:"size_bytes"`
	Deduplicated bool              `json:"deduplicated"`
}

func (c *Client) Upload(
	ctx context.Context,
	body io.Reader, filename, mime, preset string,
) (*UploadResult, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
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

	var data UploadResult
	if err := c.post(ctx, "upload", "/image/upload", w.FormDataContentType(), &buf, &data); err != nil {
		return nil, err
	}
	if data.VariantURLs == nil {
		data.VariantURLs = map[string]string{}
	}
	return &data, nil
}

type ReferencePingResult struct {
	Updated  int64    `json:"updated"`
	NotFound []string `json:"not_found"`
}

func (c *Client) ReferencePing(ctx context.Context, hashes []string) (*ReferencePingResult, error) {
	if len(hashes) == 0 {
		return &ReferencePingResult{}, nil
	}
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	if len(hashes) > 1000 {
		return nil, fmt.Errorf("imageclient: batch size %d exceeds limit 1000", len(hashes))
	}
	body, _ := json.Marshal(struct {
		Hashes []string `json:"hashes"`
	}{Hashes: hashes})

	var data ReferencePingResult
	if err := c.post(ctx, "reference-ping", "/image/reference-ping", "application/json", bytes.NewReader(body), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

type ImageMeta struct {
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Thumbhash string `json:"thumbhash,omitempty"`
}

func (c *Client) MetaBatch(ctx context.Context, hashes []string) (map[string]ImageMeta, error) {
	if len(hashes) == 0 {
		return map[string]ImageMeta{}, nil
	}
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	if len(hashes) > 1000 {
		return nil, fmt.Errorf("imageclient: batch size %d exceeds limit 1000", len(hashes))
	}
	body, _ := json.Marshal(struct {
		Hashes []string `json:"hashes"`
	}{Hashes: hashes})

	var data struct {
		Metas map[string]ImageMeta `json:"metas"`
	}
	if err := c.post(ctx, "meta-batch", "/image/meta-batch", "application/json", bytes.NewReader(body), &data); err != nil {
		return nil, err
	}
	if data.Metas == nil {
		return map[string]ImageMeta{}, nil
	}
	return data.Metas, nil
}

func (c *Client) post(ctx context.Context, op, path, contentType string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("build %s request: %w", op, err)
	}
	req.Header.Set("Authorization", c.basicAuth)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return upstream.Transport(service, op, err)
	}
	defer resp.Body.Close()
	raw, err := upstream.ReadBody(resp.Body)
	if err != nil {
		return upstream.Transport(service, op, err)
	}

	var env struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	decodeErr := json.Unmarshal(raw, &env)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		if decodeErr == nil && env.Code == 0 {
			if err := json.Unmarshal(env.Data, out); err == nil {
				return nil
			}
		}
		return &upstream.Error{Service: service, Op: op, Kind: upstream.Internal, Status: resp.StatusCode,
			RequestID: resp.Header.Get("X-Request-ID"), Detail: "undecodable success body"}
	}
	return failure(op, resp, env.Code, env.Message, decodeErr == nil)
}

func failure(op string, resp *http.Response, code int, message string, house bool) *upstream.Error {
	e := &upstream.Error{
		Service:    service,
		Op:         op,
		Status:     resp.StatusCode,
		RequestID:  resp.Header.Get("X-Request-ID"),
		RetryAfter: upstream.RetryAfter(resp.Header),
	}
	if !house || code == 0 {
		e.Kind = upstream.Internal
		if e.Status >= 500 {
			e.Kind = upstream.Unavailable
		}
		return e
	}
	e.Code = strconv.Itoa(code)
	e.Detail = message
	switch code {
	case codeFileTooLarge:
		e.Kind, e.Cause = upstream.Rejected, ErrFileTooLarge
	case codeMIMEDenied:
		e.Kind, e.Cause = upstream.Rejected, ErrMIMEDenied
	case codeDecodeFailed:
		e.Kind, e.Cause = upstream.Rejected, ErrDecodeFailed
	case codeModerationRejected:
		e.Kind, e.Cause = upstream.Rejected, ErrModerationRejected
	case codeQuotaExceeded:
		e.Kind, e.Cause = upstream.RateLimited, ErrQuotaExceeded
	case codeUploadDisabled:
		e.Kind, e.Cause = upstream.Unavailable, ErrUploadDisabled
	case codeNotFound:
		e.Kind = upstream.NotFound
	default:
		e.Kind = upstream.ByStatus(e.Status)
	}
	return e
}

func (c *Client) MainURL(hash string) string {
	return c.variantPath(hash, "")
}

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
