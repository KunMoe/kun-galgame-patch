// Package client wraps the HTTP calls to the Galgame Wiki Service.
//
// Background (D8 / D11): this project (the patch service) no longer stores
// galgame / tag / official metadata locally; instead it fetches it from the
// Wiki Service by vndb_id. Wiki's search is backed by Meilisearch with CJK
// tokenization, typo tolerance and facet aggregation, far better than in-repo
// ILIKE or a local index.
//
// See docs/galgame_wiki/api-reference.md.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// WikiError is returned by write methods on the Client when the Wiki Service
// envelope reports a non-zero `code`. It carries the wire-level (code,
// message) so the outer handler can transparently forward them — per
// docs/galgame_wiki/integration-guide.md §2 "直接透传 Wiki Service 的 code +
// message 给前端".
type WikiError struct {
	Code    int
	Message string
}

func (e *WikiError) Error() string {
	return fmt.Sprintf("wiki business error code=%d: %s", e.Code, e.Message)
}

// Client is a thin wrapper around calls to the Wiki Service.
type Client struct {
	baseURL         string
	http            *http.Client
	basicAuthHeader string // set via SetBasicAuth; required by GetWikiMessageFeed
}

// New constructs a Client. baseURL looks like http://127.0.0.1:9280/api
func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// wikiResponse is the common envelope for all Wiki JSON responses.
type wikiResponse[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

// Paginated is the shape of the data field in Wiki paginated responses.
type Paginated[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
}

// ─── Models (only the fields this project uses) ─────

// GalgameHit is a single item returned from Wiki /galgame/search.
//
// U1 (2026-05-18): the old `released string` field is gone; Wiki now exposes
// `release_date` (YYYY-MM-DD string or null) + `release_date_tba` (bool).
// The search query params `released_from / released_to / sort=released_*`
// remain unchanged on Wiki side (year-based filter/sort over the derived
// `released_year` index field).
type GalgameHit struct {
	ID               int     `json:"id"`
	VndbID           string  `json:"vndb_id"`
	NameEnUs         string  `json:"name_en_us"`
	NameZhCn         string  `json:"name_zh_cn"`
	NameJaJp         string  `json:"name_ja_jp"`
	NameZhTw         string  `json:"name_zh_tw"`
	Banner           string  `json:"banner"`
	ContentLimit     string  `json:"content_limit"`
	AgeLimit         string  `json:"age_limit"`
	OriginalLanguage string  `json:"original_language"`
	ReleaseDate         *string           `json:"release_date"`
	ReleaseDateTBA      bool              `json:"release_date_tba"`
	EffectiveBannerHash string            `json:"effective_banner_hash"`
	Covers              []CoverInput      `json:"covers"`
	Screenshots         []ScreenshotInput `json:"screenshots"`
	View                int               `json:"view"`
	Status              int               `json:"status"`
	TagIDs              []int             `json:"tag_ids"`
	OfficialIDs         []int             `json:"official_ids"`
	EngineIDs           []int             `json:"engine_ids"`
}

// GalgameBrief is the lightweight shape returned by /galgame/batch.
// U1: includes release_date / release_date_tba (see GalgameHit comment).
type GalgameBrief struct {
	ID                 int     `json:"id"`
	VndbID             string  `json:"vndb_id"`
	NameEnUs           string  `json:"name_en_us"`
	NameZhCn           string  `json:"name_zh_cn"`
	NameJaJp           string  `json:"name_ja_jp"`
	NameZhTw           string  `json:"name_zh_tw"`
	Banner             string  `json:"banner"`
	ContentLimit       string  `json:"content_limit"`
	AgeLimit           string  `json:"age_limit"`
	OriginalLanguage   string  `json:"original_language"`
	ReleaseDate         *string           `json:"release_date"`
	ReleaseDateTBA      bool              `json:"release_date_tba"`
	EffectiveBannerHash string            `json:"effective_banner_hash"`
	Covers              []CoverInput      `json:"covers"`
	Screenshots         []ScreenshotInput `json:"screenshots"`
	UserID              int               `json:"user_id"`
	ResourceUpdateTime  string            `json:"resource_update_time"`
}

// Tag is Wiki's galgame_tag.
type Tag struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	Aliases       []string `json:"aliases"`
	Category      string   `json:"category"`
	GalgameCount  int      `json:"galgame_count"`
}

// Official is Wiki's galgame_official (developer/publisher).
type Official struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	Aliases      []string `json:"aliases"`
	Category     string   `json:"category"`
	Lang         string   `json:"lang"`
	Link         string   `json:"link"`
	Description  string   `json:"description"`
	GalgameCount int      `json:"galgame_count"`
}

// CoverInput / ScreenshotInput mirror the Wiki cover/screenshot row shape
// (docs/galgame_wiki/03-relations.md §封面 / 截图). Used as both response
// element (galgame.covers / galgame.screenshots) and request element (PUT
// /galgame body covers/screenshots arrays) — identical fields, single round
// trip. ImageHash references image_service (no cross-service FK; the hash is
// guaranteed live via Wiki refping).
//
// Wiki PR5 (2026-05-18) replaced `banner_image_hash` with `covers[sort_order=0]`
// as the canonical "pinned banner"; the derived response field
// `effective_banner_hash` is the image_hash of that row (or empty if none).
type CoverInput struct {
	ImageHash string `json:"image_hash"`
	SortOrder int    `json:"sort_order"`
	Sexual    int    `json:"sexual"`
	Violence  int    `json:"violence"`
	Source    string `json:"source"`
	SourceKey string `json:"source_key"`
}

type ScreenshotInput struct {
	ImageHash string `json:"image_hash"`
	SortOrder int    `json:"sort_order"`
	Caption   string `json:"caption"`
	Sexual    int    `json:"sexual"`
	Violence  int    `json:"violence"`
	Source    string `json:"source"`
	SourceKey string `json:"source_key"`
}

// ─── Generic GET ─────────────────────────────────────

// get sends a GET request, parses the {code, message, data} envelope and unmarshals data into out.
func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("调用 Wiki 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取 Wiki 响应失败: %w", err)
	}

	var wrapper wikiResponse[json.RawMessage]
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return fmt.Errorf("解析 Wiki 响应失败: %w (body=%s)", err, truncate(string(body), 200))
	}
	if wrapper.Code != 0 {
		return fmt.Errorf("Wiki 业务错误 code=%d: %s", wrapper.Code, wrapper.Message)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(wrapper.Data, out); err != nil {
		return fmt.Errorf("解析 Wiki data 失败: %w", err)
	}
	return nil
}

// ─── High-level methods ──────────────────────────────

// SearchGalgameParams are query parameters for /galgame/search.
type SearchGalgameParams struct {
	Q               string
	Status          string // csv of statuses, e.g. "0" = published only; "" = no filter
	ContentLimit    string // sfw / nsfw
	AgeLimit        string // all / r18
	OriginalLang    string // csv, e.g. "ja-jp,en-us"
	TagIDs          []int
	OfficialIDs     []int
	EngineIDs       []int
	SeriesID        int
	ReleasedFrom    int
	ReleasedTo      int
	IncludeIntro    bool
	Sort            string // relevance / released_desc / released_asc / view / updated
	Page            int
	Limit           int
}

// SearchGalgame calls /galgame/search.
func (c *Client) SearchGalgame(ctx context.Context, p SearchGalgameParams) (*Paginated[GalgameHit], error) {
	q := url.Values{}
	if p.Q != "" {
		q.Set("q", p.Q)
	}
	if p.Status != "" {
		// docs/galgame_wiki/05-search.md: status csv; omit = no filter.
		q.Set("status", p.Status)
	}
	if p.ContentLimit != "" {
		q.Set("content_limit", p.ContentLimit)
	}
	if p.AgeLimit != "" {
		q.Set("age_limit", p.AgeLimit)
	}
	if p.OriginalLang != "" {
		q.Set("original_language", p.OriginalLang)
	}
	if len(p.TagIDs) > 0 {
		q.Set("tag_ids", joinInts(p.TagIDs))
	}
	if len(p.OfficialIDs) > 0 {
		q.Set("official_ids", joinInts(p.OfficialIDs))
	}
	if len(p.EngineIDs) > 0 {
		q.Set("engine_ids", joinInts(p.EngineIDs))
	}
	if p.SeriesID > 0 {
		q.Set("series_id", strconv.Itoa(p.SeriesID))
	}
	if p.ReleasedFrom > 0 {
		q.Set("released_from", strconv.Itoa(p.ReleasedFrom))
	}
	if p.ReleasedTo > 0 {
		q.Set("released_to", strconv.Itoa(p.ReleasedTo))
	}
	if p.IncludeIntro {
		q.Set("include_intro", "true")
	}
	if p.Sort != "" {
		q.Set("sort", p.Sort)
	}
	if p.Page > 0 {
		q.Set("page", strconv.Itoa(p.Page))
	}
	if p.Limit > 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	q.Set("facets", "false")
	q.Set("highlight", "false")

	var out Paginated[GalgameHit]
	if err := c.get(ctx, "/galgame/search", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GalgameFull is the full galgame returned from /galgame/:gid (including intro / tag_ids / official_ids).
// Used to enrich detail pages.
type GalgameFull struct {
	ID               int    `json:"id"`
	VndbID           string `json:"vndb_id"`
	NameEnUs         string `json:"name_en_us"`
	NameZhCn         string `json:"name_zh_cn"`
	NameJaJp         string `json:"name_ja_jp"`
	NameZhTw         string `json:"name_zh_tw"`
	Banner           string `json:"banner"`
	IntroEnUs        string `json:"intro_en_us"`
	IntroZhCn        string `json:"intro_zh_cn"`
	IntroJaJp        string `json:"intro_ja_jp"`
	IntroZhTw        string `json:"intro_zh_tw"`
	ContentLimit     string  `json:"content_limit"`
	AgeLimit         string  `json:"age_limit"`
	OriginalLanguage string  `json:"original_language"`
	ReleaseDate      *string `json:"release_date"`
	ReleaseDateTBA   bool    `json:"release_date_tba"`
	View             int     `json:"view"`
	SeriesID         *int    `json:"series_id"`
	Alias            []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"alias"`
	Tag []struct {
		GalgameID    int `json:"galgame_id"`
		TagID        int `json:"tag_id"`
		SpoilerLevel int `json:"spoiler_level"`
		Tag          Tag `json:"tag"`
	} `json:"tag"`
	Official []struct {
		GalgameID  int      `json:"galgame_id"`
		OfficialID int      `json:"official_id"`
		Official   Official `json:"official"`
	} `json:"official"`
	Engine []struct {
		GalgameID int    `json:"galgame_id"`
		EngineID  int    `json:"engine_id"`
		Engine    map[string]any `json:"engine"`
	} `json:"engine"`
	Link []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
		Link string `json:"link"`
	} `json:"link"`
	EffectiveBannerHash string            `json:"effective_banner_hash"`
	Covers              []CoverInput      `json:"covers"`
	Screenshots         []ScreenshotInput `json:"screenshots"`
	Created             string            `json:"created"`
	Updated             string            `json:"updated"`
}

// GalgameDetailEnvelope is the data envelope for /galgame/:gid. Wiki nests another layer of galgame + users under data.
type GalgameDetailEnvelope struct {
	Galgame GalgameFull            `json:"galgame"`
	Users   map[string]any         `json:"users"`
}

// GetGalgame calls /galgame/:gid; used to enrich detail pages.
func (c *Client) GetGalgame(ctx context.Context, gid int) (*GalgameDetailEnvelope, error) {
	var out GalgameDetailEnvelope
	if err := c.get(ctx, fmt.Sprintf("/galgame/%d", gid), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CheckGalgameByVndbID calls /galgame/check?vndb_id=xxx and returns (exists, galgame_id).
// Used as a pre-check for POST /api/patch.
func (c *Client) CheckGalgameByVndbID(ctx context.Context, vndbID string) (exists bool, galgameID int, err error) {
	q := url.Values{}
	q.Set("vndb_id", vndbID)

	var out struct {
		Exists    bool `json:"exists"`
		GalgameID int  `json:"galgame_id"`
	}
	if err := c.get(ctx, "/galgame/check", q, &out); err != nil {
		return false, 0, err
	}
	return out.Exists, out.GalgameID, nil
}

// GalgameBatch calls /galgame/batch?ids=1,2,3 to fetch lightweight galgame info in bulk.
func (c *Client) GalgameBatch(ctx context.Context, ids []int) ([]GalgameBrief, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	q := url.Values{}
	q.Set("ids", joinInts(ids))

	var out []GalgameBrief
	if err := c.get(ctx, "/galgame/batch", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// TagSearchResult is the response from /tag/search (note: it is not wrapped in Paginated; total is at the top level).
type TagSearchResult struct {
	Items            []Tag `json:"items"`
	Total            int64 `json:"total"`
	ProcessingTimeMs int64 `json:"processing_time_ms"`
}

// SearchTag calls /tag/search.
func (c *Client) SearchTag(ctx context.Context, q, category string, limit int) (*TagSearchResult, error) {
	params := url.Values{}
	if q != "" {
		params.Set("q", q)
	}
	if category != "" {
		params.Set("category", category)
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	var out TagSearchResult
	if err := c.get(ctx, "/tag/search", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// OfficialSearchResult is the response from /official/search.
type OfficialSearchResult struct {
	Items            []Official `json:"items"`
	Total            int64      `json:"total"`
	ProcessingTimeMs int64      `json:"processing_time_ms"`
}

// SearchOfficial calls /official/search.
func (c *Client) SearchOfficial(ctx context.Context, q, category, lang string, limit int) (*OfficialSearchResult, error) {
	params := url.Values{}
	if q != "" {
		params.Set("q", q)
	}
	if category != "" {
		params.Set("category", category)
	}
	if lang != "" {
		params.Set("lang", lang)
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	var out OfficialSearchResult
	if err := c.get(ctx, "/official/search", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ─── Write methods (require user OAuth access_token) ───
//
// Per integration-guide.md §2, write operations are proxied through the site
// backend, but the user identity is carried by the user's OAuth access_token
// (the same one we already keep in the Redis session). The Wiki Service
// validates the JWT, extracts the userID, and enforces creator/admin rules
// itself — the patch backend does not need to re-implement authorization.

// UpdateGalgameRequest mirrors the documented JSON body of PUT /galgame/:gid.
// All fields are pointers so the JSON encoding only includes what was set
// (any unset field on the Wiki side stays unchanged).
//
// U1: ReleaseDate / ReleaseDateTBA replace the old `released string`.
// W2 / Wiki PR5: BannerImageHash dropped — new banner edits go through
// `Covers[sort_order=0]` or the multipart `file` field on PUT /galgame.
// Covers / Screenshots follow presence semantics (omit = keep集合 unchanged;
// `[]` = clear all; non-empty = authoritative full replace — caller MUST
// resubmit current full set, see docs/galgame_wiki/00-handbook §15 PR2-5 段).
type UpdateGalgameRequest struct {
	NameEnUs         *string            `json:"name_en_us,omitempty"`
	NameJaJp         *string            `json:"name_ja_jp,omitempty"`
	NameZhCn         *string            `json:"name_zh_cn,omitempty"`
	NameZhTw         *string            `json:"name_zh_tw,omitempty"`
	IntroEnUs        *string            `json:"intro_en_us,omitempty"`
	IntroJaJp        *string            `json:"intro_ja_jp,omitempty"`
	IntroZhCn        *string            `json:"intro_zh_cn,omitempty"`
	IntroZhTw        *string            `json:"intro_zh_tw,omitempty"`
	ContentLimit     *string            `json:"content_limit,omitempty"`
	AgeLimit         *string            `json:"age_limit,omitempty"`
	OriginalLanguage *string            `json:"original_language,omitempty"`
	ReleaseDate      *string            `json:"release_date,omitempty"`
	ReleaseDateTBA   *bool              `json:"release_date_tba,omitempty"`
	Aliases          *string            `json:"aliases,omitempty"`
	Covers           *[]CoverInput      `json:"covers,omitempty"`
	Screenshots      *[]ScreenshotInput `json:"screenshots,omitempty"`
	SeriesID         *int               `json:"series_id,omitempty"`
	TagIDs           *[]int             `json:"tag_ids,omitempty"`
	OfficialIDs      *[]int             `json:"official_ids,omitempty"`
	EngineIDs        *[]int             `json:"engine_ids,omitempty"`
	IsMinor          *bool              `json:"is_minor,omitempty"`
}

// UpdateGalgame proxies PUT /galgame/:gid. Returns the raw `data` payload
// (left as RawMessage so callers can re-wrap without an extra unmarshal).
// On a non-zero Wiki envelope code, returns *WikiError.
func (c *Client) UpdateGalgame(ctx context.Context, accessToken string, gid int, body any) (json.RawMessage, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode update body: %w", err)
	}
	u := fmt.Sprintf("%s/galgame/%d", c.baseURL, gid)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build wiki update request: %w", err)
	}
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wiki PUT galgame: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read wiki response: %w", err)
	}

	var env wikiResponse[json.RawMessage]
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode wiki envelope: %w (body=%s)", err, truncate(string(raw), 200))
	}
	if env.Code != 0 {
		return nil, &WikiError{Code: env.Code, Message: env.Message}
	}
	return env.Data, nil
}

// UpdateGalgameMultipart proxies PUT /galgame/:gid in multipart/form-data
// mode (per docs/galgame_wiki/01-galgame.md §Banner 上传). The wire shape:
//
//   data=<JSON string mirroring UpdateGalgameRequest>
//   file=<image binary>           (optional, only when changing banner)
//
// Used by the patch site's edit form when the user picks a new banner.
// Returns Wiki's raw `data` payload on success and *WikiError on a non-zero
// envelope code (so the handler can transparently forward it).
func (c *Client) UpdateGalgameMultipart(
	ctx context.Context,
	accessToken string,
	gid int,
	jsonBody any,
	fileName string,
	fileContent []byte,
	fileMime string,
) (json.RawMessage, error) {
	payload, err := json.Marshal(jsonBody)
	if err != nil {
		return nil, fmt.Errorf("encode update body: %w", err)
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("data", string(payload)); err != nil {
		return nil, fmt.Errorf("write data field: %w", err)
	}
	if len(fileContent) > 0 {
		// Build a proper Content-Type part header for the binary so the Wiki
		// side recognizes the mime type (image/jpeg|png|webp).
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition",
			fmt.Sprintf(`form-data; name="file"; filename=%q`, fileName))
		if fileMime != "" {
			h.Set("Content-Type", fileMime)
		}
		fw, err := w.CreatePart(h)
		if err != nil {
			return nil, fmt.Errorf("create file part: %w", err)
		}
		if _, err := fw.Write(fileContent); err != nil {
			return nil, fmt.Errorf("write file part: %w", err)
		}
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	u := fmt.Sprintf("%s/galgame/%d", c.baseURL, gid)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, &buf)
	if err != nil {
		return nil, fmt.Errorf("build wiki update request: %w", err)
	}
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wiki PUT galgame (multipart): %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read wiki response: %w", err)
	}
	var env wikiResponse[json.RawMessage]
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode wiki envelope: %w (body=%s)", err, truncate(string(raw), 200))
	}
	if env.Code != 0 {
		return nil, &WikiError{Code: env.Code, Message: env.Message}
	}
	return env.Data, nil
}

// ─── helpers ─────────────────────────────────────────

func joinInts(xs []int) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		parts = append(parts, strconv.Itoa(x))
	}
	return strings.Join(parts, ",")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
