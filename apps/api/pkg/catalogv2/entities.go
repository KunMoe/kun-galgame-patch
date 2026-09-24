package catalogv2

import (
	"context"
	"net/url"
	"strconv"
	"strings"
)

type Company struct {
	ID          string                   `json:"id"`
	DisplayName string                   `json:"display_name"`
	Latin       *string                  `json:"latin"`
	Lang        *string                  `json:"lang"`
	Localized   map[string]LocalizedText `json:"localized"`
	CompanyKind string                   `json:"company_kind"`
	WorkCount   int                      `json:"work_count"`
	Aliases     []EntityName             `json:"aliases"`
	Logo        *Image                   `json:"logo"`
	Intros      []Intro                  `json:"intros"`
	Links       []Link                   `json:"links"`
}

func (c Company) IntID() (int64, bool) { return ParseID(c.ID) }

type CreditName struct {
	ID          string                   `json:"id"`
	DisplayName string                   `json:"display_name"`
	Latin       *string                  `json:"latin"`
	Lang        *string                  `json:"lang"`
	Localized   map[string]LocalizedText `json:"localized"`
	PersonID    *string                  `json:"person_id"`
	// gender and the three birth parts are person-level facts the detail face
	// reaches through person_id; they need no include token and no second fetch.
	// The parts are independently fuzzy: a year can exist with no month or day.
	Gender     *string      `json:"gender"`
	BirthYear  *int         `json:"birth_year"`
	BirthMonth *int         `json:"birth_month"`
	BirthDay   *int         `json:"birth_day"`
	Aliases    []EntityName `json:"aliases"`
	Photo      *Image       `json:"photo"`
	Siblings   []CreditName `json:"siblings"`
	Intros     []Intro      `json:"intros"`
	Links      []Link       `json:"links"`
	Refs       []Ref        `json:"refs"`
}

func (n CreditName) IntID() (int64, bool) { return ParseID(n.ID) }

type NameCredit struct {
	Work  Work             `json:"work"`
	Roles []NameCreditRole `json:"roles"`
}

type NameCreditRole struct {
	RoleKey       string  `json:"role_key"`
	RoleName      string  `json:"role_name"`
	CharacterID   *string `json:"character_id"`
	CharacterName *string `json:"character_name"`
}

// An Appearance reaches a work through a roster row OR through a voice credit.
// Reached the second way there is no roster row to read, so roster_role is
// "unknown" and spoiler "none" — absence of a strength, not a weak one.
type Appearance struct {
	Work       Work         `json:"work"`
	RosterRole string       `json:"roster_role"`
	Spoiler    string       `json:"spoiler"`
	Voices     []CreditName `json:"voices"`
}

type Character struct {
	ID          string                   `json:"id"`
	DisplayName string                   `json:"display_name"`
	Latin       *string                  `json:"latin"`
	Lang        *string                  `json:"lang"`
	Localized   map[string]LocalizedText `json:"localized"`
	Gender      *string                  `json:"gender"`
	Birthday    *string                  `json:"birthday"`
	Image       *Image                   `json:"image"`
	Figure      *Image                   `json:"figure"`
	Traits      []CharacterTrait         `json:"traits"`
	Aliases     []EntityName             `json:"aliases"`
	Intros      []Intro                  `json:"intros"`
	Refs        []Ref                    `json:"refs"`
}

type CharacterTrait struct {
	ID             string                   `json:"id"`
	DisplayName    string                   `json:"display_name"`
	Group          *string                  `json:"group"`
	Localized      map[string]LocalizedText `json:"localized"`
	GroupLocalized map[string]LocalizedText `json:"group_localized"`
	Spoiler        string                   `json:"spoiler"`
	IsSexual       bool                     `json:"is_sexual"`
	IsLie          bool                     `json:"is_lie"`
}

func (ch Character) IntID() (int64, bool) { return ParseID(ch.ID) }

type Tag struct {
	ID          string                   `json:"id"`
	DisplayName string                   `json:"display_name"`
	Tier        string                   `json:"tier"`
	TagKind     string                   `json:"tag_kind"`
	IsSexual    bool                     `json:"is_sexual"`
	WorkCount   int                      `json:"work_count"`
	Localized   map[string]LocalizedText `json:"localized"`
	Intros      []Intro                  `json:"intros"`
}

func (t Tag) IntID() (int64, bool) { return ParseID(t.ID) }

type Series struct {
	Object      string `json:"object"`
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	WorkCount   int    `json:"work_count"`
	// Counted over every live-claimed member, NOT narrowed by the nsfw= this
	// request sends — so it stays true for a series whose members an SFW reader
	// sees none of, which is the only way that page can explain its own
	// emptiness.
	HasNSFW bool    `json:"has_nsfw"`
	Intros  []Intro `json:"intros"`
}

func (s Series) IntID() (int64, bool) { return ParseID(s.ID) }

// A detail face answers a bare id+name row for every block the request does not
// name, so an entity fetched with no include= renders an empty page without
// erroring. The tokens are each face's own closed enum — an unknown one is a
// 400 — so these lists track catalog's collect specs.
var (
	companyInclude    = []string{"aliases", "logo", "intros", "links"}
	creditNameInclude = []string{"aliases", "photo", "siblings", "intros", "links", "refs"}
	characterInclude  = []string{
		"gender", "birthday", "image", "figure", "traits", "aliases", "intros", "refs",
	}
	tagInclude    = []string{"intros"}
	seriesInclude = []string{"intros"}
)

func entityPath(prefix string, id int64, include []string, spoiler string, nsfw bool) string {
	v := url.Values{}
	v.Set("include", strings.Join(include, ","))
	if spoiler != "" {
		v.Set("spoiler", spoiler)
	}
	if nsfw {
		v.Set("nsfw", "true")
	} else {
		v.Set("nsfw", "false")
	}
	return prefix + FormatID(id) + "?" + v.Encode()
}

func (c *Client) GetCompany(ctx context.Context, id int64, nsfw bool) (*Company, error) {
	var out Company
	if err := c.get(ctx, entityPath("/v2/catalog/companies/", id, companyInclude, "", nsfw), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetCreditName(ctx context.Context, id int64, nsfw bool) (*CreditName, error) {
	var out CreditName
	if err := c.get(ctx, entityPath("/v2/catalog/credit-names/", id, creditNameInclude, "", nsfw), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreditNameCredits(ctx context.Context, id int64, nsfw bool, cursor string, limit int) (*List[NameCredit], error) {
	return getPaged[NameCredit](ctx, c, "/v2/catalog/credit-names/"+FormatID(id)+"/credits", nsfw, cursor, limit)
}

func (c *Client) GetCharacter(ctx context.Context, id int64, nsfw bool) (*Character, error) {
	var out Character
	if err := c.get(ctx, entityPath("/v2/catalog/characters/", id, characterInclude, spoilerMajor, nsfw), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CharacterAppearances(ctx context.Context, id int64, nsfw bool, cursor string, limit int) (*List[Appearance], error) {
	return getPaged[Appearance](ctx, c, "/v2/catalog/characters/"+FormatID(id)+"/appearances", nsfw, cursor, limit)
}

func (c *Client) GetSeries(ctx context.Context, id int64, nsfw bool) (*Series, error) {
	var out Series
	if err := c.get(ctx, entityPath("/v2/catalog/series/", id, seriesInclude, "", nsfw), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetTag(ctx context.Context, id int64, nsfw bool) (*Tag, error) {
	var out Tag
	if err := c.get(ctx, entityPath("/v2/catalog/tags/", id, tagInclude, "", nsfw), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) EntityByRef(ctx context.Context, object, source, externalID string, nsfw bool) (id int64, err error) {
	v := url.Values{}
	v.Set("refs", source+":"+externalID)
	v.Set("limit", "1")
	if nsfw {
		v.Set("nsfw", "true")
	}
	path := "/v2/catalog/" + object + "s?" + v.Encode()
	if object == "company" {
		path = "/v2/catalog/companies?" + v.Encode()
	}
	if object == "credit_name" {
		path = "/v2/catalog/credit-names?" + v.Encode()
	}
	var page List[struct {
		ID string `json:"id"`
	}]
	if err := c.get(ctx, path, &page); err != nil {
		return 0, err
	}
	if len(page.Items) == 0 {
		return 0, Absent("GET /v2/catalog/" + object + "s?refs")
	}
	n, ok := ParseID(page.Items[0].ID)
	if !ok {
		return 0, Absent("GET /v2/catalog/" + object + "s?refs")
	}
	return n, nil
}

func getPaged[T any](ctx context.Context, c *Client, path string, nsfw bool, cursor string, limit int) (*List[T], error) {
	v := url.Values{}
	if limit > 0 {
		v.Set("limit", strconv.Itoa(limit))
	}
	if cursor != "" {
		v.Set("cursor", cursor)
	}
	if nsfw {
		v.Set("nsfw", "true")
	} else {
		v.Set("nsfw", "false")
	}
	q := v.Encode()
	if q != "" {
		path += "?" + q
	}
	var out List[T]
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	if out.Items == nil {
		out.Items = []T{}
	}
	return &out, nil
}
