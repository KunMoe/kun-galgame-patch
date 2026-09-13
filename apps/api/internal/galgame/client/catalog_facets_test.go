package client

import (
	"context"
	"strings"
	"testing"
)

func TestFacetTagsPicksTheDistinctiveCoreShelf(t *testing.T) {
	tags := []catalogWorkTag{
		{Name: "Galgame", CanonicalID: 4, Tier: "hidden", Kind: "meta", Count: 5068},
		{Name: "司田カズヒロ", Tier: "", Kind: "", Count: 0},
		{Name: "男性主人公", CanonicalID: 10, Tier: "core", Kind: "content", Count: 7465},
		{Name: "萝莉", CanonicalID: 1444, Tier: "longtail", Kind: "content", Count: 518},
		{Name: "北欧神话", CanonicalID: 20, Tier: "core", Kind: "content", Count: 32},
		{Name: "破处", CanonicalID: 30, Tier: "core", Kind: "content", Count: 4248, Sexual: true},
		{Name: "结局", CanonicalID: 40, Tier: "core", Kind: "content", Count: 99, Spoiler: 1},
		{Name: "超能力", CanonicalID: 50, Tier: "core", Kind: "content", Count: 174},
	}

	got := facetTags(tags)
	want := []string{"北欧神话", "超能力", "破处", "男性主人公"}
	if len(got) != len(want) {
		t.Fatalf("picked %d tags, want %d: %+v", len(got), len(want), got)
	}
	for i, name := range want {
		if got[i].name != name {
			t.Errorf("tag %d = %q, want %q", i, got[i].name, name)
		}
	}

	entry := facetEntry{tags: got}
	sfw := entry.facet(false)
	for _, tag := range sfw.Tags {
		if tag.Name == "破处" {
			t.Error("a sexual tag reached an SFW reader's shelf")
		}
	}
	if len(entry.facet(true).Tags) != len(want) {
		t.Errorf("nsfw shelf = %d tags, want %d", len(entry.facet(true).Tags), len(want))
	}
}

func TestFacetFromReadsOnlyTheTwoCreditedRoles(t *testing.T) {
	credits := []catalogCreditGroup{
		{RoleKey: "voice-actor", Credits: []catalogCreditItem{
			{catalogPersonRef: catalogPersonRef{ID: 1, DisplayName: "声優A"}},
		}},
		// 剧本 and 原画 are the same roles under bangumi's vocabulary; the fold
		// is what makes them findable under the shared key.
		{RoleKey: "剧本", Credits: []catalogCreditItem{
			{catalogPersonRef: catalogPersonRef{ID: 2, DisplayName: "なかひろ"}},
		}},
		{RoleKey: "illustration", Credits: []catalogCreditItem{
			{catalogPersonRef: catalogPersonRef{ID: 3, DisplayName: "司田カズヒロ"}},
			{catalogPersonRef: catalogPersonRef{ID: 4, DisplayName: "なつめえり"}},
			{catalogPersonRef: catalogPersonRef{ID: 5, DisplayName: "ミズタマ"}},
		}},
	}

	got := facetFrom(catalogWork{Credits: credits})
	if len(got.scenario) != 1 || got.scenario[0].JaJp != "なかひろ" {
		t.Errorf("scenario = %+v, want the folded 剧本 credit", got.scenario)
	}
	if len(got.illustration) != facetStaffMax {
		t.Errorf("illustration = %d names, want the %d-name cap", len(got.illustration), facetStaffMax)
	}
}

// The shelf used to cost a second works read per page, keyed back to the cards
// by gid. It rides the hydrate now, and since migration 037 there is no gid
// lookup in front of it either: a page is exactly one works read, and that read
// has to name tags and credits or every card in every list renders bare.
func TestGalgameCardBatchDrawsTheShelfInTheHydrateRead(t *testing.T) {
	srv := newCatalogFake(t)
	briefs, err := NewWithKey(srv.URL, "nm_test_key").
		GalgameCardBatch(context.Background(), []int{7}, "sfw")
	if err != nil {
		t.Fatalf("GalgameCardBatch: %v", err)
	}
	srv.wantPaths(t, "/v2/catalog/works")

	if got, want := srv.last().query.Get("include"), strings.Join(listCardInclude, ","); got != want {
		t.Errorf("include = %q, want %q", got, want)
	}
	if len(briefs) != 1 {
		t.Fatalf("briefs = %+v, want the one hydrated card", briefs)
	}
	f := briefs[0].Facet
	if f == nil {
		t.Fatal("facet = nil, want the shelf the read paid for")
	}
	if len(f.Tags) != 1 || f.Tags[0].Name != "北欧神话" || f.Tags[0].ID != 20 {
		t.Errorf("tags = %+v, want the core content shelf", f.Tags)
	}
	if len(f.Scenario) != 1 || f.Scenario[0].JaJp != "なかひろ" {
		t.Errorf("scenario = %+v, want the credited writer", f.Scenario)
	}
}

// credits is the heaviest block the works face answers — it carries every 声优
// — and the NSFW gate, the detail header and the cron all discard it.
func TestGalgameBatchStaysLean(t *testing.T) {
	srv := newCatalogFake(t)
	briefs, err := NewWithKey(srv.URL, "nm_test_key").
		GalgameBatch(context.Background(), []int{7}, "sfw")
	if err != nil {
		t.Fatalf("GalgameBatch: %v", err)
	}
	if got, want := srv.last().query.Get("include"), strings.Join(cardInclude, ","); got != want {
		t.Errorf("include = %q, want %q", got, want)
	}
	if len(briefs) != 1 || briefs[0].Facet != nil {
		t.Errorf("facet = %+v, want none on the lean read", briefs[0].Facet)
	}
}
