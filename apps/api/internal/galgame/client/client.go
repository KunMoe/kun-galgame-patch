package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const galgameCodeNotFound = 404

type GalgameError struct {
	Code       int
	Message    string
	HTTPStatus int
	Moved      int64
}

func (e *GalgameError) Error() string {
	return fmt.Sprintf("galgame business error code=%d: %s", e.Code, e.Message)
}

func (e *GalgameError) Absent() bool {
	if e.HTTPStatus == 0 {
		return e.Code == galgameCodeNotFound
	}
	return e.HTTPStatus == http.StatusNotFound && e.Code == catalogCodeNotFound
}

func IsAbsent(err error) bool {
	var gerr *GalgameError
	return errors.As(err, &gerr) && gerr.Absent()
}

func MovedTarget(err error) (int64, bool) {
	var gerr *GalgameError
	if !errors.As(err, &gerr) {
		return 0, false
	}
	if gerr.HTTPStatus != http.StatusMovedPermanently || gerr.Code != catalogCodeMoved || gerr.Moved <= 0 {
		return 0, false
	}
	return gerr.Moved, true
}

func AsBadRequest(err error) (*GalgameError, bool) {
	var gerr *GalgameError
	if !errors.As(err, &gerr) || gerr.HTTPStatus != http.StatusBadRequest {
		return nil, false
	}
	return gerr, true
}

func upstreamError(resp *http.Response, code int, message string) *GalgameError {
	return &GalgameError{Code: code, Message: message, HTTPStatus: resp.StatusCode}
}

type Client struct {
	v1Base string
	apiKey string
	http   *http.Client
	gids   *gidMap
}

func NewWithKey(baseURL, apiKey string) *Client {
	base := strings.TrimRight(baseURL, "/")
	return &Client{
		v1Base: base + "/v1",
		apiKey: apiKey,
		http: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		gids: newGIDMap(),
	}
}

type galgameResponse[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type Paginated[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
}

type GalgameBrief struct {
	ID                         int               `json:"id"`
	CatalogWorkID              int64             `json:"catalog_work_id,omitempty"`
	VndbID                     string            `json:"vndb_id"`
	ClaimState                 string            `json:"claim_state"`
	NameEnUs                   string            `json:"name_en_us"`
	NameZhCn                   string            `json:"name_zh_cn"`
	NameJaJp                   string            `json:"name_ja_jp"`
	NameZhTw                   string            `json:"name_zh_tw"`
	Banner                     string            `json:"banner"`
	ContentLimit               string            `json:"content_limit"`
	AgeLimit                   string            `json:"age_limit"`
	OriginalLanguage           string            `json:"original_language"`
	ReleaseDate                *string           `json:"release_date"`
	ReleasePrecision           string            `json:"release_precision,omitempty"`
	EffectiveBannerHash        string            `json:"effective_banner_hash"`
	EffectiveBannerWidth       int               `json:"effective_banner_width,omitempty"`
	EffectiveBannerHeight      int               `json:"effective_banner_height,omitempty"`
	EffectiveBannerThumbhash   string            `json:"effective_banner_thumbhash,omitempty"`
	EffectivePortraitHash      string            `json:"effective_portrait_hash,omitempty"`
	EffectivePortraitWidth     int               `json:"effective_portrait_width,omitempty"`
	EffectivePortraitHeight    int               `json:"effective_portrait_height,omitempty"`
	EffectivePortraitThumbhash string            `json:"effective_portrait_thumbhash,omitempty"`
	Covers                     []CoverInput      `json:"covers"`
	Screenshots                []ScreenshotInput `json:"screenshots"`
}

type GalgameHit struct {
	ID                       int               `json:"id"`
	CatalogWorkID            int64             `json:"catalog_work_id,omitempty"`
	VndbID                   string            `json:"vndb_id"`
	ClaimState               string            `json:"claim_state"`
	NameEnUs                 string            `json:"name_en_us"`
	NameZhCn                 string            `json:"name_zh_cn"`
	NameJaJp                 string            `json:"name_ja_jp"`
	NameZhTw                 string            `json:"name_zh_tw"`
	Banner                   string            `json:"banner"`
	ContentLimit             string            `json:"content_limit"`
	AgeLimit                 string            `json:"age_limit"`
	OriginalLanguage         string            `json:"original_language"`
	ReleaseDate              *string           `json:"release_date"`
	EffectiveBannerHash      string            `json:"effective_banner_hash"`
	EffectiveBannerWidth     int               `json:"effective_banner_width,omitempty"`
	EffectiveBannerHeight    int               `json:"effective_banner_height,omitempty"`
	EffectiveBannerThumbhash string            `json:"effective_banner_thumbhash,omitempty"`
	Covers                   []CoverInput      `json:"covers"`
	Screenshots              []ScreenshotInput `json:"screenshots"`
	TagIDs                   []int             `json:"tag_ids"`
	OfficialIDs              []int             `json:"official_ids"`
	EngineIDs                []int             `json:"engine_ids"`
}

type Tag struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	Aliases      []string `json:"aliases"`
	Category     string   `json:"category"`
	GalgameCount int      `json:"galgame_count"`
}

type Official struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	Aliases      []string `json:"aliases"`
	Category     string   `json:"category"`
	Lang         string   `json:"lang"`
	Link         string   `json:"link"`
	Description  string   `json:"description"`
	GalgameCount int      `json:"galgame_count"`
	LogoHash     string   `json:"logo_hash"`
}

type CoverInput struct {
	ImageHash string `json:"image_hash"`
	SortOrder int    `json:"sort_order"`
	Sexual    int    `json:"sexual"`
	Violence  int    `json:"violence"`
	Source    string `json:"source"`
	SourceKey string `json:"source_key"`
	Kind      string `json:"kind,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Thumbhash string `json:"thumbhash,omitempty"`
}

type ScreenshotInput struct {
	ImageHash string `json:"image_hash"`
	SortOrder int    `json:"sort_order"`
	Caption   string `json:"caption"`
	Sexual    int    `json:"sexual"`
	Violence  int    `json:"violence"`
	Source    string `json:"source"`
	SourceKey string `json:"source_key"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Thumbhash string `json:"thumbhash,omitempty"`
}

func (c *Client) getV1Raw(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
	data, _, err := c.getV1RawStatus(ctx, path, query)
	return data, err
}

func (c *Client) getV1RawStatus(ctx context.Context, path string, query url.Values) (json.RawMessage, int, error) {
	u := c.v1Base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("构造请求失败: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("调用 galgame 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("读取 galgame 响应失败: %w", err)
	}

	var wrapper galgameResponse[json.RawMessage]
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("解析 galgame 响应失败: %w (body=%s)", err, truncate(string(body), 200))
	}
	if wrapper.Code != 0 {
		gerr := upstreamError(resp, wrapper.Code, wrapper.Message)
		if wrapper.Code == catalogCodeMoved {
			var moved struct {
				CurrentID int64 `json:"current_id"`
			}
			if json.Unmarshal(wrapper.Data, &moved) == nil {
				gerr.Moved = moved.CurrentID
			}
		}
		return nil, resp.StatusCode, gerr
	}
	return wrapper.Data, resp.StatusCode, nil
}

func (c *Client) getV1(ctx context.Context, path string, query url.Values, out any) error {
	data, err := c.getV1Raw(ctx, path, query)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("解析 galgame data 失败: %w", err)
	}
	return nil
}

type SearchGalgameParams struct {
	Q            string
	ContentLimit string
	AgeLimit     string
	OriginalLang string
	TagIDs       []int
	OfficialIDs  []int
	EngineIDs    []int
	SeriesID     int
	ReleasedFrom int
	ReleasedTo   int
	SearchIntro  bool
	Sort         string
	Page         int
	Limit        int
}

func searchSortForCatalog(sort string) string {
	switch strings.TrimSpace(sort) {
	case "", "relevance":
		return ""
	case "view":
		return "popularity"
	default:
		return sort
	}
}

func (c *Client) SearchGalgame(ctx context.Context, p SearchGalgameParams) (*Paginated[GalgameHit], error) {
	q := url.Values{}
	if p.Q != "" {
		q.Set("q", p.Q)
	}
	gate := gateFor(p.ContentLimit)
	if p.AgeLimit == "r18" {
		gate.contentRating = "r18"
	}
	gate.apply(q)
	q.Set("claim_state", "live")

	if lang := joinCatalogLangs(p.OriginalLang); lang != "" {
		q.Set("olang", lang)
	}
	if len(p.TagIDs) > 0 {
		q.Set("tag_id", joinInts(p.TagIDs))
	}
	if len(p.OfficialIDs) > 0 {
		q.Set("label_id", strconv.Itoa(p.OfficialIDs[0]))
	}
	if len(p.EngineIDs) > 0 {
		q.Set("engine_id", strconv.Itoa(p.EngineIDs[0]))
	}
	if p.SeriesID > 0 {
		q.Set("series_id", strconv.Itoa(p.SeriesID))
	}
	if p.ReleasedFrom > 0 {
		q.Set("released_after", yearLowerBound(p.ReleasedFrom))
	}
	if p.ReleasedTo > 0 {
		q.Set("released_before", yearUpperBound(p.ReleasedTo))
	}
	if p.SearchIntro {
		q.Set("search_intro", "1")
	}
	if s := searchSortForCatalog(p.Sort); s != "" {
		q.Set("sort", s)
	}
	if p.Page > 0 {
		q.Set("page", strconv.Itoa(p.Page))
	}
	if p.Limit > 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	q.Set("include", "names,covers,refs")

	var data catalogWorksSearchData
	if err := c.getV1(ctx, "/catalog/works/search", q, &data); err != nil {
		return nil, err
	}
	out := Paginated[GalgameHit]{Total: data.Total}
	for i := range data.Items {
		out.Items = append(out.Items, catalogItemToHit(&data.Items[i]))
	}
	return &out, nil
}

type GalgameFullTag struct {
	GalgameID    int `json:"galgame_id"`
	TagID        int `json:"tag_id"`
	SpoilerLevel int `json:"spoiler_level"`
	Tag          Tag `json:"tag"`
}

type GalgameFullOfficial struct {
	GalgameID  int      `json:"galgame_id"`
	OfficialID int      `json:"official_id"`
	Official   Official `json:"official"`
}

type GalgameSeries struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type GalgameFull struct {
	ID               int     `json:"id"`
	CatalogWorkID    int64   `json:"catalog_work_id,omitempty"`
	VndbID           string  `json:"vndb_id"`
	ClaimState       string  `json:"claim_state"`
	NameEnUs         string  `json:"name_en_us"`
	NameZhCn         string  `json:"name_zh_cn"`
	NameJaJp         string  `json:"name_ja_jp"`
	NameZhTw         string  `json:"name_zh_tw"`
	Banner           string  `json:"banner"`
	IntroEnUs        string  `json:"intro_en_us"`
	IntroZhCn        string  `json:"intro_zh_cn"`
	IntroJaJp        string  `json:"intro_ja_jp"`
	IntroZhTw        string  `json:"intro_zh_tw"`
	ContentLimit     string  `json:"content_limit"`
	AgeLimit         string  `json:"age_limit"`
	OriginalLanguage string  `json:"original_language"`
	ReleaseDate      *string `json:"release_date"`

	Tag        []GalgameFullTag      `json:"tag"`
	Official   []GalgameFullOfficial `json:"official"`
	Characters []GalgameCharacter    `json:"characters"`
	Staff      []GalgameStaffGroup   `json:"staff"`
	Ratings    []GalgameRating       `json:"ratings"`
	Series     []GalgameSeries       `json:"series"`

	EffectiveBannerHash        string            `json:"effective_banner_hash"`
	EffectiveBannerWidth       int               `json:"effective_banner_width,omitempty"`
	EffectiveBannerHeight      int               `json:"effective_banner_height,omitempty"`
	EffectiveBannerThumbhash   string            `json:"effective_banner_thumbhash,omitempty"`
	EffectivePortraitHash      string            `json:"effective_portrait_hash,omitempty"`
	EffectivePortraitWidth     int               `json:"effective_portrait_width,omitempty"`
	EffectivePortraitHeight    int               `json:"effective_portrait_height,omitempty"`
	EffectivePortraitThumbhash string            `json:"effective_portrait_thumbhash,omitempty"`
	Covers                     []CoverInput      `json:"covers"`
	Screenshots                []ScreenshotInput `json:"screenshots"`
	Created                    string            `json:"created"`
	Updated                    string            `json:"updated"`
}

type GalgameDetailEnvelope struct {
	Galgame GalgameFull `json:"galgame"`
}

func (c *Client) GetGalgame(ctx context.Context, gid int, contentLimit string) (*GalgameDetailEnvelope, error) {
	catalogID, found, err := c.resolveGID(ctx, gid)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, &GalgameError{Code: galgameCodeNotFound, Message: "galgame not found"}
	}

	gate := gateFor(contentLimit)
	q := url.Values{}
	applyNSFW(q)
	q.Set("spoilers", "2")
	q.Set("include", "credits")

	var w catalogWork
	if err := c.getV1(ctx, fmt.Sprintf("/catalog/works/%d", catalogID), q, &w); err != nil {
		return nil, err
	}
	if !w.ClaimedBy.live() {
		return nil, &GalgameError{Code: galgameCodeNotFound, Message: "galgame not found"}
	}
	full := catalogWorkToFull(&w)
	if !gate.allows(full.ContentLimit) {
		return nil, &GalgameError{Code: galgameCodeNotFound, Message: "galgame not found"}
	}
	return &GalgameDetailEnvelope{Galgame: full}, nil
}

func (c *Client) CheckGalgameByVndbID(ctx context.Context, vndbID string) (exists bool, galgameID int, err error) {
	q := url.Values{}
	q.Set("source", "vndb")
	q.Set("external_id", vndbID)
	q.Set("nsfw", "1")

	data, status, err := c.getV1RawStatus(ctx, "/catalog/lookup", q)
	if err != nil {
		if catalogAbsent(status, err) {
			return false, 0, nil
		}
		return false, 0, err
	}

	var out struct {
		ClaimedBy *struct {
			Site   string `json:"site"`
			WorkID int64  `json:"work_id"`
		} `json:"claimed_by"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return false, 0, fmt.Errorf("解析 catalog lookup data 失败: %w", err)
	}
	if out.ClaimedBy == nil || !isGIDClaimSite(out.ClaimedBy.Site) {
		return false, 0, nil
	}
	return true, int(out.ClaimedBy.WorkID), nil
}

const BatchMaxIDs = 100

func (c *Client) GalgameBatch(ctx context.Context, ids []int, contentLimit string) ([]GalgameBrief, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) > CatalogWorksIDsMax {
		return nil, fmt.Errorf("GalgameBatch: %d ids exceeds the %d-id ceiling — chunk by client.CatalogWorksIDsMax", len(ids), CatalogWorksIDsMax)
	}
	byGID, err := c.resolveGIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	if len(byGID) == 0 {
		return nil, nil
	}
	catalogIDs := make([]int64, 0, len(byGID))
	for _, id := range byGID {
		catalogIDs = append(catalogIDs, id)
	}

	q := url.Values{}
	q.Set("ids", joinInt64s(catalogIDs))
	q.Set("include", "names,covers,refs")
	q.Set("limit", strconv.Itoa(CatalogWorksIDsMax))
	gateFor(contentLimit).apply(q)

	var data catalogWorksListData
	if err := c.getV1(ctx, "/catalog/works", q, &data); err != nil {
		return nil, err
	}
	out := make([]GalgameBrief, 0, len(data.Items))
	for i := range data.Items {
		it := &data.Items[i]
		if !it.ClaimedBy.live() {
			continue
		}
		out = append(out, catalogItemToBrief(it))
	}
	return out, nil
}

type GalgameCalendar struct {
	Month string              `json:"month"`
	Today string              `json:"today"`
	Items []GalgameBrief      `json:"items"`
	Meta  GalgameCalendarMeta `json:"meta"`
}

type GalgameCalendarMeta struct {
	PrevMonth string `json:"prev_month"`
	NextMonth string `json:"next_month"`
	HasPrev   bool   `json:"has_prev"`
	HasNext   bool   `json:"has_next"`
	MinMonth  string `json:"min_month"`
	MaxMonth  string `json:"max_month"`
	Count     int    `json:"count"`
}

const calendarPageLimit = 100

const calendarMaxPages = 50

func (c *Client) GetGalgameCalendar(ctx context.Context, month, contentLimit string) (*GalgameCalendar, error) {
	out := &GalgameCalendar{}
	cursor := ""
	for page := 0; page < calendarMaxPages; page++ {
		q := url.Values{}
		if month != "" {
			q.Set("month", month)
		}
		q.Set("include", "names,covers,refs")
		q.Set("limit", strconv.Itoa(calendarPageLimit))
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		gateFor(contentLimit).apply(q)

		var data catalogCalendarData
		if err := c.getV1(ctx, "/catalog/calendar", q, &data); err != nil {
			return nil, err
		}
		if page == 0 {
			out.Month = data.Month
			out.Today = data.Meta.Today
			out.Meta = GalgameCalendarMeta{
				PrevMonth: shiftMonth(data.Month, -1),
				NextMonth: shiftMonth(data.Month, +1),
				HasPrev:   derefBool(data.Meta.HasPrev),
				HasNext:   derefBool(data.Meta.HasNext),
				MinMonth:  data.Meta.MinMonth,
				MaxMonth:  data.Meta.MaxMonth,
				Count:     int(data.Count),
			}
		}
		for i := range data.Items {
			it := &data.Items[i]
			if !it.ClaimedBy.renderable() {
				continue
			}
			out.Items = append(out.Items, catalogItemToBrief(it))
		}
		if data.NextCursor == nil || *data.NextCursor == "" {
			break
		}
		cursor = *data.NextCursor
	}
	return out, nil
}

func derefBool(p *bool) bool { return p != nil && *p }

func shiftMonth(month string, n int) string {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		return ""
	}
	return t.AddDate(0, n, 0).Format("2006-01")
}

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
