package client

import (
	"context"
	"fmt"
	"log/slog"

	"kun-galgame-patch-api/pkg/catalogv2"
)

// The five 资料库 families moyu can open a page for. There is no engine family
// here even though catalog indexes one: moyu ships no /galgame/engine/:id, and
// a search result that cannot be clicked is worse than one that is missing.
const (
	EntityFamilyCharacter = "character"
	EntityFamilyCompany   = "company"
	EntityFamilyStaff     = "staff"
	EntityFamilyTag       = "tag"
	EntityFamilySeries    = "series"
)

var entityFamilies = []string{
	EntityFamilyCharacter,
	EntityFamilyCompany,
	EntityFamilyStaff,
	EntityFamilyTag,
	EntityFamilySeries,
}

// catalog's search face names a credit name credit_name; moyu's route for it is
// /galgame/staff/:id, and the family travels to the browser under that name.
var entitySearchObject = map[string]string{
	EntityFamilyCharacter: "character",
	EntityFamilyCompany:   "company",
	EntityFamilyStaff:     "credit_name",
	EntityFamilyTag:       "tag",
	EntityFamilySeries:    "series",
}

func EntityFamilies() []string { return entityFamilies }

func IsEntityFamily(family string) bool {
	_, ok := entitySearchObject[family]
	return ok
}

// EntitySearchItem is one 资料库 card. Name travels on all four slots for the
// same reason a work title does — the reader's 标题语言 picks in the browser.
type EntitySearchItem struct {
	ID        int         `json:"id"`
	Family    string      `json:"family"`
	Name      KunLanguage `json:"name"`
	ImageHash string      `json:"image_hash,omitempty"`
	WorkCount int         `json:"work_count"`
}

// Tags are the one family whose rows this site drops after catalog has counted
// them: an SFW reader may not see a sexual tag. Reporting catalog's count beside
// a page it filtered reads as broken — 触手 answers 12 tags and shows 2 — so the
// whole match set is held and counted here instead, the way the user lane holds
// OAuth's. Beyond this window the count is a floor; the tag vocabulary is a few
// thousand rows and no keyword has come close.
const searchTagWindow = 100

// SearchEntities answers one family of the catalog's name index.
func (c *Client) SearchEntities(
	ctx context.Context, family, q string, page, limit int, contentLimit string,
) ([]EntitySearchItem, int64, error) {
	object, ok := entitySearchObject[family]
	if !ok {
		return nil, 0, fmt.Errorf("unknown entity family %q", family)
	}
	window, offset := limit, 0
	if family == EntityFamilyTag {
		window, offset, page = searchTagWindow, (page-1)*limit, 1
	}

	hits, err := c.v2.SearchEntities(ctx, catalogv2.SearchQuery{
		Object: object, Q: q, Page: page, Limit: window, NSFW: true,
	})
	if err != nil {
		return nil, 0, err
	}

	items := make([]EntitySearchItem, 0, len(hits.Items))
	for i := range hits.Items {
		h := hits.Items[i]
		id, ok := h.IntID()
		name := entityHitNames(family, h)
		if !ok || name.canonical() == "" {
			slog.Warn("catalog search hit has no usable id or name; it is dropped",
				"family", family, "id", h.ID, "display_name", h.DisplayName)
			continue
		}
		items = append(items, EntitySearchItem{ID: int(id), Family: family, Name: name})
	}
	items = c.attachEntityFacts(ctx, family, items, contentLimit)

	if family != EntityFamilyTag {
		return items, hits.Count(), nil
	}
	total := int64(len(items))
	if offset >= len(items) {
		return []EntitySearchItem{}, total, nil
	}
	return items[offset:min(offset+limit, len(items))], total, nil
}

// Catalog's tag vocabulary is written in Chinese while every other family's
// display_name is the Japanese original, and a search hit carries no lang tag
// to tell them apart — so the slot is chosen per family here.
func entityHitNames(family string, h catalogv2.SearchHit) KunLanguage {
	lang := "ja"
	if family == EntityFamilyTag {
		lang = "zh-Hans"
	}
	return catalogEntityNames(localizedFrom(h.Localized), h.DisplayName, lang, strOrEmpty(h.Latin))
}

// attachEntityFacts fills the half of a card the search face does not answer: a
// picture and a work count. It is decoration — a card without its face is still
// a usable result — so a failed batch is logged and the rows are kept as they
// are, except for an SFW reader's tags: the batch is also what clears a tag of
// the sexual family, and keeping them on a failure served every sexual tag to
// that reader. Staff is absent on purpose: a credit name has no work count of its own
// and its photo is null for every row catalog holds, so the batch would cost a
// request per search and change nothing.
func (c *Client) attachEntityFacts(
	ctx context.Context, family string, items []EntitySearchItem, contentLimit string,
) []EntitySearchItem {
	if len(items) == 0 || family == EntityFamilyStaff {
		return items
	}
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, int64(item.ID))
	}

	type fact struct {
		imageHash string
		workCount int
		sexual    bool
	}
	facts := make(map[int]fact, len(ids))
	var err error
	switch family {
	case EntityFamilyCharacter:
		var rows []catalogv2.Character
		if rows, err = c.v2.CharactersByIDs(ctx, ids, true); err == nil {
			reveal := revealsSexual(contentLimit)
			for _, r := range rows {
				id, _ := r.IntID()
				facts[int(id)] = fact{imageHash: artFor(imageHash(r.Image), imageSexual(r.Image), reveal)}
			}
		}
	case EntityFamilyCompany:
		var rows []catalogv2.Company
		if rows, err = c.v2.CompaniesByIDs(ctx, ids, true); err == nil {
			for _, r := range rows {
				id, _ := r.IntID()
				facts[int(id)] = fact{imageHash: imageHash(r.Logo), workCount: r.WorkCount}
			}
		}
	case EntityFamilyTag:
		var rows []catalogv2.Tag
		if rows, err = c.v2.TagsByIDs(ctx, ids, true); err == nil {
			for _, r := range rows {
				id, _ := r.IntID()
				facts[int(id)] = fact{workCount: r.WorkCount, sexual: r.IsSexual}
			}
		}
	case EntityFamilySeries:
		var rows []catalogv2.Series
		if rows, err = c.v2.SeriesByIDs(ctx, ids, true); err == nil {
			for _, r := range rows {
				id, _ := r.IntID()
				facts[int(id)] = fact{workCount: r.WorkCount}
			}
		}
	}
	hideSexual := family == EntityFamilyTag && gateFor(contentLimit).contentLimit == "sfw"
	if err != nil {
		slog.Warn("资料库搜索的配图与作品数获取失败", "family", family, "error", err, "sfw_tags_dropped", hideSexual)
		if hideSexual {
			return items[:0]
		}
		return items
	}

	out := items[:0]
	for _, item := range items {
		f, found := facts[item.ID]
		// A tag the registry does not answer for cannot be classified, and the
		// gate closes rather than guessing it safe.
		if hideSexual && !found {
			slog.Warn("catalog tag batch did not answer a search hit; it is kept from an sfw reader", "tag", item.ID)
			continue
		}
		if hideSexual && f.sexual {
			continue
		}
		item.ImageHash, item.WorkCount = f.imageHash, f.workCount
		out = append(out, item)
	}
	return out
}

// ResolveEntities answers the names behind ids the caller already holds. The
// 高级筛选 panel keeps its 会社 and 标签 in the URL so a filtered search can be
// shared, and a link opened cold arrives with ids and nothing to label its
// chips with.
//
// Unlike the search lane this does not drop a sexual tag for an SFW reader:
// what that gate protects is browsing the vocabulary, and an id the request
// already names is not a discovery.
func (c *Client) ResolveEntities(ctx context.Context, family string, ids []int) ([]EntitySearchItem, error) {
	wide := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			wide = append(wide, int64(id))
		}
	}
	if len(wide) == 0 {
		return []EntitySearchItem{}, nil
	}

	items := make([]EntitySearchItem, 0, len(wide))
	switch family {
	case EntityFamilyCompany:
		rows, err := c.v2.CompaniesByIDs(ctx, wide, true)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			id, _ := r.IntID()
			items = append(items, EntitySearchItem{
				ID: int(id), Family: family, WorkCount: r.WorkCount, ImageHash: imageHash(r.Logo),
				Name: catalogEntityNames(localizedFrom(r.Localized), r.DisplayName, "ja", strOrEmpty(r.Latin)),
			})
		}
	case EntityFamilyTag:
		rows, err := c.v2.TagsByIDs(ctx, wide, true)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			id, _ := r.IntID()
			items = append(items, EntitySearchItem{
				ID: int(id), Family: family, WorkCount: r.WorkCount,
				Name: catalogEntityNames(localizedFrom(r.Localized), r.DisplayName, "zh-Hans", ""),
			})
		}
	default:
		return nil, fmt.Errorf("entity family %q has no batch face", family)
	}
	return items, nil
}
