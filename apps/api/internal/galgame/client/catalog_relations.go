package client

import (
	"cmp"
	"log/slog"
	"slices"

	"kun-galgame-patch-api/pkg/catalogv2"
)

// GalgameRelated is one work related to the page's work, every relation to it
// merged into Phrases, strongest first.
type GalgameRelated struct {
	ID                         int         `json:"id"`
	Medium                     string      `json:"medium"`
	Name                       KunLanguage `json:"name"`
	ContentLimit               string      `json:"content_limit"`
	ReleaseDate                string      `json:"release_date"`
	EffectivePortraitHash      string      `json:"effective_portrait_hash"`
	EffectivePortraitThumbhash string      `json:"effective_portrait_thumbhash"`
	Phrases                    []string    `json:"phrases"`
}

var relationRank = map[string]int{
	"sequel_of":           0,
	"fandisc_of":          1,
	"side_story_of":       2,
	"remake_of":           3,
	"alternative_version": 4,
	"collects":            5,
	"same_series":         6,
	"same_setting":        7,
	"alternative_setting": 8,
	"crossover_with":      9,
	"shares_character":    10,
	"adaptation_of":       11,
}

func rankOf(relationType string) int {
	if r, ok := relationRank[relationType]; ok {
		return r
	}
	return len(relationRank)
}

func relatedWorks(rels *[]catalogv2.WorkRelation, contentLimit string) []GalgameRelated {
	out := []GalgameRelated{}
	if rels == nil {
		return out
	}
	gate := gateFor(contentLimit)

	type phrase struct {
		rank int
		text string
	}
	type entry struct {
		item    GalgameRelated
		phrases []phrase
	}
	byID := map[int64]*entry{}
	var order []int64
	for i := range *rels {
		rel := &(*rels)[i]
		w := &rel.Work
		id, ok := w.IntID()
		if !ok {
			slog.Warn("catalog related work id is not a catalog id; it is dropped", "id", w.ID)
			continue
		}
		if !claimedFrom(w.Claim).renderable() {
			continue
		}
		limit, _ := contentAxisOf(w.ContentLimit, w.ContentRating)
		if !gate.allows(limit) {
			continue
		}
		e, seen := byID[id]
		if !seen {
			e = &entry{item: GalgameRelated{
				ID:           int(id),
				Medium:       w.Medium,
				Name:         workNames(localizedFrom(w.Localized), w.DisplayName, w.OLang, strOrEmpty(w.Latin)),
				ContentLimit: limit,
				ReleaseDate:  strOrEmpty(w.ReleaseDate),
			}}
			if w.Cover != nil {
				e.item.EffectivePortraitHash = imageHash(w.Cover)
				e.item.EffectivePortraitThumbhash = strOrEmpty(w.Cover.Thumbhash)
			}
			byID[id] = e
			order = append(order, id)
		}
		if rel.Phrase != "" && !slices.ContainsFunc(e.phrases, func(p phrase) bool { return p.text == rel.Phrase }) {
			e.phrases = append(e.phrases, phrase{rankOf(rel.RelationType), rel.Phrase})
		}
	}

	type ranked struct {
		item GalgameRelated
		rank int
	}
	rows := make([]ranked, 0, len(order))
	for _, id := range order {
		e := byID[id]
		slices.SortStableFunc(e.phrases, func(a, b phrase) int { return cmp.Compare(a.rank, b.rank) })
		rank := len(relationRank)
		e.item.Phrases = make([]string, 0, len(e.phrases))
		for _, p := range e.phrases {
			rank = min(rank, p.rank)
			e.item.Phrases = append(e.item.Phrases, p.text)
		}
		rows = append(rows, ranked{e.item, rank})
	}
	slices.SortFunc(rows, func(a, b ranked) int {
		if c := cmp.Compare(a.rank, b.rank); c != 0 {
			return c
		}
		ad, bd := a.item.ReleaseDate, b.item.ReleaseDate
		switch {
		case ad != "" && bd == "":
			return -1
		case ad == "" && bd != "":
			return 1
		case ad != bd:
			return cmp.Compare(ad, bd)
		}
		return cmp.Compare(a.item.ID, b.item.ID)
	})
	for _, r := range rows {
		out = append(out, r.item)
	}
	return out
}
