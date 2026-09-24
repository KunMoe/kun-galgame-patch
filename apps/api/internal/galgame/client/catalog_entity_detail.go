package client

import (
	"context"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"

	"kun-galgame-patch-api/pkg/catalogv2"
)

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
	ID         int         `json:"id"`
	Name       KunLanguage `json:"name"`
	Aliases    []string    `json:"aliases"`
	ImageHash  string      `json:"image_hash,omitempty"`
	FigureHash string      `json:"figure_hash,omitempty"`
	Gender     int         `json:"gender,omitempty"`
	// A character birthday carries no year — catalog publishes MM-DD — so the
	// two parts travel apart rather than as a date the frontend would have to
	// invent a year for.
	BirthM int                     `json:"birth_m,omitempty"`
	BirthD int                     `json:"birth_d,omitempty"`
	Intros []GalgameEntityIntro    `json:"intros"`
	Traits []GalgameCharacterTrait `json:"traits"`
	Links  []GalgameEntityLink     `json:"links"`
}

type GalgameStaffRole struct {
	RoleKey   string `json:"role_key"`
	RoleName  string `json:"role_name"`
	Character string `json:"character,omitempty"`
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
}

func (c *Client) GetCharacter(ctx context.Context, id int, contentLimit string) (*GalgameCharacterDetail, error) {
	ch, err := c.v2.GetCharacter(ctx, int64(id), true)
	if err != nil {
		return nil, err
	}
	out := catalogCharacterToDetail(ch, contentLimit != "nsfw" && contentLimit != "all")
	return &out, nil
}

func (c *Client) GetStaff(ctx context.Context, id int) (*GalgameStaffDetail, error) {
	n, err := c.v2.GetCreditName(ctx, int64(id), true)
	if err != nil {
		return nil, err
	}
	out := catalogNameToDetail(n)
	return &out, nil
}

func staffRoles(roles []catalogv2.NameCreditRole) []GalgameStaffRole {
	out := make([]GalgameStaffRole, 0, len(roles))
	// A person credited for the same job by two sources arrives as two rows
	// under two vocabulary keys, which the fold turns into "脚本 · 脚本".
	seen := make(map[string]bool, len(roles))
	for _, r := range roles {
		key, roleName := catalogFoldRole(r.RoleKey, r.RoleName)
		character := strOrEmpty(r.CharacterName)
		if seen[key+"\x00"+character] {
			continue
		}
		seen[key+"\x00"+character] = true
		out = append(out, GalgameStaffRole{RoleKey: key, RoleName: roleName, Character: character})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return catalogRoleWeight(out[i].RoleKey) < catalogRoleWeight(out[j].RoleKey)
	})
	return out
}

func genderInt(g *string) int {
	if g == nil {
		return 0
	}
	switch *g {
	case "male":
		return 1
	case "female":
		return 2
	default:
		return 0
	}
}

func catalogCharacterToDetail(ch *catalogv2.Character, hideSexual bool) GalgameCharacterDetail {
	id, _ := ch.IntID()
	out := GalgameCharacterDetail{
		ID: int(id),
		Name: catalogEntityNames(
			localizedFrom(ch.Localized), ch.DisplayName, strOrEmpty(ch.Lang), strOrEmpty(ch.Latin),
		),
		Aliases:    catalogAliasValues(aliasRowsFrom(ch.Aliases)),
		ImageHash:  imageHash(ch.Image),
		FigureHash: imageHash(ch.Figure),
		Gender:     genderInt(ch.Gender),
		Intros:     catalogIntros(introRowsFrom(ch.Intros)),
		Traits:     make([]GalgameCharacterTrait, 0, len(ch.Traits)),
		Links:      catalogRefLinks(refRowsFrom(ch.Refs), catalogCharacterPage),
	}
	out.BirthM, out.BirthD = monthDay(strOrEmpty(ch.Birthday))
	for i := range ch.Traits {
		t := &ch.Traits[i]
		if hideSexual && t.IsSexual {
			continue
		}
		name := catalogVocabularyName(localizedFrom(t.Localized), t.DisplayName)
		if name == "" {
			continue
		}
		tid, _ := catalogv2.ParseID(t.ID)
		out.Traits = append(out.Traits, GalgameCharacterTrait{
			ID:      int(tid),
			Name:    name,
			Group:   catalogVocabularyName(localizedFrom(t.GroupLocalized), strOrEmpty(t.Group)),
			Spoiler: spoilerInt(t.Spoiler),
			Lie:     t.IsLie,
		})
	}
	return out
}

func catalogNameToDetail(n *catalogv2.CreditName) GalgameStaffDetail {
	id, _ := n.IntID()
	out := GalgameStaffDetail{
		ID: int(id),
		Name: catalogEntityNames(
			localizedFrom(n.Localized), n.DisplayName, strOrEmpty(n.Lang), strOrEmpty(n.Latin),
		),
		Aliases:   catalogAliasValues(aliasRowsFrom(n.Aliases)),
		PhotoHash: imageHash(n.Photo),
		Gender:    genderInt(n.Gender),
		BirthY:    intOrZero(n.BirthYear),
		BirthM:    intOrZero(n.BirthMonth),
		BirthD:    intOrZero(n.BirthDay),
		Siblings:  make([]GalgamePersonRef, 0, len(n.Siblings)),
		Intros:    catalogIntros(introRowsFrom(n.Intros)),
		Links:     catalogRefLinks(refRowsFrom(n.Refs), catalogStaffPage),
	}
	for _, s := range personRefsFrom(n.Siblings) {
		if name := s.names(); name.canonical() != "" {
			out.Siblings = append(out.Siblings, GalgamePersonRef{ID: int(s.ID), Name: name})
		}
	}
	for _, link := range linkRowsFrom(n.Links) {
		if link.URL == "" {
			continue
		}
		out.Links = append(out.Links, GalgameEntityLink{Name: linkDisplayName(link.Source, link.URL), URL: link.URL})
	}
	return out
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

// A character birthday is "MM-DD" with no year at all, so this parses two parts
// and not a date.
func monthDay(s string) (int, int) {
	if len(s) != 5 || s[2] != '-' {
		return 0, 0
	}
	m, merr := strconv.Atoi(s[:2])
	d, derr := strconv.Atoi(s[3:])
	if merr != nil || derr != nil {
		return 0, 0
	}
	return m, d
}
