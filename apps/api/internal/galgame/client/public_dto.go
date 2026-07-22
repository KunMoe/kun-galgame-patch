package client

// NextMoe /v1 public-contract wire DTOs + mappers (open-API phase 2 wave 07, W3).
//
// Background: the galgame read set migrated off the internal bridge face
// (raw-model shapes = legacy) onto the frozen /v1 public contract (curated
// shapes). This file holds the /v1 wire structs this client parses and the
// mappers that project them back onto the moyu-internal DTOs (GalgameBrief /
// GalgameHit / GalgameFull) the enricher already consumes — so moyu's OWN API
// output stays byte-stable. The mapping is value-level (moyu's DTO struct tags
// fix the output KEYS); a handful of raw-model-only fields the /v1 curation
// deliberately drops (per-cover sexual/violence/source provenance, taxonomy
// alias-row metadata) have no /v1 source and fall to their zero value — the FE
// does not consume them (W3 census). See refs/plans/09.../07-route-b-endgame.md.

import (
	"strings"
)

// ─── /v1 wire structs (only the fields this client reads) ─────────────

// v1Names is the /v1 localized-names object: every key present, empty → null.
type v1Names struct {
	JaJP *string `json:"ja-jp"`
	ZhCN *string `json:"zh-cn"`
	ZhTW *string `json:"zh-tw"`
	EnUS *string `json:"en-us"`
}

// v1Intro is the /v1 include=intro block (same empty→null discipline).
type v1Intro struct {
	ZhCN *string `json:"zh-cn"`
	EnUS *string `json:"en-us"`
	JaJP *string `json:"ja-jp"`
	ZhTW *string `json:"zh-tw"`
}

// v1Image is one rendered /v1 image: a COMPLETE CDN URL (never a bare hash) +
// intrinsic dims + ThumbHash. Kind is only meaningful on covers[]. Sexual /
// Violence are the per-image content-rating levels (W1c) — present on
// covers[]/screenshots[] entries only when the key is nsfw-capable (moyu's
// internal key carries galgame:nsfw since W1a P5). Caption is the per-screenshot
// gallery text (W1c, screenshots-only). sort_order is NOT on the /v1 wire — the
// arrays are server-ordered, so the moyu adapter synthesizes it from the index.
type v1Image struct {
	URL       string `json:"url"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Thumbhash string `json:"thumbhash"`
	Kind      string `json:"kind,omitempty"`
	Sexual    *int   `json:"sexual"`
	Violence  *int   `json:"violence"`
	Caption   string `json:"caption"`
}

// v1Images is the /v1 images block. covers/screenshots present only under their
// include tokens; all entries NSFW-filtered on the sfw face.
type v1Images struct {
	Banner      *v1Image  `json:"banner"`
	Portrait    *v1Image  `json:"portrait"`
	Covers      []v1Image `json:"covers"`
	Screenshots []v1Image `json:"screenshots"`
}

// v1TagRef is one include=tag_refs entry.
type v1TagRef struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Category     string `json:"category"`
	SpoilerLevel int    `json:"spoiler_level"`
}

// v1OfficialRef is one include=official_refs entry.
type v1OfficialRef struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Lang     string `json:"lang"`
}

// v1EngineRef is one include=engine_refs entry.
type v1EngineRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// v1Taxonomy is the /v1 include=taxonomy block (+ the W1a rich ref sub-keys).
type v1Taxonomy struct {
	SeriesID     *int             `json:"series_id"`
	TagRefs      *[]v1TagRef      `json:"tag_refs"`
	OfficialRefs *[]v1OfficialRef `json:"official_refs"`
	EngineRefs   *[]v1EngineRef   `json:"engine_refs"`
}

// v1Link is one include=links entry (curated: id/name/link/source only).
type v1Link struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Link   string `json:"link"`
	Source string `json:"source"`
}

// v1Meta is the /v1 include=meta block — its scalars mirror the internal bridge
// face's values (migration-parity contract). resource_update_time / created are
// null on the zero timestamp (the internal Timestamp discipline).
type v1Meta struct {
	OriginalLanguage   string  `json:"original_language"`
	VNDBID             string  `json:"vndb_id"`
	Status             int     `json:"status"`
	ContentLimit       string  `json:"content_limit"`
	ReleasePrecision   string  `json:"release_precision"`
	SeriesID           *int    `json:"series_id"`
	CatalogWorkID      *int64  `json:"catalog_work_id"`
	UserID             int     `json:"user_id"`
	ResourceUpdateTime *string `json:"resource_update_time"`
	View               int     `json:"view"`
	Created            *string `json:"created"`
}

// v1Galgame is the /v1 aggregate detail record (GET /v1/galgame/{id}). Only the
// fields the moyu enricher reads off GalgameFull are typed.
type v1Galgame struct {
	ID               int         `json:"id"`
	Names            v1Names     `json:"names"`
	Intro            *v1Intro    `json:"intro"`
	ReleaseDate      *string     `json:"release_date"`
	ReleaseDateTBA   bool        `json:"release_date_tba"`
	OriginalLanguage string      `json:"original_language"`
	AgeLimit         string      `json:"age_limit"`
	Images           v1Images    `json:"images"`
	Taxonomy         *v1Taxonomy `json:"taxonomy"`
	CatalogWorkID    *int64      `json:"catalog_work_id"`
	Updated          string      `json:"updated"`
	Links            *[]v1Link   `json:"links"`
	Meta             *v1Meta     `json:"meta"`
}

// v1Item is the /v1 thin list/batch/search item (with include=meta expansion).
type v1Item struct {
	ID          int      `json:"id"`
	Names       v1Names  `json:"names"`
	ReleaseDate *string  `json:"release_date"`
	AgeLimit    string   `json:"age_limit"`
	Portrait    *v1Image `json:"portrait"`
	Banner      *v1Image `json:"banner"`
	Updated     string   `json:"updated"`
	Meta        *v1Meta  `json:"meta"`
}

// v1BatchData is the batch envelope ({items}, no total).
type v1BatchData struct {
	Items []v1Item `json:"items"`
}

// v1SearchData is the /v1 search envelope (+ optional pending under a user JWT).
type v1SearchData struct {
	Items   []v1Item  `json:"items"`
	Total   int64     `json:"total"`
	Pending *[]v1Item `json:"pending"`
}

// ─── mappers: /v1 → moyu-internal DTOs ────────────────────────────────

// hashFromURL extracts the content-addressed image hash from a /v1 sharded CDN
// URL ({base}/aa/bb/<hash>.webp): the basename minus the .webp extension. The
// bare-hash form the moyu CoverInput/GalgameBrief store carries. Returns "" for
// an empty URL (no pinned image).
func hashFromURL(u string) string {
	if u == "" {
		return ""
	}
	base := u
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		base = base[i+1:]
	}
	return strings.TrimSuffix(base, ".webp")
}

// v1ContentLimit translates the moyu content_limit convention to the /v1 wire.
// moyu's "" means "no filter — return the row / full brief regardless of grading"
// (the internal bridge's permissive default); on /v1 the absent param defaults to
// sfw (which also NSFW-strips a game's nsfw cover pins), so "" maps to "all" to
// preserve the permissive semantics. "all"/"nsfw" require the key's galgame:nsfw
// scope (moyu's internal key carries it since W1a P5) — without it /v1 silently
// falls back to sfw. sfw / nsfw / all pass through unchanged.
func v1ContentLimit(cl string) string {
	if cl == "" {
		return "all"
	}
	return cl
}

// str derefs a *string to its value ("" on nil) — the /v1 empty→null locale
// discipline collapses back to the moyu "" convention.
func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// zeroTS is the JSON rendering of a Go zero time.Time — the value the internal
// bridge brief emits for an un-set resource_update_time (EnrichBriefs formats
// g.ResourceUpdateTime.Time().UTC() with this layout, and the zero time renders
// as this literal). The /v1 meta drops it to null on zero, so this restores the
// bridge-parity value.
const zeroTS = "0001-01-01T00:00:00Z"

// resourceUpdateTime maps the /v1 meta.resource_update_time (*string, null on
// zero) to the moyu string field, restoring the bridge's zero-time literal.
func resourceUpdateTime(m *v1Meta) string {
	if m == nil || m.ResourceUpdateTime == nil {
		return zeroTS
	}
	return *m.ResourceUpdateTime
}

// v1ItemToBrief projects a /v1 thin item (with include=meta) onto the internal
// GalgameBrief the enricher consumes. Fields the internal batch brief does not
// carry (covers/screenshots/banner string) stay at their zero value, matching
// the bridge brief byte-for-byte.
func v1ItemToBrief(it *v1Item) GalgameBrief {
	b := GalgameBrief{
		ID:                 it.ID,
		NameEnUs:           str(it.Names.EnUS),
		NameZhCn:           str(it.Names.ZhCN),
		NameJaJp:           str(it.Names.JaJP),
		NameZhTw:           str(it.Names.ZhTW),
		AgeLimit:           it.AgeLimit,
		ReleaseDate:        it.ReleaseDate,
		ResourceUpdateTime: resourceUpdateTime(it.Meta),
	}
	if it.Meta != nil {
		b.VndbID = it.Meta.VNDBID
		b.Status = it.Meta.Status
		b.ContentLimit = it.Meta.ContentLimit
		b.OriginalLanguage = it.Meta.OriginalLanguage
		b.UserID = it.Meta.UserID
	}
	if it.Banner != nil {
		b.EffectiveBannerHash = hashFromURL(it.Banner.URL)
		b.EffectiveBannerWidth = it.Banner.Width
		b.EffectiveBannerHeight = it.Banner.Height
		b.EffectiveBannerThumbhash = it.Banner.Thumbhash
	}
	return b
}

// v1ItemToHit projects a /v1 thin item onto the internal GalgameHit (the search
// item shape). GalgameHit is a superset of GalgameBrief; the extra id-array /
// intro / view fields have no thin-item source and stay zero (the bridge search
// item does not carry them either).
func v1ItemToHit(it *v1Item) GalgameHit {
	h := GalgameHit{
		ID:          it.ID,
		NameEnUs:    str(it.Names.EnUS),
		NameZhCn:    str(it.Names.ZhCN),
		NameJaJp:    str(it.Names.JaJP),
		NameZhTw:    str(it.Names.ZhTW),
		AgeLimit:    it.AgeLimit,
		ReleaseDate: it.ReleaseDate,
	}
	if it.Meta != nil {
		h.VndbID = it.Meta.VNDBID
		h.Status = it.Meta.Status
		h.ContentLimit = it.Meta.ContentLimit
		h.OriginalLanguage = it.Meta.OriginalLanguage
		h.View = it.Meta.View
	}
	if it.Banner != nil {
		h.EffectiveBannerHash = hashFromURL(it.Banner.URL)
		h.EffectiveBannerWidth = it.Banner.Width
		h.EffectiveBannerHeight = it.Banner.Height
		h.EffectiveBannerThumbhash = it.Banner.Thumbhash
	}
	return h
}

// derefInt returns *p, or 0 when p is nil — the /v1 rating levels are pointer
// ints (absent when the key is not nsfw-capable); moyu's row fields are plain
// ints, so a missing level collapses to 0.
func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// v1CoversToInputs maps /v1 images.covers → the moyu CoverInput slice. image_hash
// is derived from the sharded URL; kind + dims + thumbhash carry over; sexual /
// violence come from the W1c scope-gated levels; sort_order is synthesized from
// the (server-ordered) array index. The /v1 curation still drops per-cover
// source/source_key — these have no /v1 source and stay "" (unconsumed by the
// moyu FE; W3 census).
func v1CoversToInputs(covers []v1Image) []CoverInput {
	if len(covers) == 0 {
		return nil
	}
	out := make([]CoverInput, 0, len(covers))
	for i := range covers {
		out = append(out, CoverInput{
			ImageHash: hashFromURL(covers[i].URL),
			SortOrder: i,
			Sexual:    derefInt(covers[i].Sexual),
			Violence:  derefInt(covers[i].Violence),
			Kind:      covers[i].Kind,
			Width:     covers[i].Width,
			Height:    covers[i].Height,
			Thumbhash: covers[i].Thumbhash,
		})
	}
	return out
}

// v1ScreenshotsToInputs maps /v1 images.screenshots → the moyu ScreenshotInput
// slice, incl. the W1c scope-gated sexual/violence levels + ungated caption +
// index-synthesized sort_order (drives the FE gallery filter/sort/labels). Only
// source/source_key stay "" (no /v1 source; FE-unconsumed).
func v1ScreenshotsToInputs(shots []v1Image) []ScreenshotInput {
	if len(shots) == 0 {
		return nil
	}
	out := make([]ScreenshotInput, 0, len(shots))
	for i := range shots {
		out = append(out, ScreenshotInput{
			ImageHash: hashFromURL(shots[i].URL),
			SortOrder: i,
			Caption:   shots[i].Caption,
			Sexual:    derefInt(shots[i].Sexual),
			Violence:  derefInt(shots[i].Violence),
			Width:     shots[i].Width,
			Height:    shots[i].Height,
			Thumbhash: shots[i].Thumbhash,
		})
	}
	return out
}

// v1GalgameToFull projects the /v1 aggregate detail record onto the internal
// GalgameFull the detail enricher consumes. include=intro,taxonomy,meta,
// tag_refs,official_refs,engine_refs,covers,screenshots,links must be requested
// so the mapped blocks are present.
func v1GalgameToFull(g *v1Galgame) GalgameFull {
	f := GalgameFull{
		ID:               g.ID,
		NameEnUs:         str(g.Names.EnUS),
		NameZhCn:         str(g.Names.ZhCN),
		NameJaJp:         str(g.Names.JaJP),
		NameZhTw:         str(g.Names.ZhTW),
		AgeLimit:         g.AgeLimit,
		OriginalLanguage: g.OriginalLanguage,
		ReleaseDate:      g.ReleaseDate,
		ReleaseDateTBA:   g.ReleaseDateTBA,
		Updated:          g.Updated,
		Covers:           v1CoversToInputs(g.Images.Covers),
		Screenshots:      v1ScreenshotsToInputs(g.Images.Screenshots),
	}
	if g.Intro != nil {
		f.IntroEnUs = str(g.Intro.EnUS)
		f.IntroZhCn = str(g.Intro.ZhCN)
		f.IntroJaJp = str(g.Intro.JaJP)
		f.IntroZhTw = str(g.Intro.ZhTW)
	}
	if g.Meta != nil {
		f.VndbID = g.Meta.VNDBID
		f.ContentLimit = g.Meta.ContentLimit
		f.View = g.Meta.View
		f.SeriesID = g.Meta.SeriesID
		f.Created = str(g.Meta.Created)
	}
	if g.Images.Banner != nil {
		f.EffectiveBannerHash = hashFromURL(g.Images.Banner.URL)
		f.EffectiveBannerWidth = g.Images.Banner.Width
		f.EffectiveBannerHeight = g.Images.Banner.Height
		f.EffectiveBannerThumbhash = g.Images.Banner.Thumbhash
	}
	if g.Taxonomy != nil {
		if g.Taxonomy.TagRefs != nil {
			for _, t := range *g.Taxonomy.TagRefs {
				f.Tag = append(f.Tag, galgameFullTag(t))
			}
		}
		if g.Taxonomy.OfficialRefs != nil {
			for _, o := range *g.Taxonomy.OfficialRefs {
				f.Official = append(f.Official, galgameFullOfficial(o))
			}
		}
		if g.Taxonomy.EngineRefs != nil {
			for _, e := range *g.Taxonomy.EngineRefs {
				f.Engine = append(f.Engine, galgameFullEngine(g.ID, e))
			}
		}
	}
	return f
}

// galgameFullTag builds one GalgameFull.Tag entry from a /v1 tag_ref. Aliases
// have no tag_ref source (the bridge detail's nested tag omits them too, so the
// enricher's PatchDetailTag.Aliases stays absent either way).
func galgameFullTag(t v1TagRef) struct {
	GalgameID    int `json:"galgame_id"`
	TagID        int `json:"tag_id"`
	SpoilerLevel int `json:"spoiler_level"`
	Tag          Tag `json:"tag"`
} {
	return struct {
		GalgameID    int `json:"galgame_id"`
		TagID        int `json:"tag_id"`
		SpoilerLevel int `json:"spoiler_level"`
		Tag          Tag `json:"tag"`
	}{
		TagID:        t.ID,
		SpoilerLevel: t.SpoilerLevel,
		Tag:          Tag{ID: t.ID, Name: t.Name, Category: t.Category},
	}
}

// galgameFullOfficial builds one GalgameFull.Official entry from a /v1
// official_ref.
func galgameFullOfficial(o v1OfficialRef) struct {
	GalgameID  int      `json:"galgame_id"`
	OfficialID int      `json:"official_id"`
	Official   Official `json:"official"`
} {
	return struct {
		GalgameID  int      `json:"galgame_id"`
		OfficialID int      `json:"official_id"`
		Official   Official `json:"official"`
	}{
		OfficialID: o.ID,
		Official:   Official{ID: o.ID, Name: o.Name, Category: o.Category, Lang: o.Lang},
	}
}

// galgameFullEngine builds one GalgameFull.Engine entry from a /v1 engine_ref.
func galgameFullEngine(gid int, e v1EngineRef) struct {
	GalgameID int            `json:"galgame_id"`
	EngineID  int            `json:"engine_id"`
	Engine    map[string]any `json:"engine"`
} {
	return struct {
		GalgameID int            `json:"galgame_id"`
		EngineID  int            `json:"engine_id"`
		Engine    map[string]any `json:"engine"`
	}{
		GalgameID: gid,
		EngineID:  e.ID,
		Engine:    map[string]any{"id": e.ID, "name": e.Name},
	}
}
