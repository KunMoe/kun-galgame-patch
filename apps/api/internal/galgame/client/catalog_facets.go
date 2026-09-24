package client

import (
	"context"
	"log/slog"
	"sort"

	"kun-galgame-patch-api/pkg/catalogv2"
)

// GalgameCardTag is one tag a list card prints. The id deep-links the tag page.
type GalgameCardTag struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// GalgameFacet is what a card shows once it has room for more than a title:
// the two credits a reader picks a game by, and a short shelf of tags.
type GalgameFacet struct {
	Scenario     []KunLanguage    `json:"scenario,omitempty"`
	Illustration []KunLanguage    `json:"illustration,omitempty"`
	Tags         []GalgameCardTag `json:"tags,omitempty"`
}

func (f GalgameFacet) empty() bool {
	return len(f.Scenario) == 0 && len(f.Illustration) == 0 && len(f.Tags) == 0
}

const (
	facetStaffMax = 2
	facetTagMax   = 6
)

// Both blocks ride the works list face; asking for tags there answers the
// spoiler=none ceiling, which is the one the shelf wants.
var facetInclude = []string{"tags", "credits"}

type facetTag struct {
	id     int
	name   string
	sexual bool
}

type facetEntry struct {
	scenario     []KunLanguage
	illustration []KunLanguage
	tags         []facetTag
}

// facetOf distills the shelf a list card prints from one works-list item. It
// answers nil unless the read asked for tags and credits: an item hydrated
// through cardInclude alone carries neither block, and an empty shelf must not
// render as an empty row.
func facetOf(w catalogv2.Work, sexualOK bool) *GalgameFacet {
	f := facetFrom(catalogWork{Tags: workTags(w), Credits: workCredits(w)}).facet(sexualOK)
	if f.empty() {
		return nil
	}
	return &f
}

// facetsByWorkID reads the shelf for works already in hand. Only the company
// page needs it: that roster walks catalog's whole rollup to sort and slice it
// here, and asking the walk for credits would carry them for every work the
// company ever shipped to print 24.
func (c *Client) facetsByWorkID(ctx context.Context, ids []int64, gate catalogGate) map[int64]*GalgameFacet {
	out := make(map[int64]*GalgameFacet, len(ids))
	if c == nil || len(ids) == 0 {
		return out
	}
	page, err := c.v2.ListWorks(ctx, catalogv2.WorksQuery{
		IDs: ids, NSFW: true, Include: facetInclude, Limit: CatalogWorksIDsMax,
	})
	if err != nil {
		return out
	}
	sexualOK := gate.contentLimit != "sfw"
	for i := range page.Items {
		id, ok := page.Items[i].IntID()
		if !ok {
			slog.Warn("catalog work id is not a catalog id; its shelf is dropped", "id", page.Items[i].ID)
			continue
		}
		out[id] = facetOf(page.Items[i], sexualOK)
	}
	return out
}

func (e facetEntry) facet(sexualOK bool) GalgameFacet {
	f := GalgameFacet{Scenario: e.scenario, Illustration: e.illustration}
	for _, t := range e.tags {
		if t.sexual && !sexualOK {
			continue
		}
		f.Tags = append(f.Tags, GalgameCardTag{ID: t.id, Name: t.name})
		if len(f.Tags) == facetTagMax {
			break
		}
	}
	return f
}

func facetFrom(w catalogWork) facetEntry {
	e := facetEntry{tags: facetTags(w.Tags)}
	for _, g := range catalogStaff(w.Credits, nil) {
		switch g.RoleKey {
		case "scenario":
			e.scenario = facetNames(g.People)
		case "illustration":
			e.illustration = facetNames(g.People)
		}
	}
	return e
}

func facetNames(people []GalgameStaffMember) []KunLanguage {
	names := make([]KunLanguage, 0, facetStaffMax)
	for _, p := range people {
		if len(names) == facetStaffMax {
			break
		}
		names = append(names, p.Name)
	}
	return names
}

// The shelf comes out of catalog's own tiering: core is the curated inventory,
// content drops the meta rows ("Galgame", "PC") that say nothing about the
// work, and an unmapped row is a raw folksonomy string — on work 3 those are
// staff names, not tags. Rarest first, because the head of that distribution is
// the same everywhere: 男性主人公 sits on 7,465 works and would lead every card
// in the list, while the tag on 174 is the reason to open this one.
func facetTags(tags []catalogWorkTag) []facetTag {
	picked := make([]catalogWorkTag, 0, len(tags))
	for _, t := range tags {
		if t.CanonicalID <= 0 || t.Tier != "core" || t.Kind != "content" || t.Spoiler > 0 {
			continue
		}
		picked = append(picked, t)
	}
	sort.SliceStable(picked, func(i, j int) bool { return picked[i].Count < picked[j].Count })

	out := make([]facetTag, 0, len(picked))
	for _, t := range picked {
		out = append(out, facetTag{id: int(t.CanonicalID), name: t.Name, sexual: t.Sexual})
	}
	return out
}
