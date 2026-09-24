package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"kun-galgame-patch-api/pkg/catalogv2"

	"github.com/redis/go-redis/v9"
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
	return e.HTTPStatus == http.StatusNotFound && (e.Code == catalogCodeNotFound || e.Code == galgameCodeNotFound)
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
	if gerr.Code != catalogCodeMoved || gerr.Moved <= 0 {
		return 0, false
	}
	if gerr.HTTPStatus != http.StatusMovedPermanently && gerr.HTTPStatus != http.StatusNotFound {
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
	v2 *catalogv2.Client
}

func NewWithKey(baseURL, apiKey string) *Client {
	return &Client{v2: catalogv2.New(baseURL, apiKey)}
}

func (c *Client) V2() *catalogv2.Client { return c.v2 }

// WithRedis hands the catalog client the shared read cache. Optional on
// purpose: a client built without it makes every read a request, which is what
// the tests want.
func (c *Client) WithRedis(rdb *redis.Client) *Client {
	c.v2.WithRedis(rdb)
	return c
}

type Paginated[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
}

type GalgameBrief struct {
	ID                         int               `json:"id"`
	ForumGID                   int               `json:"forum_gid,omitempty"`
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
	Maker                      *GalgameMaker     `json:"maker,omitempty"`
	Covers                     []CoverInput      `json:"covers"`
	Screenshots                []ScreenshotInput `json:"screenshots"`

	// The DLsite product this work is sold as, off catalog's refs. Server-side
	// only: the browser is handed the finished purchase URL, never the id it was
	// built from.
	DlsiteWorkno string `json:"-"`

	// Set only by the reads that ask for tags and credits, and serialized
	// because the taxonomy proxy re-parses these briefs out of this client's own
	// JSON: a `json:"-"` shelf arrived nil there and every tag / 会社 / 系列 page
	// rendered bare rows. applyGalgame lifts it onto the card, which is where
	// the browser reads it.
	Facet *GalgameFacet `json:"facet,omitempty"`
}

// The company a card credits. The name travels as four slots rather than one
// string because the reader's 标题语言 setting picks between them in the
// browser, the same way it does for a work title.
type GalgameMaker struct {
	ID   int         `json:"id"`
	Name KunLanguage `json:"name"`
}

type GalgameHit struct {
	ID                         int               `json:"id"`
	ForumGID                   int               `json:"forum_gid,omitempty"`
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
	EffectiveBannerHash        string            `json:"effective_banner_hash"`
	EffectiveBannerWidth       int               `json:"effective_banner_width,omitempty"`
	EffectiveBannerHeight      int               `json:"effective_banner_height,omitempty"`
	EffectiveBannerThumbhash   string            `json:"effective_banner_thumbhash,omitempty"`
	EffectivePortraitHash      string            `json:"effective_portrait_hash,omitempty"`
	EffectivePortraitWidth     int               `json:"effective_portrait_width,omitempty"`
	EffectivePortraitHeight    int               `json:"effective_portrait_height,omitempty"`
	EffectivePortraitThumbhash string            `json:"effective_portrait_thumbhash,omitempty"`
	Maker                      *GalgameMaker     `json:"maker,omitempty"`
	Covers                     []CoverInput      `json:"covers"`
	Screenshots                []ScreenshotInput `json:"screenshots"`
	TagIDs                     []int             `json:"tag_ids"`
	OfficialIDs                []int             `json:"official_ids"`
	EngineIDs                  []int             `json:"engine_ids"`
	Facet                      *GalgameFacet     `json:"facet,omitempty"`
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

// Every v2 read that builds a galgame card asks for the same blocks: a request
// that does not name one gets a bare id+name row for it, which renders as an
// empty slot rather than an error.
var cardInclude = []string{"titles", "covers", "refs", "companies"}

// A list card also prints a tag shelf and two credits. It is a separate set
// because credits is the heaviest block the works face answers — it carries
// every 声优 — and the reads that only need the axis or the header (the NSFW
// gate, the detail hydrate, the cron) throw all of it away.
var listCardInclude = []string{"titles", "covers", "refs", "companies", "tags", "credits"}

func worksQueryFor(p SearchGalgameParams) catalogv2.WorksQuery {
	q := catalogv2.WorksQuery{
		Q:            p.Q,
		Sort:         searchSortForCatalog(p.Sort),
		Page:         p.Page,
		Limit:        p.Limit,
		OLang:        joinCatalogLangs(p.OriginalLang),
		NSFW:         true,
		Include:      listCardInclude,
		IncludeTotal: true,
		Facets:       []string{"olang"},
		SearchIntro:  p.SearchIntro,
	}
	gate := gateFor(p.ContentLimit)
	q.ContentLimit = gate.contentLimit
	if p.AgeLimit == "r18" {
		q.ContentRating = "r18"
	}
	if len(p.TagIDs) > 0 {
		q.TagIDs = joinInts(p.TagIDs)
	}
	if len(p.OfficialIDs) > 0 {
		q.CompanyID = int64(p.OfficialIDs[0])
	}
	if len(p.EngineIDs) > 0 {
		q.EngineID = int64(p.EngineIDs[0])
	}
	if p.SeriesID > 0 {
		q.SeriesID = int64(p.SeriesID)
	}
	if p.ReleasedFrom > 0 {
		q.ReleasedAfter = yearLowerBound(p.ReleasedFrom)
	}
	if p.ReleasedTo > 0 {
		q.ReleasedBefore = yearUpperBound(p.ReleasedTo)
	}
	return q
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
	page, err := c.v2.ListWorks(ctx, worksQueryFor(p))
	if err != nil {
		return nil, catalogErr(err)
	}
	sexualOK := gateFor(p.ContentLimit).contentLimit != "sfw"
	out := Paginated[GalgameHit]{Total: page.Count()}
	for i := range page.Items {
		it := workToListItem(page.Items[i])
		if !it.ClaimedBy.renderable() || it.publicGID() == 0 {
			continue
		}
		hit := catalogItemToHit(&it)
		hit.Facet = facetOf(page.Items[i], sexualOK)
		out.Items = append(out.Items, hit)
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
	ID           int    `json:"id"`
	Name         string `json:"name"`
	GalgameCount int    `json:"galgame_count"`
}

type GalgameFull struct {
	ID               int     `json:"id"`
	ForumGID         int     `json:"forum_gid,omitempty"`
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
	Maker                      *GalgameMaker     `json:"maker,omitempty"`
	Covers                     []CoverInput      `json:"covers"`
	Screenshots                []ScreenshotInput `json:"screenshots"`
	Created                    string            `json:"created"`
	Updated                    string            `json:"updated"`
}

type GalgameDetailEnvelope struct {
	Galgame GalgameFull `json:"galgame"`
}

func (c *Client) GetGalgame(ctx context.Context, gid int, contentLimit string) (*GalgameDetailEnvelope, error) {
	w, err := c.v2.GetWork(ctx, int64(gid), true)
	if err != nil {
		return nil, catalogErr(err)
	}
	detail := workToDetail(*w)
	if !detail.ClaimedBy.renderable() {
		return nil, &GalgameError{Code: galgameCodeNotFound, Message: "galgame not found"}
	}
	full := catalogWorkToFull(&detail)
	if !gateFor(contentLimit).allows(full.ContentLimit) {
		return nil, &GalgameError{Code: galgameCodeNotFound, Message: "galgame not found"}
	}
	return &GalgameDetailEnvelope{Galgame: full}, nil
}

func (c *Client) CheckGalgameByVndbID(ctx context.Context, vndbID string) (exists bool, galgameID int, err error) {
	w, err := c.v2.WorkByRef(ctx, "vndb", vndbID, true)
	if err != nil {
		if errors.Is(err, catalogv2.ErrNotFound) {
			return false, 0, nil
		}
		return false, 0, catalogErr(err)
	}
	id, ok := catalogv2.ParseID(w.ID)
	if !ok || !claimedFrom(w.Claim).renderable() {
		return false, 0, nil
	}
	return true, int(id), nil
}

const BatchMaxIDs = 100

func (c *Client) GalgameBatch(ctx context.Context, ids []int, contentLimit string) ([]GalgameBrief, error) {
	return c.galgameBatch(ctx, ids, contentLimit, cardInclude)
}

// GalgameCardBatch hydrates the same rows for a list card, which prints a tag
// shelf and two credits beside the poster. Every list on the site draws that
// card, so the shelf rides the hydrate rather than a second read per page.
func (c *Client) GalgameCardBatch(ctx context.Context, ids []int, contentLimit string) ([]GalgameBrief, error) {
	return c.galgameBatch(ctx, ids, contentLimit, listCardInclude)
}

func (c *Client) galgameBatch(ctx context.Context, ids []int, contentLimit string, include []string) ([]GalgameBrief, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) > CatalogWorksIDsMax {
		return nil, fmt.Errorf("GalgameBatch: %d ids exceeds the %d-id ceiling — chunk by client.CatalogWorksIDsMax", len(ids), CatalogWorksIDsMax)
	}
	catalogIDs := catalogIDsOf(ids)
	if len(catalogIDs) == 0 {
		return nil, nil
	}

	gate := gateFor(contentLimit)
	page, err := c.v2.ListWorks(ctx, catalogv2.WorksQuery{
		IDs: catalogIDs, NSFW: true, Include: include,
		ContentLimit: gate.contentLimit, Limit: CatalogWorksIDsMax,
	})
	if err != nil {
		return nil, catalogErr(err)
	}
	out := make([]GalgameBrief, 0, len(page.Items))
	for i := range page.Items {
		it := workToListItem(page.Items[i])
		if !it.ClaimedBy.renderable() || it.publicGID() == 0 {
			continue
		}
		b := catalogItemToBrief(&it)
		b.Facet = facetOf(page.Items[i], gate.contentLimit != "sfw")
		out = append(out, b)
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
	if month == "" {
		month = time.Now().UTC().Format("2006-01")
	}
	out := &GalgameCalendar{
		Month: month,
		Today: time.Now().UTC().Format("2006-01-02"),
		Meta: GalgameCalendarMeta{
			PrevMonth: shiftMonth(month, -1),
			NextMonth: shiftMonth(month, +1),
			HasPrev:   true,
			HasNext:   true,
		},
	}
	cursor := ""
	for page := 0; page < calendarMaxPages; page++ {
		data, err := c.v2.Calendar(ctx, month, true, cursor, calendarPageLimit)
		if err != nil {
			return nil, catalogErr(err)
		}
		if page == 0 {
			out.Meta.Count = int(data.Count())
		}
		for i := range data.Items {
			it := workToListItem(data.Items[i])
			if !it.ClaimedBy.renderable() {
				continue
			}
			if contentLimit != "" {
				cl, _ := contentAxisOf(it.ContentLimit, it.ContentRating)
				if !gateFor(contentLimit).allows(cl) {
					continue
				}
			}
			out.Items = append(out.Items, catalogItemToBrief(&it))
		}
		if data.Next() == "" {
			break
		}
		cursor = data.Next()
	}
	return out, nil
}

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
