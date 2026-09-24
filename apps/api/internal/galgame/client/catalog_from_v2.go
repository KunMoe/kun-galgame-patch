package client

import (
	"log/slog"

	"kun-galgame-patch-api/pkg/catalogv2"
)

func claimedFrom(c *catalogv2.Claim) *catalogClaimedBy {
	if c == nil {
		return nil
	}
	id, _ := catalogv2.ParseID(c.SiteWorkID)
	return &catalogClaimedBy{
		Site:   c.Site,
		WorkID: id,
		State:  c.State,
	}
}

func localizedFrom(m map[string]catalogv2.LocalizedText) map[string]catalogLocalizedName {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]catalogLocalizedName, len(m))
	for k, v := range m {
		out[k] = catalogLocalizedName{Value: v.Value, Machine: v.IsMachine}
	}
	return out
}

func personRefsFrom(rows []catalogv2.CreditName) []catalogPersonRef {
	out := make([]catalogPersonRef, 0, len(rows))
	for i := range rows {
		n := &rows[i]
		id, _ := n.IntID()
		out = append(out, catalogPersonRef{
			ID: id, DisplayName: n.DisplayName, Lang: strOrEmpty(n.Lang),
			Latin: strOrEmpty(n.Latin), Localized: localizedFrom(n.Localized),
		})
	}
	return out
}

func aliasRowsFrom(rows []catalogv2.EntityName) []catalogAlias {
	out := make([]catalogAlias, 0, len(rows))
	for _, r := range rows {
		out = append(out, catalogAlias{
			Value: r.Value, Lang: r.Lang, Kind: r.AliasKind, Machine: r.IsMachine,
		})
	}
	return out
}

func introRowsFrom(rows []catalogv2.Intro) []catalogIntroRow {
	out := make([]catalogIntroRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, catalogIntroRow{
			Lang: r.Lang, Intro: r.Value, Source: r.Source, Machine: r.IsMachine,
		})
	}
	return out
}

func linkRowsFrom(rows []catalogv2.Link) []catalogEntityLink {
	out := make([]catalogEntityLink, 0, len(rows))
	for _, r := range rows {
		out = append(out, catalogEntityLink{Source: r.Source, URL: r.URL})
	}
	return out
}

func refRowsFrom(rows []catalogv2.Ref) []catalogRef {
	return refsFrom(&rows)
}

func refsFrom(refs *[]catalogv2.Ref) []catalogRef {
	if refs == nil {
		return nil
	}
	out := make([]catalogRef, 0, len(*refs))
	for _, r := range *refs {
		out = append(out, catalogRef{Source: r.Source, ExternalID: r.ExternalID})
	}
	return out
}

func intOrZero(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// null is "nobody assessed this image", never safe. One helper used to read
// only the sexual words for both axes and answer 0 for anything else, so a null
// grade came out as safe and every violent or brutal image as tame.
func sexualGrade(s *string) *int { return grade(s, "safe", "suggestive", "explicit") }

func violenceGrade(s *string) *int { return grade(s, "tame", "violent", "brutal") }

func grade(s *string, scale ...string) *int {
	if s == nil {
		return nil
	}
	for level, word := range scale {
		if *s == word {
			return &level
		}
	}
	return nil
}

const gradeSafe = 0

// revealsSexual reports whether the reader opted into adult content. The
// handlers default an absent content_limit to sfw, so only these two open it.
func revealsSexual(contentLimit string) bool {
	return contentLimit == "nsfw" || contentLimit == "all"
}

// artFor answers the hash a reader may be shown. A cover is not gated here —
// the work's own content_limit already is catalog's verdict on its covers.
func artFor(hash string, sexual *int, reveal bool) string {
	if reveal || (sexual != nil && *sexual == gradeSafe) {
		return hash
	}
	return ""
}

func spoilerInt(s string) int {
	switch s {
	case "minor":
		return 1
	case "major":
		return 2
	default:
		return 0
	}
}

func imageHash(img *catalogv2.Image) string {
	if img == nil {
		return ""
	}
	if img.Hash != "" {
		return img.Hash
	}
	return hashFromURL(img.URL)
}

func workToListItem(w catalogv2.Work) catalogWorkListItem {
	id, ok := w.IntID()
	if !ok {
		slog.Warn("catalog work id is not a catalog id; the row will not render",
			"id", w.ID, "display_name", w.DisplayName)
	}
	item := catalogWorkListItem{
		ID:               id,
		Medium:           w.Medium,
		DisplayName:      w.DisplayName,
		Latin:            strOrEmpty(w.Latin),
		ContentRating:    w.ContentRating,
		ContentLimit:     w.ContentLimit,
		OLang:            w.OLang,
		ReleaseDate:      w.ReleaseDate,
		ReleasePrecision: strOrEmpty(w.ReleasePrecision),
		ClaimedBy:        claimedFrom(w.Claim),
		Updated:          w.UpdatedAt,
		Localized:        localizedFrom(w.Localized),
		Refs:             refsFrom(w.Refs),
	}
	if w.Cover != nil {
		item.Cover = w.Cover.URL
	}
	slots := catalogCoverSlots{}
	if w.Banner != nil {
		b := imageToSlot(w.Banner)
		slots.Banner = &b
	}
	if w.Cover != nil {
		p := imageToSlot(w.Cover)
		slots.Portrait = &p
	}
	if w.Covers != nil {
		for i := range *w.Covers {
			c := &(*w.Covers)[i]
			slot := coverToSlot(c)
			if c.PortraitPinned && slots.Portrait == nil {
				slots.Portrait = &slot
			}
			if !c.PortraitPinned && slots.Banner == nil {
				slots.Banner = &slot
			}
		}
	}
	if slots.Banner != nil || slots.Portrait != nil {
		item.Covers = &slots
	}
	item.Labels = labelsFrom(w.Companies)
	return item
}

func labelsFrom(companies *[]catalogv2.WorkCompany) []catalogWorkLabel {
	if companies == nil {
		return nil
	}
	out := make([]catalogWorkLabel, 0, len(*companies))
	for i := range *companies {
		co := &(*companies)[i]
		id, _ := co.IntID()
		kind := co.CompanyKind
		if co.AttributionRole != "" {
			kind = co.AttributionRole
		}
		out = append(out, catalogWorkLabel{
			ID: id, DisplayName: co.DisplayName, Localized: localizedFrom(co.Localized),
			LabelKind: co.CompanyKind, Kind: kind, Role: co.AttributionRole,
			LogoHash: imageHash(co.Logo),
		})
	}
	return out
}

func imageToSlot(img *catalogv2.Image) catalogCoverSlot {
	return catalogCoverSlot{
		URL:       img.URL,
		Width:     intOrZero(img.Width),
		Height:    intOrZero(img.Height),
		Thumbhash: strOrEmpty(img.Thumbhash),
		Sexual:    sexualGrade(img.Sexual),
		Violence:  violenceGrade(img.Violence),
		Source:    img.Source,
	}
}

func coverToSlot(c *catalogv2.Cover) catalogCoverSlot {
	return catalogCoverSlot{
		URL:       c.URL,
		Width:     intOrZero(c.Width),
		Height:    intOrZero(c.Height),
		Thumbhash: strOrEmpty(c.Thumbhash),
		Sexual:    sexualGrade(c.Sexual),
		Violence:  violenceGrade(c.Violence),
		Source:    c.Source,
	}
}

func workTags(w catalogv2.Work) []catalogWorkTag {
	if w.Tags == nil {
		return nil
	}
	var out []catalogWorkTag
	for _, t := range *w.Tags {
		id, _ := catalogv2.ParseID(strOrEmpty(t.ID))
		tier, kind := "", ""
		if t.Tier != nil {
			tier = *t.Tier
		}
		if t.TagKind != nil {
			kind = *t.TagKind
		}
		out = append(out, catalogWorkTag{
			Name: t.DisplayName, Source: t.Source, CanonicalID: id, Count: intOrZero(t.WorkCount),
			Tier: tier, Kind: kind, Spoiler: spoilerInt(t.Spoiler), Sexual: t.IsSexual,
		})
	}
	return out
}

func workCredits(w catalogv2.Work) []catalogCreditGroup {
	if w.Credits == nil {
		return nil
	}
	var out []catalogCreditGroup
	for _, g := range *w.Credits {
		group := catalogCreditGroup{RoleKey: g.RoleKey, RoleName: g.RoleName}
		for _, e := range g.Credits {
			id, _ := catalogv2.ParseID(e.ID)
			cid, _ := catalogv2.ParseID(strOrEmpty(e.CharacterID))
			group.Credits = append(group.Credits, catalogCreditItem{
				catalogPersonRef: catalogPersonRef{
					ID: id, DisplayName: e.DisplayName, Lang: strOrEmpty(e.Lang),
					Latin: strOrEmpty(e.Latin), Localized: localizedFrom(e.Localized),
				},
				CharacterID: cid,
				Character:   strOrEmpty(e.CharacterName),
			})
		}
		out = append(out, group)
	}
	return out
}

func workToDetail(w catalogv2.Work) catalogWork {
	item := workToListItem(w)
	out := catalogWork{
		ID:               item.ID,
		Medium:           item.Medium,
		DisplayName:      item.DisplayName,
		Latin:            item.Latin,
		OLang:            item.OLang,
		ContentRating:    item.ContentRating,
		ContentLimit:     item.ContentLimit,
		ReleaseDate:      item.ReleaseDate,
		ReleasePrecision: item.ReleasePrecision,
		Created:          w.CreatedAt,
		Updated:          w.UpdatedAt,
		Refs:             item.Refs,
		ClaimedBy:        item.ClaimedBy,
		Localized:        item.Localized,
		CoverSlots:       item.Covers,
		Labels:           item.Labels,
	}
	if w.Intros != nil {
		for _, row := range *w.Intros {
			out.Intros = append(out.Intros, catalogWorkIntro{
				Lang: row.Lang, Intro: row.Value, Source: row.Source, Machine: row.IsMachine,
			})
		}
	}
	if w.Covers != nil {
		for _, c := range *w.Covers {
			out.Covers = append(out.Covers, catalogDetailCover{
				URL:            c.URL,
				Kind:           c.Kind,
				PortraitPinned: c.PortraitPinned,
				Sexual:         sexualGrade(c.Sexual),
				Violence:       violenceGrade(c.Violence),
				Source:         c.Source,
				Width:          intOrZero(c.Width),
				Height:         intOrZero(c.Height),
				Thumbhash:      strOrEmpty(c.Thumbhash),
			})
		}
	}
	if w.Screenshots != nil {
		for _, s := range *w.Screenshots {
			out.Screenshots = append(out.Screenshots, catalogScreenshot{
				URL: s.URL, Caption: s.Caption, Sexual: sexualGrade(s.Sexual),
				Violence: violenceGrade(s.Violence), Source: s.Source,
				Width: intOrZero(s.Width), Height: intOrZero(s.Height),
				Thumbhash: strOrEmpty(s.Thumbhash),
			})
		}
	}
	out.Tags = workTags(w)
	if w.Characters != nil {
		for i := range *w.Characters {
			ch := &(*w.Characters)[i]
			id, _ := ch.IntID()
			out.Characters = append(out.Characters, catalogWorkCharacter{
				ID: id, DisplayName: ch.DisplayName, Localized: localizedFrom(ch.Localized),
				Lang: strOrEmpty(ch.Lang), Latin: strOrEmpty(ch.Latin),
				Kind: ch.RosterRole, Spoiler: spoilerInt(ch.Spoiler),
				Image: imageHash(ch.Image), ImageSexual: imageSexual(ch.Image),
				Figure: imageHash(ch.Figure), FigureSexual: imageSexual(ch.Figure),
				Voices: personRefsFrom(ch.Voices),
			})
		}
	}
	out.Credits = workCredits(w)
	if w.Series != nil {
		for _, sr := range *w.Series {
			id, ok := catalogv2.ParseID(sr.ID)
			if !ok {
				slog.Warn("catalog work series id is not a catalog id; the series is dropped",
					"work", w.ID, "series", sr.ID, "name", sr.DisplayName)
				continue
			}
			out.Series = append(out.Series, catalogWorkSeries{
				ID: id, Name: sr.DisplayName, MemberCount: sr.MemberCount,
			})
		}
	}
	if w.Ratings != nil {
		for i := range *w.Ratings {
			r := &(*w.Ratings)[i]
			row := catalogRating{
				Source: r.Source, Score: r.Score, VoteCount: r.VoteCount, Rank: r.Rank,
			}
			if r.Distribution != nil {
				for _, b := range *r.Distribution {
					row.Distribution = append(row.Distribution,
						catalogRatingBucket{Score: b.Score, Count: b.Count})
				}
			}
			out.Ratings = append(out.Ratings, row)
		}
	}
	return out
}
