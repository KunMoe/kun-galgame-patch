package catalogv2

type Work struct {
	Object        string                   `json:"object"`
	ID            string                   `json:"id"`
	Medium        string                   `json:"medium"`
	DisplayName   string                   `json:"display_name"`
	Latin         *string                  `json:"latin"`
	Localized     map[string]LocalizedText `json:"localized"`
	OLang         string                   `json:"olang"`
	ContentRating string                   `json:"content_rating"`
	ContentLimit  string                   `json:"content_limit"`
	ReleaseDate   *string                  `json:"release_date"`
	CreatedAt     string                   `json:"created_at"`
	UpdatedAt     string                   `json:"updated_at"`
	Cover         *Image                   `json:"cover"`
	Banner        *Image                   `json:"banner"`
	Claim         *Claim                   `json:"claim"`
	Titles        *[]WorkTitle             `json:"titles"`
	Refs          *[]Ref                   `json:"refs"`
	Credits       *[]CreditGroup           `json:"credits"`
	Ratings       *[]Rating                `json:"ratings"`
	Tags          *[]WorkTag               `json:"tags"`
	Intros        *[]Intro                 `json:"intros"`
	Covers        *[]Cover                 `json:"covers"`
	Screenshots   *[]Screenshot            `json:"screenshots"`
	Characters    *[]WorkCharacter         `json:"characters"`
	Companies     *[]WorkCompany           `json:"companies"`
	Series        *[]WorkSeriesRef         `json:"series"`
	ViaCompany    *ViaCompany              `json:"via_company"`
}

// Named only on the company_id + company_rollup lane, and only on rows the
// queried company reaches through an imprint or subsidiary.
type ViaCompany struct {
	ID          string                   `json:"id"`
	DisplayName string                   `json:"display_name"`
	Localized   map[string]LocalizedText `json:"localized"`
}

type WorkSeriesRef struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Source      string `json:"source"`
	MemberCount int    `json:"member_count"`
}

func (w Work) IntID() (int64, bool) { return ParseID(w.ID) }

func (c WorkCompany) IntID() (int64, bool) { return ParseID(c.ID) }

func (ch WorkCharacter) IntID() (int64, bool) { return ParseID(ch.ID) }

type WorkTitle struct {
	Lang      string `json:"lang"`
	Title     string `json:"title"`
	TitleKind string `json:"title_kind"`
	IsMachine bool   `json:"is_machine"`
}

type CreditGroup struct {
	RoleKey  string        `json:"role_key"`
	RoleName string        `json:"role_name"`
	Credits  []CreditEntry `json:"credits"`
}

type CreditEntry struct {
	ID            string                   `json:"id"`
	DisplayName   string                   `json:"display_name"`
	Latin         *string                  `json:"latin"`
	Lang          *string                  `json:"lang"`
	Localized     map[string]LocalizedText `json:"localized"`
	CharacterID   *string                  `json:"character_id"`
	CharacterName *string                  `json:"character_name"`
}

type Rating struct {
	Source    string  `json:"source"`
	Score     float64 `json:"score"`
	VoteCount int     `json:"vote_count"`
	Rank      *int    `json:"rank"`
	// Bucket values are the source's own scale, and erogamescape's are deciles,
	// so score is a number rather than a count index. Present on the work detail
	// face only, never on a list face.
	Distribution *[]RatingBucket `json:"distribution"`
}

type RatingBucket struct {
	Score float64 `json:"score"`
	Count int     `json:"count"`
}

type WorkTag struct {
	ID          *string `json:"id"`
	DisplayName string  `json:"display_name"`
	Source      string  `json:"source"`
	Tier        *string `json:"tier"`
	TagKind     *string `json:"tag_kind"`
	Spoiler     string  `json:"spoiler"`
	IsSexual    bool    `json:"is_sexual"`
	WorkCount   *int    `json:"work_count"`
}

type Intro struct {
	Lang      string `json:"lang"`
	Value     string `json:"value"`
	IsMachine bool   `json:"is_machine"`
	Source    string `json:"source"`
}

type Cover struct {
	ID             string  `json:"id"`
	VoteCount      int     `json:"vote_count"`
	PortraitPinned bool    `json:"portrait_pinned"`
	URL            string  `json:"url"`
	Hash           string  `json:"hash"`
	Width          *int    `json:"width"`
	Height         *int    `json:"height"`
	Thumbhash      *string `json:"thumbhash"`
	Sexual         *string `json:"sexual"`
	Violence       *string `json:"violence"`
	Source         string  `json:"source"`
}

type Screenshot struct {
	Hash      string  `json:"hash"`
	URL       string  `json:"url"`
	Caption   string  `json:"caption"`
	Sexual    *string `json:"sexual"`
	Violence  *string `json:"violence"`
	Source    string  `json:"source"`
	Width     *int    `json:"width"`
	Height    *int    `json:"height"`
	Thumbhash *string `json:"thumbhash"`
}

type WorkCharacter struct {
	ID          string                   `json:"id"`
	DisplayName string                   `json:"display_name"`
	Latin       *string                  `json:"latin"`
	Lang        *string                  `json:"lang"`
	Localized   map[string]LocalizedText `json:"localized"`
	RosterRole  string                   `json:"roster_role"`
	Spoiler     string                   `json:"spoiler"`
	Image       *Image                   `json:"image"`
	Figure      *Image                   `json:"figure"`
	Voices      []CreditName             `json:"voices"`
}

type WorkCompany struct {
	ID              string                   `json:"id"`
	DisplayName     string                   `json:"display_name"`
	Localized       map[string]LocalizedText `json:"localized"`
	CompanyKind     string                   `json:"company_kind"`
	AttributionRole string                   `json:"attribution_role"`
	WorkCount       int                      `json:"work_count"`
	Logo            *Image                   `json:"logo"`
}
