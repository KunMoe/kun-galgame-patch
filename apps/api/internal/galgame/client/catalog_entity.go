package client

import (
	"log/slog"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

// KunLanguage is moyu's four name slots. Entity names travel whole rather than
// flattened to one string because the reader's 标题语言 setting picks between
// them in the browser, the same way it does for a work title.
//
// A work's name also carries its display_name and latin whole, for a work whose
// original language none of the four slots speaks: the picker falls back to
// them rather than to a blank title.
type KunLanguage struct {
	EnUs string `json:"en-us"`
	JaJp string `json:"ja-jp"`
	ZhCn string `json:"zh-cn"`
	ZhTw string `json:"zh-tw"`

	DisplayName       string   `json:"display_name,omitempty"`
	Latin             string   `json:"latin,omitempty"`
	MachineTranslated []string `json:"machine_translated,omitempty"`
}

var kunSlots = []string{"ja-jp", "zh-cn", "zh-tw", "en-us"}

// canonical is the one name that stands for the entity when a slot cannot be
// chosen: deduplicating credits, and testing whether a name is empty at all.
func (n KunLanguage) canonical() string {
	for _, v := range []string{n.JaJp, n.ZhCn, n.EnUs, n.ZhTw, n.DisplayName, n.Latin} {
		if v != "" {
			return v
		}
	}
	return ""
}

func kunSlot(lang string) (string, bool) {
	switch slot := productLangFromCatalog(lang); slot {
	case "ja-jp", "zh-cn", "zh-tw", "en-us":
		return slot, true
	}
	return "", false
}

func kunLanguageOf(slots map[string]catalogLocalizedName) KunLanguage {
	n := KunLanguage{
		EnUs: slots["en-us"].Value, JaJp: slots["ja-jp"].Value,
		ZhCn: slots["zh-cn"].Value, ZhTw: slots["zh-tw"].Value,
	}
	for _, slot := range kunSlots {
		if row := slots[slot]; row.Machine && row.Value != "" {
			n.MachineTranslated = append(n.MachineTranslated, slot)
		}
	}
	return n
}

// catalogPersonRef is how the public face names a person inside another record:
// a roster voice or a credit row. The id is catalog's NAME id, which is what
// kungal's /galgame/staff/:id takes — not the person id behind it.
type catalogPersonRef struct {
	ID          int64                           `json:"id"`
	DisplayName string                          `json:"display_name"`
	Lang        string                          `json:"lang"`
	Latin       string                          `json:"latin"`
	Localized   map[string]catalogLocalizedName `json:"localized"`
}

type catalogWorkCharacter struct {
	ID           int64                           `json:"id"`
	DisplayName  string                          `json:"display_name"`
	Localized    map[string]catalogLocalizedName `json:"localized"`
	Latin        string                          `json:"latin"`
	Kind         string                          `json:"kind"`
	Spoiler      int                             `json:"spoiler"`
	Image        string                          `json:"image"`
	ImageSexual  *int                            `json:"image_sexual"`
	Figure       string                          `json:"figure"`
	FigureSexual *int                            `json:"figure_sexual"`
	Voices       []catalogPersonRef              `json:"voices"`
}

type catalogCreditItem struct {
	catalogPersonRef
	CharacterID int64 `json:"character_id"`
}

type catalogCreditGroup struct {
	RoleKey  string              `json:"role_key"`
	RoleName string              `json:"role_name"`
	Credits  []catalogCreditItem `json:"credits"`
}

// Score is the bucket's value on the source's own scale, not an index:
// erogamescape publishes deciles, so it is a number and not a count.
type catalogRatingBucket struct {
	Score float64 `json:"score"`
	Count int     `json:"count"`
}

type catalogRating struct {
	Source       string                `json:"source"`
	Score        float64               `json:"score"`
	VoteCount    int                   `json:"vote_count"`
	Rank         *int                  `json:"rank"`
	Distribution []catalogRatingBucket `json:"distribution"`
}

// GalgamePersonRef is one credited person. ID deep-links kungal's staff page.
type GalgamePersonRef struct {
	ID   int         `json:"id"`
	Name KunLanguage `json:"name"`
}

type GalgameCharacter struct {
	ID         int                `json:"id"`
	Name       KunLanguage        `json:"name"`
	Kind       string             `json:"kind"`
	Spoiler    int                `json:"spoiler"`
	ImageHash  string             `json:"image_hash,omitempty"`
	FigureHash string             `json:"figure_hash,omitempty"`
	Voices     []GalgamePersonRef `json:"voices"`
}

type GalgameStaffMember struct {
	ID         int           `json:"id"`
	Name       KunLanguage   `json:"name"`
	Characters []KunLanguage `json:"characters,omitempty"`
}

type GalgameStaffGroup struct {
	RoleKey  string               `json:"role_key"`
	RoleName string               `json:"role_name"`
	People   []GalgameStaffMember `json:"people"`
}

type GalgameRatingBucket struct {
	Score float64 `json:"score"`
	Count int     `json:"count"`
}

// GalgameRating is one external source's aggregate. Score sits on that source's
// own scale — vndb and bangumi 0-10, erogamescape 0-100, dlsite 0-5 — and
// catalog never normalizes between them, so nothing may compare two sources'
// scores without dividing by the per-source maximum the frontend holds.
// Distribution buckets are likewise the source's own: erogamescape's are
// deciles, everyone else's are points.
type GalgameRating struct {
	Source       string                `json:"source"`
	Score        float64               `json:"score"`
	VoteCount    int                   `json:"vote_count"`
	Rank         *int                  `json:"rank,omitempty"`
	Distribution []GalgameRatingBucket `json:"distribution,omitempty"`
}

// catalogEntityNames folds one entity's names onto moyu's four slots.
//
// display_name owns its own language slot and localized[] only fills the rest:
// character 1699 ships localized["ja"] = "Corona", a romanized spelling variant,
// beside display_name "コロナ". Letting localized win renders the roster in
// romaji for a reader who asked for Japanese. display_name carries its own tag;
// when catalog sends none it lands in ja-jp, which is what an untagged catalog
// display name almost always is.
func catalogEntityNames(localized map[string]catalogLocalizedName, displayName, lang, latin string) KunLanguage {
	slot, ok := kunSlot(lang)
	if !ok {
		slot = "ja-jp"
	}
	slots := localizedByProductKey(localized)
	if displayName != "" {
		slots[slot] = catalogLocalizedName{Value: displayName}
	}
	if latin != "" && slots["en-us"].Value == "" {
		slots["en-us"] = catalogLocalizedName{Value: latin}
	}
	return kunLanguageOf(slots)
}

func (p *catalogPersonRef) names() KunLanguage {
	return catalogEntityNames(p.Localized, p.DisplayName, p.Lang, p.Latin)
}

func namedPeople(refs []catalogPersonRef, ownerKey string, ownerID int64) []GalgamePersonRef {
	out := make([]GalgamePersonRef, 0, len(refs))
	for i := range refs {
		p := &refs[i]
		name := p.names()
		if name.canonical() == "" {
			slog.Warn("catalog credit name has no name; it is dropped", "credit_name", p.ID, ownerKey, ownerID)
			continue
		}
		out = append(out, GalgamePersonRef{ID: int(p.ID), Name: name})
	}
	return out
}

func catalogCharacters(rows []catalogWorkCharacter, revealSexual bool) []GalgameCharacter {
	out := make([]GalgameCharacter, 0, len(rows))
	for i := range rows {
		c := &rows[i]
		name := catalogEntityNames(c.Localized, c.DisplayName, "", c.Latin)
		if name.canonical() == "" {
			slog.Warn("catalog roster row has no name; the character is dropped", "character", c.ID)
			continue
		}
		out = append(out, GalgameCharacter{
			ID:         int(c.ID),
			Name:       name,
			Kind:       c.Kind,
			Spoiler:    c.Spoiler,
			ImageHash:  artFor(hashFromURL(c.Image), c.ImageSexual, revealSexual),
			FigureHash: artFor(hashFromURL(c.Figure), c.FigureSexual, revealSexual),
			Voices:     namedPeople(c.Voices, "character", c.ID),
		})
	}
	return out
}

// One work carries the same role under several keys, because each source names
// it in its own vocabulary: work 3 alone ships scenario AND 剧本, illustration
// AND 原画, music AND 音乐. Reading the keys as given renders every one of those
// as a separate 制作人员 row.
var catalogRoleFold = map[string]string{
	"剧本":                 "scenario",
	"原画":                 "illustration",
	"音乐":                 "music",
	"director-direction": "director",
}

var catalogRoleName = map[string]string{
	"scenario":     "脚本",
	"illustration": "原画",
	"music":        "音乐",
	"director":     "导演",
}

var catalogRoleOrder = []string{
	"原作",
	"scenario",
	"illustration",
	"character-design",
	"music",
	"voice-actor",
	"director",
	"composer",
	"lyric",
	"arrange",
	"vocal",
	"theme-song-composition",
	"theme-song-lyrics",
	"theme-song-performance",
	"inserted-song-performance",
}

// developer / publisher already render as the 会社 chips above the staff list.
var catalogRoleHidden = map[string]bool{"developer": true, "publisher": true}

const catalogRoleLast = "other-staff"

var catalogRoleRank = func() map[string]int {
	m := make(map[string]int, len(catalogRoleOrder))
	for i, key := range catalogRoleOrder {
		m[key] = i
	}
	return m
}()

func catalogRoleWeight(key string) int {
	if key == catalogRoleLast {
		return len(catalogRoleOrder) + 1
	}
	if r, ok := catalogRoleRank[key]; ok {
		return r
	}
	return len(catalogRoleOrder)
}

// catalogFoldRole collapses one source's vocabulary onto the shared key and
// answers the name moyu prints for it.
func catalogFoldRole(key, name string) (string, string) {
	if folded, ok := catalogRoleFold[key]; ok {
		key = folded
	}
	if pinned, ok := catalogRoleName[key]; ok {
		name = pinned
	}
	return key, name
}

func catalogStaff(groups []catalogCreditGroup, roster []catalogWorkCharacter) []GalgameStaffGroup {
	type bucket struct {
		name   string
		people []GalgameStaffMember
		at     map[string]int
	}
	rosterNames := rosterNameIndex(roster)
	order := make([]string, 0, len(groups))
	byKey := make(map[string]*bucket, len(groups))

	for gi := range groups {
		g := &groups[gi]
		if catalogRoleHidden[g.RoleKey] {
			continue
		}
		key, roleName := catalogFoldRole(g.RoleKey, g.RoleName)
		b := byKey[key]
		if b == nil {
			b = &bucket{name: roleName, at: map[string]int{}}
			byKey[key] = b
			order = append(order, key)
		}
		if b.name == "" {
			b.name = roleName
		}
		for ci := range g.Credits {
			c := &g.Credits[ci]
			name := c.names()
			norm := normalizeCreditName(name.canonical())
			if norm == "" {
				slog.Warn("catalog credit has no usable name; it is dropped",
					"credit_name", c.ID, "role_key", g.RoleKey, "name", name.canonical())
				continue
			}
			i, seen := b.at[norm]
			if !seen {
				b.at[norm] = len(b.people)
				b.people = append(b.people, GalgameStaffMember{ID: int(c.ID), Name: name})
				i = len(b.people) - 1
			} else {
				b.people[i].Name = mergeEntityNames(b.people[i].Name, name)
			}
			if played := creditCharacter(rosterNames, c); played.canonical() != "" {
				b.people[i].Characters = appendUniqueName(b.people[i].Characters, played)
			}
		}
	}

	if other := byKey[catalogRoleLast]; other != nil {
		elsewhere := make(map[string]bool)
		for key, b := range byKey {
			if key == catalogRoleLast {
				continue
			}
			for norm := range b.at {
				elsewhere[norm] = true
			}
		}
		kept := other.people[:0]
		for _, p := range other.people {
			if !elsewhere[normalizeCreditName(p.Name.canonical())] {
				kept = append(kept, p)
			}
		}
		other.people = kept
	}

	arrival := make(map[string]int, len(order))
	for i, key := range order {
		arrival[key] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		wi, wj := catalogRoleWeight(order[i]), catalogRoleWeight(order[j])
		if wi != wj {
			return wi < wj
		}
		return arrival[order[i]] < arrival[order[j]]
	})

	out := make([]GalgameStaffGroup, 0, len(order))
	for _, key := range order {
		b := byKey[key]
		if len(b.people) == 0 {
			continue
		}
		out = append(out, GalgameStaffGroup{RoleKey: key, RoleName: b.name, People: b.people})
	}
	return out
}

func rosterNameIndex(roster []catalogWorkCharacter) map[int64]KunLanguage {
	out := make(map[int64]KunLanguage, len(roster))
	for i := range roster {
		c := &roster[i]
		out[c.ID] = catalogEntityNames(c.Localized, c.DisplayName, "", c.Latin)
	}
	return out
}

// A work credit names the character it voiced only by character_id; the name
// lives on the same work's roster. Catalog never sent the character_name this
// used to fall back to: that field exists only on a name's credits face.
func creditCharacter(roster map[int64]KunLanguage, c *catalogCreditItem) KunLanguage {
	return roster[c.CharacterID]
}

// The same person reaches moyu as "保住圭" from one source and "保住圭 (Hozumi
// Kei)" from another; the parenthetical and the spacing are all that differ.
// Only a trailing parenthetical goes: a company's leading type marker is part
// of its name, and cutting at the first "(" turned "(有)PINA(ぴなぽんな)" into
// an empty key, which dropped every such credit from its game page.
func normalizeCreditName(name string) string {
	head, rest := "", name
	if strings.HasPrefix(rest, "(") || strings.HasPrefix(rest, "（") {
		if j := strings.IndexAny(rest, ")）"); j >= 0 {
			_, w := utf8.DecodeRuneInString(rest[j:])
			head, rest = rest[:j+w], rest[j+w:]
		}
	}
	if i := strings.IndexAny(rest, "(（"); i >= 0 {
		rest = rest[:i]
	}
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '　' {
			return -1
		}
		return r
	}, head+rest)
}

// Merging two credits for the same person keeps the shorter form of each slot,
// which is the one without the "(Hozumi Kei)" tail, and fills slots either of
// them left empty.
func mergeEntityNames(a, b KunLanguage) KunLanguage {
	slots := make(map[string]catalogLocalizedName, len(kunSlots))
	pick := func(slot, x, y string) {
		from := a
		if y != "" && (x == "" || len(y) < len(x)) {
			x, from = y, b
		}
		slots[slot] = catalogLocalizedName{Value: x, Machine: slices.Contains(from.MachineTranslated, slot)}
	}
	pick("en-us", a.EnUs, b.EnUs)
	pick("ja-jp", a.JaJp, b.JaJp)
	pick("zh-cn", a.ZhCn, b.ZhCn)
	pick("zh-tw", a.ZhTw, b.ZhTw)
	return kunLanguageOf(slots)
}

func appendUniqueName(slice []KunLanguage, val KunLanguage) []KunLanguage {
	if slices.ContainsFunc(slice, func(n KunLanguage) bool { return n.canonical() == val.canonical() }) {
		return slice
	}
	return append(slice, val)
}

func catalogRatings(rows []catalogRating) []GalgameRating {
	out := make([]GalgameRating, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		if r.Source == "" || r.VoteCount <= 0 {
			continue
		}
		buckets := make([]GalgameRatingBucket, 0, len(r.Distribution))
		for _, b := range r.Distribution {
			buckets = append(buckets, GalgameRatingBucket{Score: b.Score, Count: b.Count})
		}
		out = append(out, GalgameRating{
			Source:       r.Source,
			Score:        r.Score,
			VoteCount:    r.VoteCount,
			Rank:         r.Rank,
			Distribution: buckets,
		})
	}
	return out
}
