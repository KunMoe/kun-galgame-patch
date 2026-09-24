package client

import (
	"encoding/json"
	"slices"
	"testing"

	"kun-galgame-patch-api/pkg/catalogv2"
)

// Every block below arrives only because the request names it: a detail face
// fetched with no include= answers a bare id+name row, which renders an empty
// modal without erroring.
func TestProdCharacterDetailDecodes(t *testing.T) {
	wire := loadGolden[catalogv2.Character](t, "catalog_character_detail_prod.json")
	ch := catalogCharacterToDetail(&wire, false)

	if ch.ID != 1699 || ch.Name.JaJp != "コロナ" || ch.Name.ZhCn != "科罗娜" {
		t.Fatalf("identity = (%d, %+v), want 1699 under both names", ch.ID, ch.Name)
	}
	if ch.ImageHash == "" || ch.FigureHash == "" {
		t.Errorf("art = (%q, %q), want both image objects read as hashes", ch.ImageHash, ch.FigureHash)
	}
	if !slices.Contains(ch.Aliases, "Corona") {
		t.Errorf("aliases = %v, want the alias rows read as values", ch.Aliases)
	}

	t.Run("traits render in Chinese", func(t *testing.T) {
		if len(ch.Traits) == 0 {
			t.Fatal("no traits survived")
		}
		if ch.Traits[0].Name != "金发" || ch.Traits[0].Group != "毛发" {
			t.Errorf("trait[0] = %+v, want the localized vocabulary, not Blond/Hair", ch.Traits[0])
		}
		if !slices.ContainsFunc(ch.Traits, func(tr GalgameCharacterTrait) bool { return tr.Spoiler > 0 }) {
			t.Error("no spoiler trait — the modal gates them behind a reveal, so it needs one")
		}
	})

	t.Run("sfw callers do not see the sexual traits", func(t *testing.T) {
		for _, tr := range ch.Traits {
			if tr.Group == "性行为" {
				t.Fatalf("sexual trait %q reached an sfw reader", tr.Name)
			}
		}
		if nsfw := catalogCharacterToDetail(&wire, true); len(nsfw.Traits) <= len(ch.Traits) {
			t.Errorf("nsfw traits = %d, sfw = %d — the filter dropped nothing",
				len(nsfw.Traits), len(ch.Traits))
		}
	})

	t.Run("intros land on moyu's language keys", func(t *testing.T) {
		langs := make([]string, 0, len(ch.Intros))
		for _, intro := range ch.Intros {
			langs = append(langs, intro.Lang)
		}
		for _, want := range []string{"en-us", "ja-jp", "zh-cn"} {
			if !slices.Contains(langs, want) {
				t.Errorf("intro langs = %v, want %s among them", langs, want)
			}
		}
		for _, intro := range ch.Intros {
			if intro.Lang == "zh-cn" && !intro.Machine {
				t.Error("the zh intro lost its machine flag — the reader is told when it is a translation")
			}
			if intro.Source == "bangumi" {
				t.Error("the intro credit still prints catalog's source key rather than Bangumi")
			}
		}
	})

	t.Run("external links", func(t *testing.T) {
		if len(ch.Links) == 0 {
			t.Fatal("no links built from refs")
		}
		if !slices.ContainsFunc(ch.Links, func(l GalgameEntityLink) bool { return l.URL == "https://vndb.org/c17039" }) {
			t.Errorf("links = %+v, want vndb's own c-prefixed id used verbatim", ch.Links)
		}
	})
}

func TestProdNameDetailDecodes(t *testing.T) {
	wire := loadGolden[catalogv2.CreditName](t, "catalog_name_detail_prod.json")
	n := catalogNameToDetail(&wire)

	if n.ID != 1550 || n.Name.JaJp != "榎木実佳" || n.Name.ZhCn != "榎木实佳" {
		t.Fatalf("identity = (%d, %+v), want 1550 under both names", n.ID, n.Name)
	}
	if n.PhotoHash == "" {
		t.Error("photo_hash is empty — the modal renders an avatar from it")
	}
	// The person behind the name owns these; the detail face reaches them
	// through person_id, so no second fetch pays for them.
	if n.Gender != 2 || n.BirthM != 8 || n.BirthD != 21 {
		t.Errorf("profile = (gender %d, %d-%d), want the nullable columns unwrapped", n.Gender, n.BirthM, n.BirthD)
	}
	if len(n.Siblings) == 0 {
		t.Error("no siblings — a voice actor's other 名义 are the point of the person link")
	}

	t.Run("links merge the refs with catalog's own rows", func(t *testing.T) {
		var hasVndb, hasSite bool
		for _, l := range n.Links {
			if l.URL == "https://vndb.org/s26940" {
				hasVndb = true
			}
			if l.Name == "官方网站" {
				hasSite = true
			}
		}
		if !hasVndb {
			t.Errorf("links = %+v, want a person's bare vndb id prefixed with s", n.Links)
		}
		if !hasSite {
			t.Errorf("links = %+v, want catalog's own link rows kept beside the refs", n.Links)
		}
	})
}

// Name 900 is credited on work 3 as other-staff, scenario AND 剧本 — the same
// job written by two sources plus a catch-all. Rendering the roles as they
// arrive prints "其他 · 脚本 · 脚本" under the work.
func TestProdNameCreditsFoldOneRolePerJob(t *testing.T) {
	page := loadGolden[catalogv2.List[catalogv2.NameCredit]](t, "catalog_name_credits_prod.json")
	if len(page.Items) == 0 {
		t.Fatal("no credits")
	}

	credits := make([][]GalgameStaffRole, 0, len(page.Items))
	for i := range page.Items {
		credits = append(credits, staffRoles(page.Items[i].Roles))
	}

	for i, roles := range credits {
		seen := make(map[string]bool, len(roles))
		for _, role := range roles {
			key := role.RoleKey + "\x00" + role.Character
			if seen[key] {
				t.Errorf("credit[%d] repeats %q: %+v", i, role.RoleName, roles)
			}
			seen[key] = true
		}
	}
	if it := workToListItem(page.Items[0].Work); it.publicGID() == 0 {
		t.Errorf("credit[0] = %+v, want the kungal claim read as a moyu galgame id", it)
	}
	if first := credits[0]; len(first) != 2 || first[0].RoleKey != "scenario" {
		t.Errorf("roles = %+v, want 脚本 before the other-staff catch-all", first)
	}
}

// The 会社 page renders its brand mark, description, official site and aliases
// from these four; the v2 cutover shipped without them and the page went blank
// below the title.
func TestProdCompanyDetailDecodes(t *testing.T) {
	rec := loadGolden[catalogv2.Company](t, "catalog_company_detail_prod.json")

	if imageHash(rec.Logo) == "" {
		t.Error("no logo hash")
	}
	if preferredIntro(introRowsFrom(rec.Intros)) == "" {
		t.Error("no description")
	}
	if !slices.Contains(aliasValues(aliasRowsFrom(rec.Aliases)), "Broccoli") {
		t.Errorf("aliases = %v, want the alias rows read as values", aliasValues(aliasRowsFrom(rec.Aliases)))
	}
	if links := linkRowsFrom(rec.Links); len(links) == 0 || links[0].URL == "" {
		t.Errorf("links = %+v, want the official site among them", links)
	}
	if productLangFromCatalog(strOrEmpty(rec.Lang)) != "ja-jp" {
		t.Errorf("lang = %q, want ja-jp", strOrEmpty(rec.Lang))
	}
}

// A nil slice marshals to null, and the two modals read
// `detail.links.length` / `detail.siblings.length` without a guard: the v2
// cutover shipped every block nil and both threw
// "Cannot read properties of null (reading 'length')" instead of rendering
// without the missing data.
func TestEntityDetailNeverMarshalsNullArrays(t *testing.T) {
	for name, v := range map[string]any{
		"character": catalogCharacterToDetail(&catalogv2.Character{}, false),
		"staff":     catalogNameToDetail(&catalogv2.CreditName{}),
	} {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("%s: marshal: %v", name, err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, key := range []string{"aliases", "intros", "traits", "links", "siblings", "credits"} {
			if got, ok := fields[key]; ok && string(got) == "null" {
				t.Errorf("%s.%s = null, want []", name, key)
			}
		}
	}
}
