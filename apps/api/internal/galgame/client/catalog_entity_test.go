package client

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestDetailCarriesTheCatalogEntityGraph(t *testing.T) {
	srv := newCatalogFake(t)
	c := NewWithKey(srv.URL, "nm_test_key")
	ctx := context.Background()

	env, err := c.GetGalgame(ctx, 7, "")
	if err != nil {
		t.Fatalf("GetGalgame: %v", err)
	}
	g := &env.Galgame

	t.Run("the request asks for credits", func(t *testing.T) {
		if !strings.Contains(srv.last().query.Get("include"), "credits") {
			t.Fatalf("include = %q, want credits in the v2 include set", srv.last().query.Get("include"))
		}
	})

	t.Run("characters carry every name catalog has", func(t *testing.T) {
		if len(g.Characters) != 2 {
			t.Fatalf("characters = %d, want 2", len(g.Characters))
		}
		first := g.Characters[0]
		if first.Name.JaJp != "コロナ" || first.Name.ZhCn != "科罗娜" {
			t.Errorf("character = %+v, want both slots — the reader's 标题语言 picks one", first.Name)
		}
		if first.ImageHash != "chara1" || first.FigureHash != "figure1" {
			t.Errorf("art = (%q, %q), want the URL basenames", first.ImageHash, first.FigureHash)
		}
		if first.Voices == nil {
			t.Errorf("voices = nil, want an empty slice rather than a missing field")
		}
		if second := g.Characters[1]; second.Name.JaJp != "雪々" || second.Name.ZhCn != "" {
			t.Errorf("character[1] = %+v, want an untagged display name parked in ja-jp", second.Name)
		}
	})

	t.Run("staff folds the source vocabularies onto one role", func(t *testing.T) {
		keys := make([]string, 0, len(g.Staff))
		for _, group := range g.Staff {
			keys = append(keys, group.RoleKey)
		}
		want := []string{"scenario", "illustration", "voice-actor", "other-staff"}
		if !slices.Equal(keys, want) {
			t.Fatalf("role keys = %v, want %v", keys, want)
		}

		scenario := g.Staff[0]
		if scenario.RoleName != "脚本" {
			t.Errorf("scenario role name = %q, want the pinned 脚本", scenario.RoleName)
		}
		if len(scenario.People) != 2 {
			t.Fatalf("scenario people = %+v, want both vocabularies' credits in one group", scenario.People)
		}
		if scenario.People[1].Name.ZhCn != "丸户史明" {
			t.Errorf("scenario[1] = %+v, want the zh name too", scenario.People[1].Name)
		}

		voice := g.Staff[2]
		played := []KunLanguage{{JaJp: "コロナ", ZhCn: "科罗娜", MachineTranslated: []string{"zh-cn"}}}
		if len(voice.People) != 1 || !reflect.DeepEqual(voice.People[0].Characters, played) {
			t.Errorf("voice-actor = %+v, want the annotation resolved through the roster", voice.People)
		}

		other := g.Staff[3]
		if len(other.People) != 1 || other.People[0].Name.JaJp != "なかひろ" {
			t.Errorf("other-staff = %+v, want 保住圭 dropped — it already ran under 脚本", other.People)
		}
	})

	t.Run("developer stays out of the staff list", func(t *testing.T) {
		for _, group := range g.Staff {
			if group.RoleKey == "developer" {
				t.Fatal("developer reached the staff list — the 会社 chips already name it")
			}
		}
	})

	t.Run("ratings keep each source's own scale", func(t *testing.T) {
		if len(g.Ratings) != 2 {
			t.Fatalf("ratings = %+v, want dlsite dropped for having no votes", g.Ratings)
		}
		if g.Ratings[0].Source != "vndb" || g.Ratings[0].Score != 8.1 {
			t.Errorf("ratings[0] = %+v, want vndb 8.1 verbatim", g.Ratings[0])
		}
		ero := g.Ratings[1]
		if ero.Score != 78.5 {
			t.Errorf("erogamescape score = %v, want 78.5 on its own 0-100 scale", ero.Score)
		}
		if ero.Rank == nil || *ero.Rank != 2917 {
			t.Errorf("erogamescape rank = %v, want 2917", ero.Rank)
		}
		if ero.VoteCount != 42 {
			t.Errorf("erogamescape vote_count = %d, want 42", ero.VoteCount)
		}
	})

	t.Run("the portrait slot reaches the header", func(t *testing.T) {
		if g.EffectivePortraitHash != "hash3" {
			t.Errorf("portrait hash = %q, want cover_slots.portrait", g.EffectivePortraitHash)
		}
		if g.EffectivePortraitWidth != 850 || g.EffectivePortraitHeight != 1080 {
			t.Errorf("portrait dims = %dx%d, want 850x1080", g.EffectivePortraitWidth, g.EffectivePortraitHeight)
		}
		if g.EffectiveBannerHash != "hash2" {
			t.Errorf("banner hash = %q, want the landscape cover — the portrait must not displace it", g.EffectiveBannerHash)
		}
	})
}

func TestListBriefCarriesThePortraitSlot(t *testing.T) {
	srv := newCatalogFake(t)
	c := NewWithKey(srv.URL, "nm_test_key")

	briefs, err := c.GalgameBatch(context.Background(), []int{7}, "")
	if err != nil {
		t.Fatalf("GalgameBatch: %v", err)
	}
	if len(briefs) != 1 {
		t.Fatalf("briefs = %d, want 1", len(briefs))
	}
	if got := briefs[0].EffectivePortraitHash; got != "hash1" {
		t.Errorf("portrait hash = %q, want covers.portrait", got)
	}
	if got := briefs[0].EffectiveBannerHash; got != "hash2" {
		t.Errorf("banner hash = %q, want covers.banner", got)
	}
}

func TestMakerPicksTheRoleNotTheOrder(t *testing.T) {
	label := func(id int64, name, role string, localized map[string]catalogLocalizedName) catalogWorkLabel {
		return catalogWorkLabel{ID: id, DisplayName: name, Role: role, Localized: localized}
	}

	t.Run("the publisher listed first does not win", func(t *testing.T) {
		m := makerOf([]catalogWorkLabel{
			label(39, "VISUAL ARTS", "publisher", nil),
			label(2, "Key", "developer", nil),
		})
		if m == nil || m.ID != 2 {
			t.Fatalf("maker = %+v, want the developer (id 2), not the row catalog sent first", m)
		}
	})

	t.Run("a doujin work credits its circle", func(t *testing.T) {
		m := makerOf([]catalogWorkLabel{
			label(7, "とある出版", "publisher", nil),
			label(8, "サークル", "circle", nil),
		})
		if m == nil || m.ID != 8 {
			t.Fatalf("maker = %+v, want the circle (id 8)", m)
		}
	})

	t.Run("localized names travel whole", func(t *testing.T) {
		m := makerOf([]catalogWorkLabel{
			label(5147, "ゆずソフト", "developer", map[string]catalogLocalizedName{
				"zh-Hans": {Value: "柚子软件"},
			}),
		})
		if m == nil {
			t.Fatal("maker = nil")
		}
		if m.Name.JaJp != "ゆずソフト" || m.Name.ZhCn != "柚子软件" {
			t.Errorf("name = %+v, want both slots — the 标题语言 setting picks in the browser", m.Name)
		}
	})

	t.Run("an unranked role loses to a ranked one whatever the order", func(t *testing.T) {
		m := makerOf([]catalogWorkLabel{
			label(1, "无角色", "", nil),
			label(2, "发行", "publisher", nil),
		})
		if m == nil || m.ID != 2 {
			t.Fatalf("maker = %+v, want the publisher (id 2)", m)
		}
	})

	t.Run("no companies means no maker", func(t *testing.T) {
		if m := makerOf(nil); m != nil {
			t.Errorf("maker = %+v, want nil", m)
		}
	})
}
