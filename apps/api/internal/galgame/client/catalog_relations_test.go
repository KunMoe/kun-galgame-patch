package client

import (
	"context"
	"slices"
	"testing"

	"kun-galgame-patch-api/pkg/catalogv2"
)

func rel(relationType, phrase, id, medium, limit, rating string, releaseDate *string, claimState string) catalogv2.WorkRelation {
	w := catalogv2.Work{
		ID: id, Medium: medium, DisplayName: "w" + id, OLang: "ja",
		ContentLimit: limit, ContentRating: rating, ReleaseDate: releaseDate,
	}
	if claimState != "" {
		w.Claim = &catalogv2.Claim{Site: "kungal", SiteWorkID: id, State: claimState}
	}
	return catalogv2.WorkRelation{RelationType: relationType, Phrase: phrase, Work: w}
}

func ids(rows []GalgameRelated) []int {
	out := make([]int, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}

func TestRelatedWorksMergeSortAndGate(t *testing.T) {
	d2010, d2015 := "2010-05-01", "2015-01-01"
	rels := []catalogv2.WorkRelation{
		rel("adaptation_of", "被改编为", "30", "anime", "sfw", "all_ages", nil, ""),
		rel("same_series", "同系列", "21", "galgame", "sfw", "all_ages", nil, "live"),
		rel("same_series", "同系列", "20", "galgame", "sfw", "all_ages", &d2015, "live"),
		rel("same_series", "同系列", "22", "galgame", "sfw", "all_ages", &d2010, "live"),
		rel("side_story_of", "有外传", "10", "galgame", "sfw", "all_ages", nil, "live"),
		rel("fandisc_of", "有 Fandisc", "10", "galgame", "sfw", "all_ages", nil, "live"),
		rel("side_story_of", "有外传", "10", "galgame", "sfw", "all_ages", nil, "live"),
		rel("future_token", "将来的关系", "40", "galgame", "sfw", "all_ages", nil, "live"),
		rel("sequel_of", "有续作", "50", "galgame", "nsfw", "r18", nil, "live"),
		rel("sequel_of", "有续作", "60", "galgame", "sfw", "all_ages", nil, "hidden"),
	}

	t.Run("sfw reader", func(t *testing.T) {
		got := relatedWorks(&rels, "sfw")
		// fandisc ranks above same_series; within same_series dated works come
		// first by date and the undated one last; an unknown token sorts after
		// adaptation_of; the r18 sequel and the hidden one are gone.
		if want := []int{10, 22, 20, 21, 30, 40}; !slices.Equal(ids(got), want) {
			t.Fatalf("order = %v, want %v", ids(got), want)
		}
		if want := []string{"有 Fandisc", "有外传"}; !slices.Equal(got[0].Phrases, want) {
			t.Errorf("merged phrases = %v, want %v", got[0].Phrases, want)
		}
	})

	t.Run("reader who sees everything", func(t *testing.T) {
		for _, cl := range []string{"", "all"} {
			got := relatedWorks(&rels, cl)
			if got[0].ID != 50 || got[0].ContentLimit != "nsfw" {
				t.Errorf("content_limit %q: first = %+v, want the r18 sequel 50 marked nsfw", cl, got[0])
			}
			if slices.Contains(ids(got), 60) {
				t.Errorf("content_limit %q: hidden work 60 listed", cl)
			}
		}
	})

	t.Run("none is an empty list", func(t *testing.T) {
		if got := relatedWorks(nil, "sfw"); got == nil || len(got) != 0 {
			t.Errorf("relatedWorks(nil) = %#v, want []", got)
		}
	})
}

func TestGetGalgameCarriesRelations(t *testing.T) {
	srv := newCatalogFake(t)
	c := NewWithKey(srv.URL, "nm_test_key")
	ctx := context.Background()

	env, err := c.GetGalgame(ctx, 8, "sfw")
	if err != nil {
		t.Fatalf("GetGalgame: %v", err)
	}
	got := env.Galgame.Relations
	if want := []int{849, 214301}; !slices.Equal(ids(got), want) {
		t.Fatalf("relations = %v, want %v (r18 2680 gated, hidden 2681 dropped)", ids(got), want)
	}
	if want := []string{"有 Fandisc", "重制自"}; !slices.Equal(got[0].Phrases, want) {
		t.Errorf("849 phrases = %v, want %v", got[0].Phrases, want)
	}
	if got[0].Name.JaJp != "Rewrite Harvest festa!" || got[1].Medium != "anime" {
		t.Errorf("rows = %+v", got)
	}

	env, err = c.GetGalgame(ctx, 8, "")
	if err != nil {
		t.Fatalf("GetGalgame: %v", err)
	}
	side := env.Galgame.Relations[1]
	if side.ID != 2680 || side.EffectivePortraitHash != "hash9" || side.EffectivePortraitThumbhash != "th" || side.ReleaseDate != "2012-01-01" {
		t.Errorf("side story = %+v, want 2680 with cover hash9/th and its date", side)
	}
}
