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
	ID                 int                         `json:"id"`
	Name               KunLanguage                 `json:"name"`
	VndbID             string                      `json:"vndb_id"`
	BID                *int                        `json:"bid"`
	Banner             string                      `json:"banner"`
	View               int                         `json:"view"`
	Download           int                         `json:"download"`
	Type               patchModel.JSONArray        `json:"type"`
	Language           patchModel.JSONArray        `json:"language"`
	Platform           patchModel.JSONArray        `json:"platform"`
	ContentLimit       string                      `json:"content_limit"`
	Status             int                         `json:"status"`
	Created            time.Time                   `json:"created"`
	ResourceUpdateTime time.Time                   `json:"resource_update_time"`
	Count              Counts                      `json:"count"`
	User               *patchModel.PatchUser       `json:"user,omitempty"`
	Galgame            *galgameClient.GalgameBrief `json:"galgame,omitempty"`
}

// EnrichPatches enriches a batch of local patches with Wiki data into GalgameCards the frontend can render directly.
//
// A single /galgame/batch call covers all galgame_ids. If Wiki fails, only local fields are available (name is empty strings).
func EnrichPatches(ctx context.Context, wiki *galgameClient.Client, patches []patchModel.Patch) []GalgameCard {
	cards := make([]GalgameCard, len(patches))
	for i := range patches {
		cards[i] = baseCard(&patches[i])
	}
	if wiki == nil || len(patches) == 0 {
		return cards
	}

	ids := collectGalgameIDs(patches)
	if len(ids) == 0 {
		return cards
	}

	briefs, err := wiki.GalgameBatch(ctx, ids)
	if err != nil {
		slog.Warn("Wiki 富化失败，返回无 galgame 的降级结果", "error", err, "count", len(patches))
		return cards
	}
	byID := make(map[int]*galgameClient.GalgameBrief, len(briefs))
	for i := range briefs {
		byID[briefs[i].ID] = &briefs[i]
	}

	for i := range cards {
		if g, ok := byID[patches[i].GalgameID]; ok {
			applyGalgame(&cards[i], g)
		}
	}
	return cards
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
		if r.GalgameID > 0 {
			if _, ok := seen[r.GalgameID]; !ok {
				seen[r.GalgameID] = struct{}{}
				galgameIDs = append(galgameIDs, r.GalgameID)
			}
		}
	}

	briefByGID := map[int]*galgameClient.GalgameBrief{}
	if wiki != nil && len(galgameIDs) > 0 {
		if briefs, err := wiki.GalgameBatch(ctx, galgameIDs); err == nil {
			for i := range briefs {
				briefByGID[briefs[i].ID] = &briefs[i]
			}
		}
	}

	for _, r := range rows {
		s := patchModel.PatchSummary{ID: r.ID, VndbID: r.VndbID}
		if g, ok := briefByGID[r.GalgameID]; ok {
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
func EnrichPatch(ctx context.Context, wiki *galgameClient.Client, p *patchModel.Patch) GalgameCard {
	if p == nil {
		return GalgameCard{}
	}
	card := baseCard(p)
	if wiki == nil || p.GalgameID <= 0 {
		return card
	}
	briefs, err := wiki.GalgameBatch(ctx, []int{p.GalgameID})
	if err != nil || len(briefs) == 0 {
		slog.Warn("Wiki 富化失败", "patch_id", p.ID, "error", err)
		return card
	}
	applyGalgame(&card, &briefs[0])
	return card
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
func EnrichPatchDetail(ctx context.Context, wiki *galgameClient.Client, p *patchModel.Patch) PatchDetailCard {
	base := PatchDetailCard{}
	if p == nil {
		return base
	}
	base.GalgameCard = baseCard(p)
	base.Updated = p.Updated

	if wiki == nil || p.GalgameID <= 0 {
		return base
	}
	env, err := wiki.GetGalgame(ctx, p.GalgameID)
	if err != nil {
		slog.Warn("Wiki 详情富化失败", "patch_id", p.ID, "galgame_id", p.GalgameID, "error", err)
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
		if p.GalgameID <= 0 {
			continue
		}
		if _, ok := seen[p.GalgameID]; ok {
			continue
		}
		seen[p.GalgameID] = struct{}{}
		ids = append(ids, p.GalgameID)
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
		Count: Counts{
			FavoriteBy:   p.FavoriteCount,
			ContributeBy: p.ContributeCount,
			Resource:     p.ResourceCount,
			Comment:      p.CommentCount,
		},
		User: p.User,
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
