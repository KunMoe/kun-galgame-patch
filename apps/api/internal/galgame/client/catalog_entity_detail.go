package client

import (
	"context"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
)

const catalogStaffCreditsCap = 24

type catalogCharacterDetail struct {
	ID          int64                           `json:"id"`
	DisplayName string                          `json:"display_name"`
	Lang        string                          `json:"lang"`
	Latin       string                          `json:"latin"`
	Localized   map[string]catalogLocalizedName `json:"localized"`
	Aliases     []catalogAlias                  `json:"aliases"`
	Image       string                          `json:"image"`
	Figure      string                          `json:"figure"`
	Intros      []catalogIntroRow               `json:"intros"`
	Traits      []catalogCharacterTrait         `json:"traits"`
	Refs        []catalogRef                    `json:"refs"`
}

type catalogCharacterTrait struct {
	ID             int64                           `json:"id"`
	Name           string                          `json:"name"`
	Localized      map[string]catalogLocalizedName `json:"localized"`
	Group          string                          `json:"group"`
	GroupLocalized map[string]catalogLocalizedName `json:"group_localized"`
	Spoiler        int                             `json:"spoiler"`
	Sexual         bool                            `json:"sexual"`
	Lie            bool                            `json:"lie"`
}

type catalogNameDetail struct {
	ID          int64                           `json:"id"`
	DisplayName string                          `json:"display_name"`
	Lang        string                          `json:"lang"`
	Latin       string                          `json:"latin"`
	Localized   map[string]catalogLocalizedName `json:"localized"`
	Aliases     []catalogAlias                  `json:"aliases"`
	PhotoHash   string                          `json:"photo_hash"`
	Gender      *int                            `json:"gender"`
	BirthY      *int                            `json:"birth_y"`
	BirthM      *int                            `json:"birth_m"`
	BirthD      *int                            `json:"birth_d"`
	Siblings    []catalogPersonRef              `json:"siblings"`
	Intros      []catalogIntroRow               `json:"intros"`
	Refs        []catalogRef                    `json:"refs"`
	Links       []catalogEntityLink             `json:"links"`
	Credits     []catalogNameCredit             `json:"credits"`
}

type catalogNameCredit struct {
	Work  catalogWorkListItem `json:"work"`
	Roles []struct {
		RoleKey     string `json:"role_key"`
		RoleName    string `json:"role_name"`
		CharacterID int64  `json:"character_id"`
		Character   string `json:"character"`
	} `json:"roles"`
}

// Source is the site the text came from, already rendered for display — the
// modal prints it as "资料来自 X".
type GalgameEntityIntro struct {
	Lang    string `json:"lang"`
	Intro   string `json:"intro"`
	Source  string `json:"source,omitempty"`
	Machine bool   `json:"machine,omitempty"`
}

type GalgameEntityLink struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type GalgameCharacterTrait struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Group   string `json:"group"`
	Spoiler int    `json:"spoiler"`
	Lie     bool   `json:"lie"`
}

type GalgameCharacterDetail struct {
	ID         int                     `json:"id"`
	Name       KunLanguage             `json:"name"`
	Aliases    []string                `json:"aliases"`
	ImageHash  string                  `json:"image_hash,omitempty"`
	FigureHash string                  `json:"figure_hash,omitempty"`
	Intros     []GalgameEntityIntro    `json:"intros"`
	Traits     []GalgameCharacterTrait `json:"traits"`
	Links      []GalgameEntityLink     `json:"links"`
}

type GalgameStaffRole struct {
	RoleKey   string `json:"role_key"`
	RoleName  string `json:"role_name"`
	Character string `json:"character,omitempty"`
}

// GalgameStaffCredit is one work this person is credited on. GalgameID is 0 for
// a work no moyu galgame stands on, which is most of catalog — those rows name
// the work without linking anywhere.
type GalgameStaffCredit struct {
	GalgameID int                `json:"galgame_id"`
	Name      KunLanguage        `json:"name"`
	Roles     []GalgameStaffRole `json:"roles"`
}

type GalgameStaffDetail struct {
	ID        int                  `json:"id"`
	Name      KunLanguage          `json:"name"`
	Aliases   []string             `json:"aliases"`
	PhotoHash string               `json:"photo_hash,omitempty"`
	Gender    int                  `json:"gender,omitempty"`
	BirthY    int                  `json:"birth_y,omitempty"`
	BirthM    int                  `json:"birth_m,omitempty"`
	BirthD    int                  `json:"birth_d,omitempty"`
	Siblings  []GalgamePersonRef   `json:"siblings"`
	Intros    []GalgameEntityIntro `json:"intros"`
	Links     []GalgameEntityLink  `json:"links"`
	Credits   []GalgameStaffCredit `json:"credits"`
}

func (c *Client) GetCharacter(ctx context.Context, id int, contentLimit string) (*GalgameCharacterDetail, error) {
	q := url.Values{}
	applyNSFW(q)
	q.Set("spoilers", "2")

	var ch catalogCharacterDetail
	if err := c.getV1(ctx, "/catalog/characters/"+strconv.Itoa(id), q, &ch); err != nil {
		return nil, err
	}
	out := catalogCharacterToDetail(&ch, contentLimit != "nsfw" && contentLimit != "all")
	return &out, nil
}

func (c *Client) GetStaff(ctx context.Context, id int) (*GalgameStaffDetail, error) {
	q := url.Values{}
	applyNSFW(q)
	q.Set("include", "credits")
	q.Set("limit", strconv.Itoa(catalogStaffCreditsCap))

	var n catalogNameDetail
	if err := c.getV1(ctx, "/catalog/names/"+strconv.Itoa(id), q, &n); err != nil {
		return nil, err
	}
	out := catalogNameToDetail(&n)
	return &out, nil
}

func catalogCharacterToDetail(ch *catalogCharacterDetail, hideSexual bool) GalgameCharacterDetail {
	out := GalgameCharacterDetail{
		ID:         int(ch.ID),
		Name:       catalogEntityNames(ch.Localized, ch.DisplayName, ch.Lang, ch.Latin),
		Aliases:    catalogAliasValues(ch.Aliases),
		ImageHash:  hashFromURL(ch.Image),
		FigureHash: hashFromURL(ch.Figure),
		Intros:     catalogIntros(ch.Intros),
		Traits:     make([]GalgameCharacterTrait, 0, len(ch.Traits)),
		Links:      catalogRefLinks(ch.Refs, catalogCharacterPage),
	}
	for i := range ch.Traits {
		t := &ch.Traits[i]
		if hideSexual && t.Sexual {
			continue
		}
		name := catalogVocabularyName(t.Localized, t.Name)
		if name == "" {
			continue
		}
		out.Traits = append(out.Traits, GalgameCharacterTrait{
			ID:      int(t.ID),
			Name:    name,
			Group:   catalogVocabularyName(t.GroupLocalized, t.Group),
			Spoiler: t.Spoiler,
			Lie:     t.Lie,
		})
	}
	return out
}

func catalogNameToDetail(n *catalogNameDetail) GalgameStaffDetail {
	out := GalgameStaffDetail{
		ID:        int(n.ID),
		Name:      catalogEntityNames(n.Localized, n.DisplayName, n.Lang, n.Latin),
		Aliases:   catalogAliasValues(n.Aliases),
		PhotoHash: n.PhotoHash,
		Gender:    derefInt(n.Gender),
		BirthY:    derefInt(n.BirthY),
		BirthM:    derefInt(n.BirthM),
		BirthD:    derefInt(n.BirthD),
		Siblings:  make([]GalgamePersonRef, 0, len(n.Siblings)),
		Intros:    catalogIntros(n.Intros),
		Links:     catalogRefLinks(n.Refs, catalogStaffPage),
		Credits:   make([]GalgameStaffCredit, 0, len(n.Credits)),
	}
	for i := range n.Siblings {
		s := &n.Siblings[i]
		if name := s.names(); name.canonical() != "" {
			out.Siblings = append(out.Siblings, GalgamePersonRef{ID: int(s.ID), Name: name})
		}
	}
	for _, link := range n.Links {
		if link.URL == "" {
			continue
		}
		out.Links = append(out.Links, GalgameEntityLink{Name: linkDisplayName(link.Source, link.URL), URL: link.URL})
	}
	for i := range n.Credits {
		credit := &n.Credits[i]
		if !credit.Work.ClaimedBy.renderable() {
			continue
		}
		ja, zhCN, zhTW, en := namesOf(credit.Work.Localized)
		name := KunLanguage{EnUs: en, JaJp: ja, ZhCn: zhCN, ZhTw: zhTW}
		if name.canonical() == "" {
			name.JaJp = credit.Work.DisplayName
		}
		row := GalgameStaffCredit{
			GalgameID: credit.Work.ClaimedBy.gid(),
			Name:      name,
			Roles:     make([]GalgameStaffRole, 0, len(credit.Roles)),
		}
		// A person credited for the same job by two sources arrives as two rows
		// under two vocabulary keys, which the fold turns into "脚本 · 脚本".
		seen := make(map[string]bool, len(credit.Roles))
		for _, r := range credit.Roles {
			key, roleName := catalogFoldRole(r.RoleKey, r.RoleName)
			if seen[key+"\x00"+r.Character] {
				continue
			}
			seen[key+"\x00"+r.Character] = true
			row.Roles = append(row.Roles, GalgameStaffRole{RoleKey: key, RoleName: roleName, Character: r.Character})
		}
		sort.SliceStable(row.Roles, func(i, j int) bool {
			return catalogRoleWeight(row.Roles[i].RoleKey) < catalogRoleWeight(row.Roles[j].RoleKey)
		})
		out.Credits = append(out.Credits, row)
	}
	return out
}

func derefInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func catalogAliasValues(rows []catalogAlias) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.Value == "" || slices.Contains(out, r.Value) {
			continue
		}
		out = append(out, r.Value)
	}
	return out
}

func catalogIntros(rows []catalogIntroRow) []GalgameEntityIntro {
	out := make([]GalgameEntityIntro, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, r := range rows {
		lang := productLangFromCatalog(r.Lang)
		switch lang {
		case "ja-jp", "zh-cn", "zh-tw", "en-us":
		default:
			continue
		}
		if r.Intro == "" || seen[lang] {
			continue
		}
		seen[lang] = true
		out = append(out, GalgameEntityIntro{
			Lang: lang, Intro: r.Intro, Source: linkDisplayName(r.Source, ""), Machine: r.Machine,
		})
	}
	return out
}

// A trait's non-Chinese form is the English vocabulary token it was imported
// under, so this never honours a name preference: answering 金发 with Blond is
// neither a name nor Japanese.
func catalogVocabularyName(localized map[string]catalogLocalizedName, vocabulary string) string {
	for _, tag := range []string{"zh-Hans", "zh", "zh-Hant"} {
		if row, ok := localized[tag]; ok && row.Value != "" {
			return row.Value
		}
	}
	return vocabulary
}

var catalogCharacterPage = map[string]func(string) string{
	"vndb":    func(id string) string { return "https://vndb.org/" + id },
	"bangumi": func(id string) string { return "https://bgm.tv/character/" + id },
}

// vndb keys a person by a bare number here and prefixes it with s on the page,
// unlike the character refs, which already carry their c.
var catalogStaffPage = map[string]func(string) string{
	"vndb":    func(id string) string { return "https://vndb.org/s" + id },
	"bangumi": func(id string) string { return "https://bgm.tv/person/" + id },
	"erogamescape": func(id string) string {
		return "https://erogamescape.dyndns.org/~ap2/ero/toukei_kaiseki/creater.php?creater=" + id
	},
}

func catalogRefLinks(refs []catalogRef, page map[string]func(string) string) []GalgameEntityLink {
	out := make([]GalgameEntityLink, 0, len(refs))
	for _, ref := range refs {
		build, ok := page[ref.Source]
		if !ok || ref.ExternalID == "" {
			continue
		}
		out = append(out, GalgameEntityLink{Name: linkDisplayName(ref.Source, ""), URL: build(ref.ExternalID)})
	}
	return out
}

var catalogLinkSourceName = map[string]string{
	"official_site": "官方网站",
	"twitter":       "X",
	"vndb":          "VNDB",
	"bangumi":       "Bangumi",
	"erogamescape":  "批评空间",
	"dlsite":        "DLsite",
	"pixiv":         "pixiv",
}

func linkDisplayName(source, rawURL string) string {
	if name, ok := catalogLinkSourceName[strings.ToLower(strings.TrimSpace(source))]; ok {
		return name
	}
	if host := linkHost(rawURL); host != "" {
		return host
	}
	return source
}

func linkHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(u.Hostname(), "www.")
}

type catalogSeriesDetail struct {
	ID          int64             `json:"id"`
	DisplayName string            `json:"display_name"`
	Intros      []catalogIntroRow `json:"intros"`
	HasNSFW     bool              `json:"has_nsfw"`
}

type GalgameSeriesDetail struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	HasNSFW     bool   `json:"has_nsfw"`
}

func (c *Client) GetSeries(ctx context.Context, id int) (*GalgameSeriesDetail, error) {
	q := url.Values{}
	applyNSFW(q)

	var s catalogSeriesDetail
	if err := c.getV1(ctx, "/catalog/series/"+strconv.Itoa(id), q, &s); err != nil {
		return nil, err
	}
	out := &GalgameSeriesDetail{ID: int(s.ID), Name: s.DisplayName, HasNSFW: s.HasNSFW}
	for _, in := range s.Intros {
		if in.Intro != "" {
			out.Description = in.Intro
			break
		}
	}
	return out, nil
}
