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
	Site   string `json:"site"`
	WorkID int64  `json:"work_id"`
	State  string `json:"state"`
}

// The claim's site_work_id is the FORUM's page id, not this site's. Both
// downstreams answer to catalog site `kungal` and shared one gid space; this
// site's page id is the catalog work id since migration 037, kungal's is still
// the legacy number, and the claim names kungal's. Reading it as our own is
// what every "drew somebody else's game" incident was made of, so it leaves
// here under a name that says whose it is.
func (c *catalogClaimedBy) forumGID() int {
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

// This site's page id IS the catalog work id (migration 037), so there is
// nothing to elect any more. It used to prefer the claim's site_work_id and
// fall back to the catalog id, which is how one number could mean two games.
func (it *catalogWorkListItem) publicGID() int { return int(it.ID) }

func (w *catalogWork) publicGID() int { return int(w.ID) }

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
	ContentLimit  string            `json:"content_limit"`
	OLang         string            `json:"olang"`
	ReleaseDate   *string           `json:"release_date"`
	ClaimedBy     *catalogClaimedBy `json:"claimed_by"`
	Cover         string            `json:"cover"`
	Updated       string            `json:"updated"`

	Localized map[string]catalogLocalizedName `json:"localized"`
	Covers    *catalogCoverSlots              `json:"covers"`
	Refs      []catalogRef                    `json:"refs"`
	Labels    []catalogWorkLabel              `json:"labels"`
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
	ID          int64                           `json:"id"`
	DisplayName string                          `json:"display_name"`
	Localized   map[string]catalogLocalizedName `json:"localized"`
	LabelKind   string                          `json:"label_kind"`
	Kind        string                          `json:"kind"`
	// What the company DID on this work, kept apart from what it IS: catalog
	// sends attribution_role and company_kind separately and the two look
	// alike, "publisher" being a value of each. Kind above collapses them.
	Role     string `json:"role"`
	Lang     string `json:"lang"`
	LogoHash string `json:"logo_hash"`
}

type catalogWorkSeries struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	MemberCount int    `json:"member_count"`
}

type catalogWork struct {
	ID            int64             `json:"id"`
	Medium        string            `json:"medium"`
	DisplayName   string            `json:"display_name"`
	OLang         string            `json:"olang"`
	ContentRating string            `json:"content_rating"`
	ContentLimit  string            `json:"content_limit"`
	ReleaseDate   *string           `json:"release_date"`
	Created       string            `json:"created"`
	Updated       string            `json:"updated"`
	Refs          []catalogRef      `json:"refs"`
	ClaimedBy     *catalogClaimedBy `json:"claimed_by"`

	Localized map[string]catalogLocalizedName `json:"localized"`

	Intros      []catalogWorkIntro   `json:"intros"`
	Covers      []catalogDetailCover `json:"covers"`
	CoverSlots  *catalogCoverSlots   `json:"cover_slots"`
	Screenshots []catalogScreenshot  `json:"screenshots"`
	Tags        []catalogWorkTag     `json:"tags"`
	Labels      []catalogWorkLabel   `json:"labels"`

	Credits    []catalogCreditGroup   `json:"credits"`
	Characters []catalogWorkCharacter `json:"characters"`
	Ratings    []catalogRating        `json:"ratings"`
	Series     []catalogWorkSeries    `json:"series"`
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
