// Package enricher enriches local patch rows into the shape the frontend consumes directly.
//
// D12 (2026-04-21): the patch table no longer stores galgame metadata. It is
// fetched in bulk from Wiki /galgame/batch by galgame_id and assembled into the
// structure the frontend GalgameCard expects. All JSON keys are snake_case:
//
//	{
//	  id, vndb_id, bid, banner, view, download,
//	  name: { "en-us", "ja-jp", "zh-cn", "zh-tw" },
//	  type, language, platform,
//	  content_limit, status, created, resource_update_time,
//	  count: { favorite_by, contribute_by, resource, comment },
//	  galgame: { ...raw Wiki fields, optionally used by the detail page }
//	}
//
// When Wiki fails, string fields are empty but `count` stays accurate so the frontend does not break.
package enricher

import (
	"context"
	"log/slog"
	"time"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/infrastructure/markdown"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/userclient"
)

// KunLanguage mirrors the frontend KunLanguage (4 languages).
type KunLanguage struct {
	EnUs string `json:"en-us"`
	JaJp string `json:"ja-jp"`
	ZhCn string `json:"zh-cn"`
	ZhTw string `json:"zh-tw"`
}

// Counts mirrors the frontend `_count` nested object.
type Counts struct {
	FavoriteBy   int `json:"favorite_by"`
	ContributeBy int `json:"contribute_by"`
	Resource     int `json:"resource"`
	Comment      int `json:"comment"`
}

// GalgameCard is the Go mirror of the frontend `interface GalgameCard`.
// All JSON tags are snake_case to match the backend-wide convention.
type GalgameCard struct {
	ID                 int                  `json:"id"`
	Name               KunLanguage          `json:"name"`
	VndbID             string               `json:"vndb_id"`
	BID                *int                 `json:"bid"`
	Banner             string               `json:"banner"`
	View               int                  `json:"view"`
	Download           int                  `json:"download"`
	Type               patchModel.JSONArray `json:"type"`
	Language           patchModel.JSONArray `json:"language"`
	Platform           patchModel.JSONArray `json:"platform"`
	ContentLimit       string               `json:"content_limit"`
	Status             int                  `json:"status"`
	Created            time.Time            `json:"created"`
	ResourceUpdateTime time.Time            `json:"resource_update_time"`
	// ReleaseDate is the locally-mirrored wiki galgame.release_date (date
	// only; see migration 010 + backfill). Null when unknown. Surfaced so
	// list cards can render the release month and the date filter result is
	// legible. Day-precision time.Time → JSON RFC3339; the frontend formats
	// to YYYY-MM / YYYY-MM-DD.
	ReleaseDate *time.Time `json:"release_date,omitempty"`
	Count       Counts     `json:"count"`
	// User is the PATCH PUBLISHER (moyu's local patch.user_id — who registered
	// this galgame on moyu / uploaded its patches). It is moyu-owned data and
	// is what owner-gating (edit/delete) keys on. NOT the entry creator.
	User *patchModel.PatchUser `json:"user,omitempty"`
	// Creator is the GALGAME ENTRY CREATOR — the single source of truth owned by
	// Galgame Wiki (galgame.user_id, surfaced as GalgameBrief.UserID). Resolved
	// from the same OAuth user directory as User; kept SEPARATE so the "谁创建了
	// 这个词条" position uses wiki's value (aligned with kungal) while the patch
	// publisher stays its own thing. Nil when wiki has no creator / lookup miss.
	Creator *patchModel.PatchUser       `json:"creator,omitempty"`
	Galgame *galgameClient.GalgameBrief `json:"galgame,omitempty"`
}

// EnrichPatches enriches a batch of local patches with Wiki data into GalgameCards the frontend can render directly.
//
// A single /galgame/batch call covers all galgame_ids. If Wiki fails, only local fields are available (name is empty strings).
//
// If users is non-nil, publisher briefs are also batch-fetched from OAuth
// /users/batch and attached to each card's User field. Pass nil from callers
// that have no userclient handy or do not need publisher info.
//
// contentLimit is the NSFW filter forwarded to wiki/batch per
// docs/galgame_wiki/00-handbook-for-downstream.md §16 (sfw / nsfw / all).
// Pass "" to keep the legacy "no filter, preserve every patch" behavior
// (used by code paths that already hold a curated ID set — comment summaries,
// favorites — where dropping rows would surprise the caller).
// Pass "sfw" / "nsfw" / "all" for list/browse semantics: rows wiki filters out
// are *removed* from the returned slice (length may be < len(patches)). On
// wiki transient failure with a non-empty contentLimit we return nil rather
// than the unfiltered fallback — SEO safety beats showing names, since the
// fallback would surface NSFW patches that the caller explicitly tried to
// exclude.
func EnrichPatches(ctx context.Context, wiki *galgameClient.Client, users *userclient.Client, patches []patchModel.Patch, contentLimit string) []GalgameCard {
	cards := make([]GalgameCard, len(patches))
	for i := range patches {
		cards[i] = baseCard(&patches[i])
	}
	if len(patches) == 0 {
		return cards
	}

	attachUsersToCards(ctx, users, patches, cards)

	if wiki == nil {
		if contentLimit != "" {
			// No wiki = can't verify content_limit on any row. Refuse rather
			// than ship potentially NSFW names back to the caller.
			return nil
		}
		return cards
	}
	ids := collectGalgameIDs(patches)
	if len(ids) == 0 {
		return cards
	}

	briefs, err := wiki.GalgameBatch(ctx, ids, contentLimit)
	if err != nil {
		if contentLimit != "" {
			slog.Warn("Wiki 富化失败 + 处于过滤模式：返回空列表以防 NSFW 泄漏", "error", err, "count", len(patches), "content_limit", contentLimit)
			return nil
		}
		slog.Warn("Wiki 富化失败，返回无 galgame 的降级结果", "error", err, "count", len(patches))
		return cards
	}
	byID := make(map[int]*galgameClient.GalgameBrief, len(briefs))
	for i := range briefs {
		byID[briefs[i].ID] = &briefs[i]
	}

	if contentLimit != "" {
		// Filter mode: a patch.id missing from briefs means wiki filtered it
		// out (or it doesn't exist / isn't visible). Drop it from the result
		// rather than emitting a cardless row — list pages should show fewer
		// items, not stub rows pointing at filtered content.
		filtered := make([]GalgameCard, 0, len(briefs))
		for i := range cards {
			if g, ok := byID[patches[i].ID]; ok {
				applyGalgame(&cards[i], g)
				filtered = append(filtered, cards[i])
			}
		}
		return filtered
	}

	for i := range cards {
		if g, ok := byID[patches[i].ID]; ok {
			applyGalgame(&cards[i], g)
		}
	}
	return cards
}

// attachUsersToCards batch-fetches publisher briefs from OAuth and stamps the
// User field on each card. Best-effort -- on error the User field stays nil
// and the frontend renders the anonymous-fallback path.
func attachUsersToCards(ctx context.Context, users *userclient.Client, patches []patchModel.Patch, cards []GalgameCard) {
	if users == nil {
		return
	}
	uids := make([]int, 0, len(patches))
	for _, p := range patches {
		uids = append(uids, p.UserID)
	}
	briefs := userclient.BriefMapByInt(ctx, users, uids)
	for i := range cards {
		if b := briefs[patches[i].UserID]; b != nil {
			cards[i].User = &patchModel.PatchUser{
				ID:              int(b.ID),
				Name:            b.Name,
				Avatar:          b.Avatar,
				AvatarImageHash: b.AvatarImageHash,
				Roles:           b.Roles,
			}
		}
	}
}

// resolveUser turns a single OAuth user id into a PatchUser brief (name /
// avatar / roles) via the short-TTL userclient cache. Returns nil for a nil
// client, a non-positive id, or a lookup miss, so callers can treat "unknown
// user" uniformly (the frontend renders its anonymous fallback). Shared by the
// publisher (patch.user_id) and entry-creator (wiki galgame.user_id) lookups.
func resolveUser(ctx context.Context, users *userclient.Client, id int) *patchModel.PatchUser {
	if users == nil || id <= 0 {
		return nil
	}
	b, _ := users.User(ctx, uint(id))
	if b == nil {
		return nil
	}
	return &patchModel.PatchUser{
		ID:              int(b.ID),
		Name:            b.Name,
		Avatar:          b.Avatar,
		AvatarImageHash: b.AvatarImageHash,
		Roles:           b.Roles,
	}
}

// BuildPatchSummaryMap fetches Wiki briefs for the given patch IDs and returns
// a map keyed by patch_id (the local row id) of compact summaries. Patches
// whose galgame_id is missing or whose Wiki fetch fails are still included
// with empty Name/Banner so callers can render at least a link.
func BuildPatchSummaryMap(ctx context.Context, wiki *galgameClient.Client, db PatchSummaryDB, patchIDs []int) map[int]patchModel.PatchSummary {
	out := map[int]patchModel.PatchSummary{}
	if len(patchIDs) == 0 {
		return out
	}

	rows, err := db.LookupPatchesByIDs(patchIDs)
	if err != nil || len(rows) == 0 {
		return out
	}

	galgameIDs := make([]int, 0, len(rows))
	seen := make(map[int]struct{}, len(rows))
	for _, r := range rows {
		if r.ID > 0 {
			if _, ok := seen[r.ID]; !ok {
				seen[r.ID] = struct{}{}
				galgameIDs = append(galgameIDs, r.ID)
			}
		}
	}

	briefByGID := map[int]*galgameClient.GalgameBrief{}
	if wiki != nil && len(galgameIDs) > 0 {
		// No content_limit filter — summaries are attached to objects the user
		// is already viewing (their comments / favorited resources). The owning
		// patch's NSFW gate is the originating page's responsibility, not this
		// label-resolution helper. Matches wiki batch default per
		// docs/galgame_wiki/00-handbook §16.
		if briefs, err := wiki.GalgameBatch(ctx, galgameIDs, ""); err == nil {
			for i := range briefs {
				briefByGID[briefs[i].ID] = &briefs[i]
			}
		}
	}

	for _, r := range rows {
		s := patchModel.PatchSummary{ID: r.ID, VndbID: r.VndbID}
		if g, ok := briefByGID[r.ID]; ok {
			s.Banner = g.Banner
			s.Name = patchModel.PatchSummaryName{
				EnUs: g.NameEnUs,
				JaJp: g.NameJaJp,
				ZhCn: g.NameZhCn,
				ZhTw: g.NameZhTw,
			}
		}
		out[r.ID] = s
	}
	return out
}

// PatchSummaryDB is the minimal access surface BuildPatchSummaryMap needs.
// Callers typically supply a thin wrapper around their *gorm.DB so this
// package stays free of gorm imports.
type PatchSummaryDB interface {
	LookupPatchesByIDs(ids []int) ([]patchModel.Patch, error)
}

// EnrichPatch enriches a single patch (for the header card; no intro/tag/official).
// If users is non-nil, the publisher brief is also fetched and attached.
//
// Returns nil when contentLimit filters this patch out (the row exists but
// is the wrong content_limit) — the caller should translate that to a 404.
// On wiki transient failure with a non-empty contentLimit we also return nil
// rather than the unfiltered fallback (SEO safety beats fallback content).
// contentLimit semantics match docs/galgame_wiki/00-handbook §16:
//   - "" — no filter, wiki returns the row if it exists at all (legacy
//     behaviour for cases where a missing wiki row should still render with
//     local-only fields).
//   - "sfw" / "nsfw" / "all" — explicit filter; on miss we hard-fail to nil.
func EnrichPatch(ctx context.Context, wiki *galgameClient.Client, users *userclient.Client, p *patchModel.Patch, contentLimit string) *GalgameCard {
	if p == nil {
		return nil
	}
	card := baseCard(p)
	card.User = resolveUser(ctx, users, p.UserID) // 补丁发布者 (moyu patch.user_id)
	if wiki == nil || p.ID <= 0 {
		if contentLimit != "" {
			return nil
		}
		return &card
	}
	briefs, err := wiki.GalgameBatch(ctx, []int{p.ID}, contentLimit)
	if err != nil {
		slog.Warn("Wiki 富化失败", "galgame_id", p.ID, "error", err)
		if contentLimit != "" {
			return nil
		}
		return &card
	}
	if len(briefs) == 0 {
		// Either filtered out or genuinely not visible to the caller.
		return nil
	}
	applyGalgame(&card, &briefs[0])
	// 词条创建者 = wiki galgame.user_id (单一可信源，与 kungal 对齐)。applyGalgame
	// 已把 brief 挂到 card.Galgame，其 UserID 即创建者；与发布者分开解析，互不覆盖。
	if card.Galgame != nil {
		card.Creator = resolveUser(ctx, users, card.Galgame.UserID)
	}
	return &card
}

// PatchDetailTag is the lightweight tag shape surfaced to the patch detail page.
// Wiki returns spoiler_level alongside the tag, so we flatten it onto the same row.
type PatchDetailTag struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	Aliases      []string `json:"aliases,omitempty"`
	Category     string   `json:"category"`
	SpoilerLevel int      `json:"spoiler_level"`
}

// PatchDetailOfficial is the lightweight company / publisher shape for the patch detail page.
type PatchDetailOfficial struct {
	ID       int      `json:"id"`
	Name     string   `json:"name"`
	Aliases  []string `json:"aliases,omitempty"`
	Category string   `json:"category"`
	Lang     string   `json:"lang"`
}

// PatchDetailCard is for the detail page: the base GalgameCard plus intro and the
// resolved Wiki tags / officials / engine IDs. We embed the full tag/official
// objects (rather than just IDs) so the frontend can render names without a
// second round-trip to the Wiki Service.
//
// Both the raw markdown (`introduction_markdown`) and the rendered HTML
// (`introduction_html`) are returned: the frontend uses HTML for display and
// can fall back to markdown for editing.
type PatchDetailCard struct {
	GalgameCard
	IntroductionMarkdown KunLanguage           `json:"introduction_markdown"`
	IntroductionHTML     KunLanguage           `json:"introduction_html"`
	Updated              time.Time             `json:"updated"`
	Tags                 []PatchDetailTag      `json:"tags"`
	Officials            []PatchDetailOfficial `json:"officials"`
	WikiEngineIDs        []int                 `json:"wiki_engine_ids"`
}

// EnrichPatchDetail enriches the detail page: one extra /galgame/:gid call on top of EnrichPatch to get intro/associated IDs.
//
// Returns nil when contentLimit filters this patch out (wiki returns 404 for
// the row at this content_limit) — caller should translate that to its own
// 404. contentLimit semantics match EnrichPatch.
func EnrichPatchDetail(ctx context.Context, wiki *galgameClient.Client, users *userclient.Client, p *patchModel.Patch, contentLimit string) *PatchDetailCard {
	if p == nil {
		return nil
	}
	base := &PatchDetailCard{}
	base.GalgameCard = baseCard(p)
	base.Updated = p.Updated
	// Initialize the Wiki-derived slices to non-nil so an empty set serializes
	// as [] (not JSON null). The FE types declare them as non-optional arrays
	// (tags/officials/wiki_engine_ids); a null would break any .map/.length the
	// detail page does without a guard. Applies to every return path below.
	base.Tags = []PatchDetailTag{}
	base.Officials = []PatchDetailOfficial{}
	base.WikiEngineIDs = []int{}

	base.User = resolveUser(ctx, users, p.UserID) // 补丁发布者 (moyu patch.user_id)

	if wiki == nil || p.ID <= 0 {
		if contentLimit != "" {
			return nil
		}
		return base
	}
	env, err := wiki.GetGalgame(ctx, p.ID, contentLimit)
	if err != nil {
		// Wiki returns 404 for both "no such id" and "filtered out by
		// content_limit" (per docs/galgame_wiki/01-galgame.md GET /galgame/:gid).
		// In filter mode we hard-fail to nil — the caller can't disambiguate
		// transient from filter, and either way the safe move is "don't render".
		slog.Warn("Wiki 详情富化失败", "galgame_id", p.ID, "error", err)
		if contentLimit != "" {
			return nil
		}
		return base
	}

	g := &env.Galgame
	// Fill in basic fields like name / banner / content_limit
	base.Name = KunLanguage{
		EnUs: g.NameEnUs,
		JaJp: g.NameJaJp,
		ZhCn: g.NameZhCn,
		ZhTw: g.NameZhTw,
	}
	base.Banner = g.Banner
	base.ContentLimit = g.ContentLimit

	base.IntroductionMarkdown = KunLanguage{
		EnUs: g.IntroEnUs,
		JaJp: g.IntroJaJp,
		ZhCn: g.IntroZhCn,
		ZhTw: g.IntroZhTw,
	}
	base.IntroductionHTML = KunLanguage{
		EnUs: markdown.MustRender(g.IntroEnUs),
		JaJp: markdown.MustRender(g.IntroJaJp),
		ZhCn: markdown.MustRender(g.IntroZhCn),
		ZhTw: markdown.MustRender(g.IntroZhTw),
	}

	// Stamp the raw Wiki object so the edit form can pre-fill age_limit /
	// original_language without an extra round-trip. Brief fields only --
	// intro/tags/officials are surfaced via their own response fields.
	base.Galgame = &galgameClient.GalgameBrief{
		ID:                  g.ID,
		VndbID:              g.VndbID,
		NameEnUs:            g.NameEnUs,
		NameZhCn:            g.NameZhCn,
		NameJaJp:            g.NameJaJp,
		NameZhTw:            g.NameZhTw,
		Banner:              g.Banner,
		ContentLimit:        g.ContentLimit,
		AgeLimit:            g.AgeLimit,
		OriginalLanguage:    g.OriginalLanguage,
		ReleaseDate:         g.ReleaseDate,
		ReleaseDateTBA:      g.ReleaseDateTBA,
		EffectiveBannerHash:      g.EffectiveBannerHash,
		EffectiveBannerWidth:     g.EffectiveBannerWidth,
		EffectiveBannerHeight:    g.EffectiveBannerHeight,
		EffectiveBannerThumbhash: g.EffectiveBannerThumbhash,
		Covers:                   g.Covers,
		Screenshots:              g.Screenshots,
	}

	for _, t := range g.Tag {
		base.Tags = append(base.Tags, PatchDetailTag{
			ID:           t.Tag.ID,
			Name:         t.Tag.Name,
			Aliases:      t.Tag.Aliases,
			Category:     t.Tag.Category,
			SpoilerLevel: t.SpoilerLevel,
		})
	}
	for _, o := range g.Official {
		base.Officials = append(base.Officials, PatchDetailOfficial{
			ID:       o.Official.ID,
			Name:     o.Official.Name,
			Aliases:  o.Official.Aliases,
			Category: o.Official.Category,
			Lang:     o.Official.Lang,
		})
	}
	for _, e := range g.Engine {
		base.WikiEngineIDs = append(base.WikiEngineIDs, e.EngineID)
	}

	return base
}

// ─── helpers ──────────────────────────────────────

func collectGalgameIDs(patches []patchModel.Patch) []int {
	seen := map[int]struct{}{}
	ids := make([]int, 0, len(patches))
	for _, p := range patches {
		if p.ID <= 0 {
			continue
		}
		if _, ok := seen[p.ID]; ok {
			continue
		}
		seen[p.ID] = struct{}{}
		ids = append(ids, p.ID)
	}
	return ids
}

// baseCard builds the local-field portion of the card from a patch (Wiki-owned fields like Name/Banner start empty).
func baseCard(p *patchModel.Patch) GalgameCard {
	return GalgameCard{
		ID:                 p.ID,
		VndbID:             p.VndbID,
		BID:                p.BID,
		View:               p.View,
		Download:           p.Download,
		Type:               p.Type,
		Language:           p.Language,
		Platform:           p.Platform,
		Status:             p.Status,
		Created:            p.Created,
		ResourceUpdateTime: p.ResourceUpdateTime,
		ReleaseDate:        p.ReleaseDate,
		Count: Counts{
			FavoriteBy:   p.FavoriteCount,
			ContributeBy: p.ContributeCount,
			Resource:     p.ResourceCount,
			Comment:      p.CommentCount,
		},
		// User is filled by EnrichPatches/EnrichPatch via attachUsersToCards
		// (or stays nil when no userclient is provided). p.User is never
		// populated by GORM after the OAuth migration.
	}
}

// applyGalgame merges the Wiki galgame info into a card.
func applyGalgame(card *GalgameCard, g *galgameClient.GalgameBrief) {
	card.Name = KunLanguage{
		EnUs: g.NameEnUs,
		JaJp: g.NameJaJp,
		ZhCn: g.NameZhCn,
		ZhTw: g.NameZhTw,
	}
	card.Banner = g.Banner
	card.ContentLimit = g.ContentLimit
	card.Galgame = g
}

// CardFromBrief builds a GalgameCard from a Wiki brief alone (no local patch
// row). All moyu-side stats stay zero — used when enriching Wiki responses
// (tag/official detail) that include galgames moyu does not yet have a patch
// row for. The frontend can render the same card chrome and just show 0s.
func CardFromBrief(g *galgameClient.GalgameBrief) GalgameCard {
	if g == nil {
		return GalgameCard{}
	}
	// Init the JSONArray fields to non-nil so they serialize as [] not null —
	// this degraded card (a Wiki galgame with no local patch row) has no local
	// type/language/platform, and the FE type declares them as string[].
	card := GalgameCard{
		ID:       g.ID,
		VndbID:   g.VndbID,
		Type:     patchModel.JSONArray{},
		Language: patchModel.JSONArray{},
		Platform: patchModel.JSONArray{},
	}
	applyGalgame(&card, g)
	return card
}
