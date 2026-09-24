package client

import (
	"context"

	"kun-galgame-patch-api/pkg/catalogv2"
)

const (
	entityWorksDefaultLimit = 24
	entityWorksMaxLimit     = 100
)

// GalgameEntityWork is one work an entity is attached to: the card the site
// draws for it, plus the facts of the attachment itself. Roles fill for a
// credit name, RosterRole / Spoiler / Voices for a character.
type GalgameEntityWork struct {
	Galgame    GalgameBrief       `json:"galgame"`
	RosterRole string             `json:"roster_role,omitempty"`
	Spoiler    int                `json:"spoiler,omitempty"`
	Voices     []GalgamePersonRef `json:"voices,omitempty"`
	Roles      []GalgameStaffRole `json:"roles,omitempty"`
}

// NextCursor is catalog's own opaque offset cursor, passed back untouched.
type GalgameEntityWorkPage struct {
	Items      []GalgameEntityWork `json:"items"`
	NextCursor string              `json:"next_cursor,omitempty"`
}

func (c *Client) CharacterWorks(ctx context.Context, id int, contentLimit, cursor string, limit int) (*GalgameEntityWorkPage, error) {
	gate := gateFor(contentLimit)
	page, err := c.v2.CharacterAppearances(ctx, int64(id), true, cursor, entityWorksLimit(limit))
	if err != nil {
		return nil, err
	}
	works := make([]catalogv2.Work, 0, len(page.Items))
	for i := range page.Items {
		works = append(works, page.Items[i].Work)
	}
	cards, err := c.entityWorkCards(ctx, works, gate)
	if err != nil {
		return nil, err
	}

	out := &GalgameEntityWorkPage{Items: []GalgameEntityWork{}, NextCursor: page.Next()}
	for i := range page.Items {
		a := &page.Items[i]
		id, ok := a.Work.IntID()
		if !ok {
			continue
		}
		card, ok := cards[id]
		if !ok {
			continue
		}
		voices := make([]GalgamePersonRef, 0, len(a.Voices))
		for _, v := range personRefsFrom(a.Voices) {
			if name := v.names(); name.canonical() != "" {
				voices = append(voices, GalgamePersonRef{ID: int(v.ID), Name: name})
			}
		}
		out.Items = append(out.Items, GalgameEntityWork{
			Galgame: card, RosterRole: a.RosterRole, Spoiler: spoilerInt(a.Spoiler), Voices: voices,
		})
	}
	return out, nil
}

func (c *Client) StaffWorks(ctx context.Context, id int, contentLimit, cursor string, limit int) (*GalgameEntityWorkPage, error) {
	gate := gateFor(contentLimit)
	page, err := c.v2.CreditNameCredits(ctx, int64(id), true, cursor, entityWorksLimit(limit))
	if err != nil {
		return nil, err
	}
	works := make([]catalogv2.Work, 0, len(page.Items))
	for i := range page.Items {
		works = append(works, page.Items[i].Work)
	}
	cards, err := c.entityWorkCards(ctx, works, gate)
	if err != nil {
		return nil, err
	}

	out := &GalgameEntityWorkPage{Items: []GalgameEntityWork{}, NextCursor: page.Next()}
	for i := range page.Items {
		credit := &page.Items[i]
		id, ok := credit.Work.IntID()
		if !ok {
			continue
		}
		card, ok := cards[id]
		if !ok {
			continue
		}
		out.Items = append(out.Items, GalgameEntityWork{Galgame: card, Roles: staffRoles(credit.Roles)})
	}
	return out, nil
}

func entityWorksLimit(limit int) int {
	if limit <= 0 {
		return entityWorksDefaultLimit
	}
	return min(limit, entityWorksMaxLimit)
}

// The sub-faces answer each work through the basic view: no cover, no company,
// no refs. That is the whole card, so the page is redrawn through the works
// list face — which is also the only face that gates on the axis moyu displays.
// The sub-faces' own nsfw= reads content_rating, so nsfw=false there drops work
// 2156 (AIR: r18 upstream, claimed sfw) — the very row this site shows to
// everyone. They are asked wide open and content_limit= narrows here instead.
func (c *Client) entityWorkCards(ctx context.Context, works []catalogv2.Work, gate catalogGate) (map[int64]GalgameBrief, error) {
	out := make(map[int64]GalgameBrief, len(works))
	ids := make([]int64, 0, len(works))
	seen := make(map[int64]bool, len(works))
	for i := range works {
		id, ok := works[i].IntID()
		if !ok || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return out, nil
	}
	page, err := c.v2.ListWorks(ctx, catalogv2.WorksQuery{
		IDs: ids, NSFW: true, Include: listCardInclude,
		ContentLimit: gate.contentLimit, Limit: CatalogWorksIDsMax,
	})
	if err != nil {
		return nil, err
	}
	for i := range page.Items {
		it := workToListItem(page.Items[i])
		if !it.ClaimedBy.renderable() || it.publicGID() == 0 {
			continue
		}
		b := catalogItemToBrief(&it)
		b.Facet = facetOf(page.Items[i], gate.contentLimit != "sfw")
		out[it.ID] = b
	}
	return out, nil
}
