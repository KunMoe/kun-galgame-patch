package client

import (
	"sort"
	"strconv"
	"strings"
)

func hashFromURL(u string) string {
	if u == "" {
		return ""
	}
	base := u
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		base = base[i+1:]
	}
	if i := strings.LastIndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}
	return base
}

func productLangFromCatalog(lang string) string {
	l := strings.ToLower(strings.TrimSpace(lang))
	switch {
	case l == "":
		return ""
	case l == "ja" || strings.HasPrefix(l, "ja-"):
		return "ja-jp"
	case l == "zh-hant" || strings.HasPrefix(l, "zh-hant-") || l == "zh-tw" || l == "zh-hk":
		return "zh-tw"
	case l == "zh" || l == "zh-hans" || strings.HasPrefix(l, "zh"):
		return "zh-cn"
	case l == "en" || strings.HasPrefix(l, "en-"):
		return "en-us"
	}
	return lang
}

func catalogLangFromProduct(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "ja-jp":
		return "ja"
	case "zh-cn":
		return "zh-Hans"
	case "zh-tw":
		return "zh-Hant"
	case "en-us":
		return "en"
	}
	return lang
}

func contentAxisOf(claim *catalogClaimedBy, rating string) (contentLimit, ageLimit string) {
	ageLimit = "all"
	if rating == "r18" {
		ageLimit = "r18"
	}
	if claim != nil && isGIDClaimSite(claim.Site) {
		switch claim.ContentLimit {
		case "sfw", "nsfw":
			return claim.ContentLimit, ageLimit
		}
	}
	if rating == "r18" {
		return "nsfw", ageLimit
	}
	return "sfw", ageLimit
}

func normalizeCatalogDate(date *string) (*string, string) {
	if date == nil {
		return nil, ""
	}
	d := strings.TrimSpace(*date)
	switch len(d) {
	case 10:
		return &d, "day"
	case 7:
		full := d + "-01"
		return &full, "month"
	case 4:
		full := d + "-01-01"
		return &full, "year"
	}
	return nil, ""
}

// Several catalog tags fold onto one moyu column — zh-Hant and zh-TW both land
// on zh-tw — so the fold has to elect a winner, and map iteration is random in
// Go. Source before machine, then the lowest tag, so the same payload always
// renders the same title.
func localizedByProductKey(localized map[string]catalogLocalizedName) map[string]string {
	tags := make([]string, 0, len(localized))
	for tag := range localized {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	out := make(map[string]string, 4)
	for _, machine := range []bool{false, true} {
		for _, tag := range tags {
			row := localized[tag]
			if row.Machine != machine || row.Value == "" {
				continue
			}
			switch k := productLangFromCatalog(tag); k {
			case "ja-jp", "zh-cn", "zh-tw", "en-us":
				if _, taken := out[k]; !taken {
					out[k] = row.Value
				}
			}
		}
	}
	return out
}

func namesOf(localized map[string]catalogLocalizedName) (ja, zhCN, zhTW, en string) {
	n := localizedByProductKey(localized)
	return n["ja-jp"], n["zh-cn"], n["zh-tw"], n["en-us"]
}

func vndbIDOf(refs []catalogRef) string {
	for _, r := range refs {
		if r.Source == "vndb" && isVndbWorkID(r.ExternalID) {
			return r.ExternalID
		}
	}
	return ""
}

func isVndbWorkID(s string) bool {
	if len(s) < 2 || s[0] != 'v' {
		return false
	}
	for i := 1; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func coverOf(it *catalogWorkListItem) (hash string, width, height int, thumbhash string) {
	if it.Covers != nil {
		if c := it.Covers.Banner; c != nil {
			return hashFromURL(c.URL), c.Width, c.Height, c.Thumbhash
		}
		if c := it.Covers.Portrait; c != nil {
			return hashFromURL(c.URL), c.Width, c.Height, c.Thumbhash
		}
	}
	return hashFromURL(it.Cover), 0, 0, ""
}

// covers.portrait is filled from any cover when the work has no portrait-shaped
// one, so it is NOT guaranteed to be taller than wide. Crop it into the box you
// want; do not lay out from its aspect ratio.
func portraitOf(slots *catalogCoverSlots) (hash string, width, height int, thumbhash string) {
	if slots == nil || slots.Portrait == nil {
		return "", 0, 0, ""
	}
	c := slots.Portrait
	return hashFromURL(c.URL), c.Width, c.Height, c.Thumbhash
}

func claimStateOf(c *catalogClaimedBy) string {
	if c == nil || !isGIDClaimSite(c.Site) {
		return ""
	}
	return c.State
}

func catalogItemToBrief(it *catalogWorkListItem) GalgameBrief {
	ja, zhCN, zhTW, en := namesOf(it.Localized)
	cl, age := contentAxisOf(it.ClaimedBy, it.ContentRating)
	date, precision := normalizeCatalogDate(it.ReleaseDate)
	hash, w, h, th := coverOf(it)

	b := GalgameBrief{
		ID:                       it.ClaimedBy.gid(),
		CatalogWorkID:            it.ID,
		VndbID:                   vndbIDOf(it.Refs),
		ClaimState:               claimStateOf(it.ClaimedBy),
		NameJaJp:                 ja,
		NameZhCn:                 zhCN,
		NameZhTw:                 zhTW,
		NameEnUs:                 en,
		ContentLimit:             cl,
		AgeLimit:                 age,
		OriginalLanguage:         productLangFromCatalog(it.OLang),
		ReleaseDate:              date,
		ReleasePrecision:         precision,
		EffectiveBannerHash:      hash,
		EffectiveBannerWidth:     w,
		EffectiveBannerHeight:    h,
		EffectiveBannerThumbhash: th,
	}
	b.EffectivePortraitHash, b.EffectivePortraitWidth,
		b.EffectivePortraitHeight, b.EffectivePortraitThumbhash = portraitOf(it.Covers)
	if ja == "" && zhCN == "" && zhTW == "" && en == "" {
		b.NameJaJp = it.DisplayName
	}
	return b
}

func catalogItemToHit(it *catalogWorkListItem) GalgameHit {
	b := catalogItemToBrief(it)
	return GalgameHit{
		ID:                       b.ID,
		CatalogWorkID:            b.CatalogWorkID,
		VndbID:                   b.VndbID,
		ClaimState:               b.ClaimState,
		NameEnUs:                 b.NameEnUs,
		NameZhCn:                 b.NameZhCn,
		NameJaJp:                 b.NameJaJp,
		NameZhTw:                 b.NameZhTw,
		ContentLimit:             b.ContentLimit,
		AgeLimit:                 b.AgeLimit,
		OriginalLanguage:         b.OriginalLanguage,
		ReleaseDate:              b.ReleaseDate,
		EffectiveBannerHash:      b.EffectiveBannerHash,
		EffectiveBannerWidth:     b.EffectiveBannerWidth,
		EffectiveBannerHeight:    b.EffectiveBannerHeight,
		EffectiveBannerThumbhash: b.EffectiveBannerThumbhash,
	}
}

func introRows(w *catalogWork) []catalogWorkIntro {
	if len(w.Intros) > 0 {
		return w.Intros
	}
	return w.Intro
}

func introByProductKey(rows []catalogWorkIntro) map[string]string {
	out := make(map[string]string, 4)
	for _, r := range rows {
		k := productLangFromCatalog(r.Lang)
		switch k {
		case "ja-jp", "zh-cn", "zh-tw", "en-us":
			if _, taken := out[k]; !taken {
				out[k] = r.Intro
			}
		}
	}
	return out
}

func catalogCoversToInputs(covers []catalogDetailCover) []CoverInput {
	if len(covers) == 0 {
		return nil
	}
	out := make([]CoverInput, 0, len(covers))
	for i := range covers {
		c := &covers[i]
		out = append(out, CoverInput{
			ImageHash: hashFromURL(c.URL),
			SortOrder: i,
			Sexual:    c.Sexual,
			Violence:  c.Violence,
			Source:    c.Source,
			Kind:      c.Kind,
			Width:     c.Width,
			Height:    c.Height,
			Thumbhash: c.Thumbhash,
		})
	}
	return out
}

func catalogScreenshotsToInputs(shots []catalogScreenshot) []ScreenshotInput {
	if len(shots) == 0 {
		return nil
	}
	out := make([]ScreenshotInput, 0, len(shots))
	for i := range shots {
		s := &shots[i]
		out = append(out, ScreenshotInput{
			ImageHash: hashFromURL(s.URL),
			SortOrder: i,
			Caption:   s.Caption,
			Sexual:    s.Sexual,
			Violence:  s.Violence,
			Source:    s.Source,
			Width:     s.Width,
			Height:    s.Height,
			Thumbhash: s.Thumbhash,
		})
	}
	return out
}

func heroCover(covers []catalogDetailCover) *catalogDetailCover {
	if len(covers) == 0 {
		return nil
	}
	for i := range covers {
		if isLandscape(covers[i].Width, covers[i].Height) {
			return &covers[i]
		}
	}
	for i := range covers {
		if covers[i].PortraitPinned {
			return &covers[i]
		}
	}
	return &covers[0]
}

func isLandscape(w, h int) bool {
	return w > 0 && h > 0 && int64(h)*20 <= int64(w)*21
}

func portraitCover(covers []catalogDetailCover) *catalogDetailCover {
	for i := range covers {
		if covers[i].PortraitPinned {
			return &covers[i]
		}
	}
	for i := range covers {
		c := &covers[i]
		if c.Width > 0 && c.Height > 0 && !isLandscape(c.Width, c.Height) {
			return c
		}
	}
	return nil
}

func catalogWorkToFull(w *catalogWork) GalgameFull {
	cl, age := contentAxisOf(w.ClaimedBy, w.ContentRating)
	date, _ := normalizeCatalogDate(w.ReleaseDate)
	names := localizedByProductKey(w.Localized)
	intros := introByProductKey(introRows(w))

	f := GalgameFull{
		ID:               w.ClaimedBy.gid(),
		CatalogWorkID:    w.ID,
		VndbID:           vndbIDOf(w.Refs),
		ClaimState:       claimStateOf(w.ClaimedBy),
		NameJaJp:         names["ja-jp"],
		NameZhCn:         names["zh-cn"],
		NameZhTw:         names["zh-tw"],
		NameEnUs:         names["en-us"],
		IntroJaJp:        intros["ja-jp"],
		IntroZhCn:        intros["zh-cn"],
		IntroZhTw:        intros["zh-tw"],
		IntroEnUs:        intros["en-us"],
		ContentLimit:     cl,
		AgeLimit:         age,
		OriginalLanguage: productLangFromCatalog(w.OLang),
		ReleaseDate:      date,
		Created:          w.Created,
		Updated:          w.Updated,
		Covers:           catalogCoversToInputs(w.Covers),
		Screenshots:      catalogScreenshotsToInputs(w.Screenshots),
		Characters:       catalogCharacters(w.Characters),
		Staff:            catalogStaff(w.Credits, w.Characters),
		Ratings:          catalogRatings(w.Ratings),
		Series:           catalogSeries(w.Series),
	}
	if f.NameJaJp == "" && f.NameZhCn == "" && f.NameZhTw == "" && f.NameEnUs == "" {
		f.NameJaJp = w.DisplayName
	}
	if c := heroCover(w.Covers); c != nil {
		f.EffectiveBannerHash = hashFromURL(c.URL)
		f.EffectiveBannerWidth = c.Width
		f.EffectiveBannerHeight = c.Height
		f.EffectiveBannerThumbhash = c.Thumbhash
	}
	f.EffectivePortraitHash, f.EffectivePortraitWidth,
		f.EffectivePortraitHeight, f.EffectivePortraitThumbhash = portraitOf(w.CoverSlots)
	if f.EffectivePortraitHash == "" {
		if c := portraitCover(w.Covers); c != nil {
			f.EffectivePortraitHash = hashFromURL(c.URL)
			f.EffectivePortraitWidth = c.Width
			f.EffectivePortraitHeight = c.Height
			f.EffectivePortraitThumbhash = c.Thumbhash
		}
	}
	for i := range w.Tags {
		f.Tag = append(f.Tag, catalogTagToFullTag(f.ID, &w.Tags[i]))
	}
	for i := range w.Labels {
		f.Official = append(f.Official, catalogLabelToFullOfficial(f.ID, &w.Labels[i]))
	}
	return f
}

func catalogTagToFullTag(gid int, t *catalogWorkTag) GalgameFullTag {
	category := tagCategoryFor(t.Sexual)
	return GalgameFullTag{
		GalgameID:    gid,
		TagID:        int(t.CanonicalID),
		SpoilerLevel: t.Spoiler,
		Tag: Tag{
			ID:       int(t.CanonicalID),
			Name:     t.Name,
			Category: category,
		},
	}
}

func tagCategoryFor(sexual bool) string {
	if sexual {
		return "sexual"
	}
	return "content"
}

func catalogLabelToFullOfficial(gid int, l *catalogWorkLabel) GalgameFullOfficial {
	return GalgameFullOfficial{
		GalgameID:  gid,
		OfficialID: int(l.ID),
		Official: Official{
			ID:       int(l.ID),
			Name:     l.DisplayName,
			Category: l.LabelKind,
			Lang:     productLangFromCatalog(l.Lang),
			LogoHash: l.LogoHash,
		},
	}
}

func catalogSeries(rows []catalogWorkSeries) []GalgameSeries {
	if len(rows) == 0 {
		return nil
	}
	out := make([]GalgameSeries, 0, len(rows))
	for _, s := range rows {
		if s.ID == 0 || strings.TrimSpace(s.Name) == "" {
			continue
		}
		out = append(out, GalgameSeries{ID: int(s.ID), Name: s.Name})
	}
	return out
}

func joinCatalogLangs(csv string) string {
	if strings.TrimSpace(csv) == "" {
		return ""
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, catalogLangFromProduct(p))
		}
	}
	return strings.Join(out, ",")
}

func yearLowerBound(y int) string { return strconv.Itoa(y) + "-01-01" }
func yearUpperBound(y int) string { return strconv.Itoa(y) + "-12-31" }
