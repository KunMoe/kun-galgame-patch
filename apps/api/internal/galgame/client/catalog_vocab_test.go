package client

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	"kun-galgame-patch-api/pkg/catalogv2"
)

func intp(v int) *int { return &v }

func TestImageGradesReadBothClosedScales(t *testing.T) {
	for _, tc := range []struct {
		name  string
		grade func(*string) *int
		in    *string
		want  *int
	}{
		{"safe", sexualGrade, ptr("safe"), intp(0)},
		{"suggestive", sexualGrade, ptr("suggestive"), intp(1)},
		{"explicit", sexualGrade, ptr("explicit"), intp(2)},
		{"null sexual is unassessed, not safe", sexualGrade, nil, nil},
		{"a word off the scale is unassessed", sexualGrade, ptr("nude"), nil},
		{"tame", violenceGrade, ptr("tame"), intp(0)},
		{"violent", violenceGrade, ptr("violent"), intp(1)},
		{"brutal", violenceGrade, ptr("brutal"), intp(2)},
		{"null violence is unassessed", violenceGrade, nil, nil},
		{"a sexual word is not a violence grade", violenceGrade, ptr("explicit"), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.grade(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("grade = %v, want %v", deref(got), deref(tc.want))
			}
		})
	}
}

func deref(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func TestDetailCarriesAnUnassessedGradeAsNull(t *testing.T) {
	srv := newCatalogFake(t)
	env, err := NewWithKey(srv.URL, "nm_test_key").GetGalgame(context.Background(), 7, "sfw")
	if err != nil {
		t.Fatalf("GetGalgame: %v", err)
	}
	raw, err := json.Marshal(env.Galgame.Screenshots)
	if err != nil {
		t.Fatal(err)
	}
	var shots []struct {
		Hash     string `json:"image_hash"`
		Sexual   *int   `json:"sexual"`
		Violence *int   `json:"violence"`
	}
	if err := json.Unmarshal(raw, &shots); err != nil {
		t.Fatal(err)
	}
	want := map[string]*int{"shot1": intp(0), "shot2": nil, "shot3": intp(1)}
	if len(shots) != len(want) {
		t.Fatalf("screenshots = %+v, want three", shots)
	}
	for _, s := range shots {
		if !reflect.DeepEqual(s.Sexual, want[s.Hash]) {
			t.Errorf("%s sexual = %v, want %v", s.Hash, deref(s.Sexual), deref(want[s.Hash]))
		}
		if s.Violence != nil {
			t.Errorf("%s violence = %d, want null — catalog assesses none today", s.Hash, *s.Violence)
		}
	}
}

func TestCoversCarryCatalogsCoverKind(t *testing.T) {
	srv := newCatalogFake(t)
	env, err := NewWithKey(srv.URL, "nm_test_key").GetGalgame(context.Background(), 7, "")
	if err != nil {
		t.Fatalf("GetGalgame: %v", err)
	}
	kinds := make([]string, 0, len(env.Galgame.Covers))
	for _, c := range env.Galgame.Covers {
		kinds = append(kinds, c.Kind)
	}
	if !slices.Equal(kinds, []string{"main", "dig"}) {
		t.Errorf("cover kinds = %v, want cover_kind verbatim — the covers modal groups by it", kinds)
	}
}

func TestRosterArtIsGatedByItsOwnGrade(t *testing.T) {
	srv := newCatalogFake(t)
	c := NewWithKey(srv.URL, "nm_test_key")

	for _, tc := range []struct {
		contentLimit string
		wantChara2   string
	}{
		{"sfw", ""},
		{"", ""},
		{"all", "chara2"},
	} {
		t.Run("content_limit="+tc.contentLimit, func(t *testing.T) {
			env, err := c.GetGalgame(context.Background(), 7, tc.contentLimit)
			if err != nil {
				t.Fatalf("GetGalgame: %v", err)
			}
			chars := env.Galgame.Characters
			if chars[0].ImageHash != "chara1" || chars[0].FigureHash != "figure1" {
				t.Errorf("safe art = (%q, %q), want it shown to every reader", chars[0].ImageHash, chars[0].FigureHash)
			}
			if chars[1].ImageHash != tc.wantChara2 {
				t.Errorf("suggestive art = %q, want %q", chars[1].ImageHash, tc.wantChara2)
			}
		})
	}
}

func TestCharacterDetailArtIsGatedByItsGrade(t *testing.T) {
	wire := loadGolden[catalogv2.Character](t, "catalog_character_detail_prod.json")
	for _, tc := range []struct {
		name   string
		sexual *string
		reveal bool
		shown  bool
	}{
		{"safe art reaches an sfw reader", ptr("safe"), false, true},
		{"suggestive art does not", ptr("suggestive"), false, false},
		{"unassessed art does not", nil, false, false},
		{"an opted-in reader sees all of it", ptr("explicit"), true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ch := wire
			img := *wire.Image
			img.Sexual = tc.sexual
			ch.Image = &img
			got := catalogCharacterToDetail(&ch, tc.reveal)
			if (got.ImageHash != "") != tc.shown {
				t.Errorf("image_hash = %q, want shown=%v", got.ImageHash, tc.shown)
			}
		})
	}
}

func TestGenderReadsTheWholeVocabulary(t *testing.T) {
	for in, want := range map[string]int{"male": 1, "female": 2, "other": 3} {
		if got := genderInt(ptr(in)); got != want {
			t.Errorf("genderInt(%q) = %d, want %d", in, got, want)
		}
	}
	if got := genderInt(nil); got != 0 {
		t.Errorf("genderInt(nil) = %d, want 0 (unrecorded)", got)
	}
}

func TestWorkNamesNeverRenderBlank(t *testing.T) {
	t.Run("display_name fills its own language's slot", func(t *testing.T) {
		n := workNames(map[string]catalogLocalizedName{"en": {Value: "Title"}}, "标题", "zh-Hans", "")
		if n.ZhCn != "标题" || n.JaJp != "" || n.EnUs != "Title" {
			t.Errorf("names = %+v, want 标题 in zh-cn and nothing parked in ja-jp", n)
		}
	})

	t.Run("a language no slot speaks travels whole", func(t *testing.T) {
		n := workNames(nil, "타이틀", "ko", "Taiteul")
		if n.canonical() != "타이틀" || n.JaJp != "" || n.DisplayName != "타이틀" || n.Latin != "Taiteul" {
			t.Errorf("names = %+v, want every slot empty and display_name/latin carried", n)
		}
	})

	t.Run("localized keeps its own slot", func(t *testing.T) {
		n := workNames(map[string]catalogLocalizedName{"ja": {Value: "タイトル"}}, "別名", "ja", "")
		if n.JaJp != "タイトル" {
			t.Errorf("ja-jp = %q, want the localized row", n.JaJp)
		}
	})

	t.Run("machine translations are marked", func(t *testing.T) {
		n := workNames(map[string]catalogLocalizedName{
			"ja": {Value: "タイトル"}, "zh-Hans": {Value: "标题", Machine: true},
		}, "タイトル", "ja", "")
		if !slices.Equal(n.MachineTranslated, []string{"zh-cn"}) {
			t.Errorf("machine_translated = %v, want [zh-cn]", n.MachineTranslated)
		}
	})
}

func TestBriefCarriesTheNamePrimitiveAndThePrecision(t *testing.T) {
	latin := "Hanguk Geim"
	month := "month"
	date := "2026-06-01"
	w := catalogv2.Work{
		ID: "88", DisplayName: "한국 게임", Latin: &latin, OLang: "ko",
		ContentLimit: "sfw", ReleaseDate: &date, ReleasePrecision: &month,
		Localized: map[string]catalogv2.LocalizedText{"en": {Value: "Korean Game", IsMachine: true}},
	}
	it := workToListItem(w)
	b := catalogItemToBrief(&it)
	if b.DisplayName != "한국 게임" || b.Latin != latin || b.NameJaJp != "" {
		t.Errorf("brief names = %+v, want display_name and latin whole, nothing in ja-jp", b.Names())
	}
	if !slices.Equal(b.NameMachineTranslated, []string{"en-us"}) {
		t.Errorf("name_machine_translated = %v, want [en-us]", b.NameMachineTranslated)
	}
	if b.ReleasePrecision != "month" || b.ReleaseDate == nil || *b.ReleaseDate != date {
		t.Errorf("release = %v / %q, want %s at month precision", b.ReleaseDate, b.ReleasePrecision, date)
	}
	if got := catalogItemToHit(&it); got.DisplayName != b.DisplayName || got.ReleasePrecision != "month" {
		t.Errorf("hit = %+v, want the brief's display_name and precision", got)
	}
}
