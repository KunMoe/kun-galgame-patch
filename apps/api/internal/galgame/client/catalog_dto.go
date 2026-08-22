package client

const (
	catalogClaimStateLive    = "live"
	catalogClaimStateDraft   = "draft"
	catalogClaimStateHidden  = "hidden"
	catalogClaimStatePending = "pending"
)

// Claim sites are product identities, not external_ref source keys. Both
// spellings stay readable until the Wave 161 re-site has soaked.
const catalogClaimSiteKungal = "kungal"

const catalogClaimSiteLegacy = "galgame_wiki"

func isGIDClaimSite(site string) bool {
	return site == catalogClaimSiteKungal || site == catalogClaimSiteLegacy
}

type catalogClaimedBy struct {
	Site         string `json:"site"`
	WorkID       int64  `json:"work_id"`
	State        string `json:"state"`
	ContentLimit string `json:"content_limit"`
}

func (c *catalogClaimedBy) gid() int {
	if c == nil || !isGIDClaimSite(c.Site) {
		return 0
	}
	return int(c.WorkID)
}

func (c *catalogClaimedBy) renderable() bool {
	return c == nil || c.State != catalogClaimStateHidden
}

func (c *catalogClaimedBy) live() bool {
	return c != nil && isGIDClaimSite(c.Site) && c.State == catalogClaimStateLive
}

type catalogRef struct {
	Source     string `json:"source"`
	ExternalID string `json:"external_id"`
}

// The name primitive, keyed by catalog language tag (ja, zh-Hans, zh-Hant, en,
// and others the four fixed slots could not carry). Catalog wave 212 put it on
// works; the wave after it deletes the ja-jp/zh-cn/zh-tw/en-us block this client
// used to read, so nothing here may reach for those keys again. Election happens
// upstream — a source row already beat any machine translation for its tag.
type catalogLocalizedName struct {
	Value   string `json:"value"`
	Kind    string `json:"kind"`
	Machine bool   `json:"machine"`
}

type catalogCoverSlot struct {
	URL       string `json:"url"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Thumbhash string `json:"thumbhash"`
	Sexual    int    `json:"sexual"`
	Violence  int    `json:"violence"`
	Source    string `json:"source"`
}

type catalogCoverSlots struct {
	Portrait *catalogCoverSlot `json:"portrait"`
	Banner   *catalogCoverSlot `json:"banner"`
}

type catalogWorkListItem struct {
	ID            int64             `json:"id"`
	Medium        string            `json:"medium"`
	DisplayName   string            `json:"display_name"`
	ContentRating string            `json:"content_rating"`
	OLang         string            `json:"olang"`
	ReleaseDate   *string           `json:"release_date"`
	ClaimedBy     *catalogClaimedBy `json:"claimed_by"`
	Cover         string            `json:"cover"`
	Updated       string            `json:"updated"`

	Localized map[string]catalogLocalizedName `json:"localized"`
	Covers    *catalogCoverSlots              `json:"covers"`
	Refs      []catalogRef                    `json:"refs"`
}

type catalogWorksListData struct {
	Items      []catalogWorkListItem `json:"items"`
	NextCursor *string               `json:"next_cursor"`
}

type catalogWorksSearchData struct {
	Total int64                 `json:"total"`
	Page  int                   `json:"page"`
	Limit int                   `json:"limit"`
	Items []catalogWorkListItem `json:"items"`
}

type catalogCalendarMeta struct {
	Today    string `json:"today"`
	MinMonth string `json:"min_month"`
	MaxMonth string `json:"max_month"`
	HasPrev  *bool  `json:"has_prev"`
	HasNext  *bool  `json:"has_next"`
}

type catalogCalendarData struct {
	Month      string                `json:"month"`
	Year       string                `json:"year"`
	Count      int64                 `json:"count"`
	Items      []catalogWorkListItem `json:"items"`
	NextCursor *string               `json:"next_cursor"`
	Meta       catalogCalendarMeta   `json:"meta"`
}

type catalogWorkIntro struct {
	Lang    string `json:"lang"`
	Intro   string `json:"intro"`
	Source  string `json:"source"`
	Machine bool   `json:"machine"`
}

type catalogDetailCover struct {
	URL            string `json:"url"`
	Kind           string `json:"kind"`
	PortraitPinned bool   `json:"portrait_pinned"`
	Sexual         int    `json:"sexual"`
	Violence       int    `json:"violence"`
	Source         string `json:"source"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	Thumbhash      string `json:"thumbhash"`
}

type catalogScreenshot struct {
	URL       string `json:"url"`
	Caption   string `json:"caption"`
	Sexual    int    `json:"sexual"`
	Violence  int    `json:"violence"`
	Source    string `json:"source"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Thumbhash string `json:"thumbhash"`
}

type catalogWorkTag struct {
	Name        string `json:"name"`
	Count       int    `json:"count"`
	Source      string `json:"source"`
	CanonicalID int64  `json:"canonical_id"`
	Tier        string `json:"tier"`
	Kind        string `json:"kind"`
	Spoiler     int    `json:"spoiler"`
	Sexual      bool   `json:"sexual"`
}

type catalogWorkLabel struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"display_name"`
	LabelKind   string `json:"label_kind"`
	Kind        string `json:"kind"`
	Lang        string `json:"lang"`
	LogoHash    string `json:"logo_hash"`
}

type catalogWorkEngine struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type catalogWorkLink struct {
	Source string `json:"source"`
	URL    string `json:"url"`
}

type catalogWorkSeries struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type catalogWork struct {
	ID            int64             `json:"id"`
	Medium        string            `json:"medium"`
	DisplayName   string            `json:"display_name"`
	OLang         string            `json:"olang"`
	ContentRating string            `json:"content_rating"`
	ReleaseDate   *string           `json:"release_date"`
	Created       string            `json:"created"`
	Updated       string            `json:"updated"`
	Refs          []catalogRef      `json:"refs"`
	ClaimedBy     *catalogClaimedBy `json:"claimed_by"`

	Localized map[string]catalogLocalizedName `json:"localized"`

	// The detail face is mid-rename from `intro` to `intros` and the two sides
	// do not deploy together, so both keys are decoded until catalog drops the
	// singular one. Read them through introRows, never directly.
	Intro       []catalogWorkIntro   `json:"intro"`
	Intros      []catalogWorkIntro   `json:"intros"`
	Covers      []catalogDetailCover `json:"covers"`
	CoverSlots  *catalogCoverSlots   `json:"cover_slots"`
	Screenshots []catalogScreenshot  `json:"screenshots"`
	Tags        []catalogWorkTag     `json:"tags"`
	Labels      []catalogWorkLabel   `json:"labels"`
	Engines     []catalogWorkEngine  `json:"engines"`
	Links       []catalogWorkLink    `json:"links"`
	Series      []catalogWorkSeries  `json:"series"`

	// credits arrives only when the request asks for include=credits; the other
	// two are always on the detail face.
	Credits    []catalogCreditGroup   `json:"credits"`
	Characters []catalogWorkCharacter `json:"characters"`
	Ratings    []catalogRating        `json:"ratings"`
}

type catalogLookupPair struct {
	Source     string `json:"source"`
	ExternalID string `json:"external_id"`
}

type catalogLookupBatchRequest struct {
	Items []catalogLookupPair `json:"items"`
}

type catalogLookupBatchItem struct {
	Source     string            `json:"source"`
	ExternalID string            `json:"external_id"`
	Work       *catalogWorkBrief `json:"work"`
	ClaimedBy  *catalogClaimedBy `json:"claimed_by"`
}

type catalogWorkBrief struct {
	ID            int64  `json:"id"`
	Medium        string `json:"medium"`
	DisplayName   string `json:"display_name"`
	ContentRating string `json:"content_rating"`
}

type catalogLookupBatchData struct {
	Items []catalogLookupBatchItem `json:"items"`
}

type catalogLookupData struct {
	Work      *catalogWorkBrief `json:"work"`
	ClaimedBy *catalogClaimedBy `json:"claimed_by"`
}
