package client

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"kun-galgame-patch-api/pkg/catalogv2"
)

// The hand-written fakes in this package have twice kept the suite green while a
// catalog wave took production down (aliases[] in wave 209, the names block in
// wave 210): a fake serving the retired shape can never disagree with the code
// that reads it. These goldens are real /v2 responses to the exact requests this
// client makes, read on 2026-08-27 and trimmed to a few rows per array with
// every key intact. Refresh them by re-reading the live face, not by editing
// them to match the code.
func loadGolden[T any](t *testing.T, name string) T {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode: %v — one mismatched field type fails the WHOLE response", err)
	}
	return out
}

func TestProdWorkDetailDecodes(t *testing.T) {
	w := loadGolden[catalogv2.Work](t, "catalog_work_detail_prod.json")
	full := catalogWorkToFull(ptr(workToDetail(w)), false)

	if full.ID != 3 || full.VndbID != "v12984" {
		t.Fatalf("identity = (%d, %q), want (3, v12984)", full.ID, full.VndbID)
	}
	if full.NameZhCn == "" || full.NameJaJp == "" {
		t.Errorf("names = (%q, %q), want both filled from localized{}", full.NameZhCn, full.NameJaJp)
	}
	if full.IntroZhCn == "" {
		t.Error("intro_zh_cn is empty — the intros block did not reach the reader")
	}

	t.Run("roster", func(t *testing.T) {
		if len(full.Characters) == 0 {
			t.Fatal("no characters — include=characters names the block")
		}
		first := full.Characters[0]
		if first.Name.ZhCn != "科罗娜" || first.Name.JaJp == "" {
			t.Errorf("character[0] = %+v, want the zh-Hans row beside the original", first.Name)
		}
		if first.ImageHash == "" || first.FigureHash == "" {
			t.Errorf("character[0] art = (%q, %q), want both hashes", first.ImageHash, first.FigureHash)
		}
		if len(first.Voices) == 0 {
			t.Error("character[0] has no CV — voices[] is where the roster names them")
		}
	})

	t.Run("credits fold onto one role per job", func(t *testing.T) {
		keys := make([]string, 0, len(full.Staff))
		for _, g := range full.Staff {
			keys = append(keys, g.RoleKey)
		}
		for _, dead := range []string{"剧本", "原画", "音乐", "director-direction", "developer"} {
			if slices.Contains(keys, dead) {
				t.Errorf("role %q survived the fold: %v", dead, keys)
			}
		}
		for _, want := range []string{"scenario", "illustration", "music", "director"} {
			if !slices.Contains(keys, want) {
				t.Errorf("role %q missing after the fold: %v", want, keys)
			}
		}
	})

	t.Run("ratings", func(t *testing.T) {
		if len(full.Ratings) != 2 {
			t.Fatalf("ratings = %+v, want vndb and bangumi", full.Ratings)
		}
		for _, r := range full.Ratings {
			if len(r.Distribution) == 0 {
				t.Errorf("%s carries no histogram — which sources have one changes over time, so this is a warning, not a contract", r.Source)
			}
		}
	})

	// spoiler defaults to none upstream, so a request that does not raise the
	// ceiling loses these rows and leaves the page's 剧透 control with nothing
	// to reveal.
	t.Run("spoilered tags survive the ceiling", func(t *testing.T) {
		if !slices.ContainsFunc(full.Tag, func(tg GalgameFullTag) bool { return tg.SpoilerLevel > 0 }) {
			t.Errorf("no spoilered tag in %d rows", len(full.Tag))
		}
	})

	t.Run("cover slots", func(t *testing.T) {
		if full.EffectivePortraitHash == "" {
			t.Error("no portrait hash — the patch header renders a 3/4 box from it")
		}
		if full.EffectivePortraitWidth != 850 || full.EffectivePortraitHeight != 1080 {
			t.Errorf("portrait dims = %dx%d, want the slot's own",
				full.EffectivePortraitWidth, full.EffectivePortraitHeight)
		}
	})
}

func TestProdWorkCompanyLogoDecodes(t *testing.T) {
	w := loadGolden[catalogv2.Work](t, "catalog_work_company_logo_prod.json")
	full := catalogWorkToFull(ptr(workToDetail(w)), false)

	if len(full.Official) == 0 {
		t.Fatal("no companies — include=companies names the block")
	}
	o := full.Official[0].Official
	if o.Name != "ブロッコリー" || o.Category != "publisher" {
		t.Errorf("company = %+v, want the registry class, not the attribution role", o)
	}
	if o.LogoHash == "" {
		t.Error("no logo hash — the 会社 chip renders its brand mark from it")
	}
}

func ptr[T any](v T) *T { return &v }
