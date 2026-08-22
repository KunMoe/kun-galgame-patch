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

type KunLanguage = galgameClient.KunLanguage

type Counts struct {
	FavoriteBy   int `json:"favorite_by"`
	ContributeBy int `json:"contribute_by"`
	Resource     int `json:"resource"`
	Comment      int `json:"comment"`
}

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
	IsOnForum          bool                        `json:"is_on_forum"`
	Created            time.Time                   `json:"created"`
	ResourceUpdateTime time.Time                   `json:"resource_update_time"`
	ReleaseDate        *time.Time                  `json:"release_date,omitempty"`
	Count              Counts                      `json:"count"`
	User               *patchModel.PatchUser       `json:"user,omitempty"`
	Creator            *patchModel.PatchUser       `json:"creator,omitempty"`
	Galgame            *galgameClient.GalgameBrief `json:"galgame,omitempty"`
}

func EnrichPatches(ctx context.Context, galgame *galgameClient.Client, users *userclient.Client, patches []patchModel.Patch, contentLimit string) []GalgameCard {
	cards := make([]GalgameCard, len(patches))
	for i := range patches {
		cards[i] = baseCard(&patches[i])
	}
	if len(patches) == 0 {
		return cards
	}

	attachUsersToCards(ctx, users, patches, cards)

	if galgame == nil {
		if contentLimit != "" {
			return nil
		}
		return cards
	}
	ids := collectGalgameIDs(patches)
	if len(ids) == 0 {
		return cards
	}

	briefs, err := galgame.GalgameBatch(ctx, ids, contentLimit)
	if err != nil {
		if contentLimit != "" {
			slog.Warn("galgame 富化失败 + 处于过滤模式：返回空列表以防 NSFW 泄漏", "error", err, "count", len(patches), "content_limit", contentLimit)
			return nil
		}
		slog.Warn("galgame 富化失败，返回无 galgame 的降级结果", "error", err, "count", len(patches))
		return cards
	}
	byID := make(map[int]*galgameClient.GalgameBrief, len(briefs))
	for i := range briefs {
		byID[briefs[i].ID] = &briefs[i]
	}

	if contentLimit != "" {
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
				SiteRoles:       b.SiteRoles,
			}
		}
	}
}

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
		SiteRoles:       b.SiteRoles,
	}
}

func BuildPatchSummaryMap(ctx context.Context, galgame *galgameClient.Client, db PatchSummaryDB, patchIDs []int) map[int]patchModel.PatchSummary {
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
	if galgame != nil && len(galgameIDs) > 0 {
		if briefs, err := galgame.GalgameBatch(ctx, galgameIDs, ""); err == nil {
			for i := range briefs {
				briefByGID[briefs[i].ID] = &briefs[i]
			}
		}
	}

	for _, r := range rows {
		s := patchModel.PatchSummary{ID: r.ID, VndbID: r.VndbID}
		if g, ok := briefByGID[r.ID]; ok {
			s.Banner = g.Banner
			s.EffectiveBannerHash = g.EffectiveBannerHash
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

func resolveUserPtr(ctx context.Context, users *userclient.Client, id *int) *patchModel.PatchUser {
	if id == nil {
		return nil
	}
	return resolveUser(ctx, users, *id)
}

type PatchSummaryDB interface {
	LookupPatchesByIDs(ids []int) ([]patchModel.Patch, error)
}

func EnrichPatch(ctx context.Context, galgame *galgameClient.Client, users *userclient.Client, p *patchModel.Patch, contentLimit string) *GalgameCard {
	if p == nil {
		return nil
	}
	card := baseCard(p)
	card.User = resolveUser(ctx, users, p.UserID)
	if galgame == nil || p.ID <= 0 {
		if contentLimit != "" {
			return nil
		}
		return &card
	}
	briefs, err := galgame.GalgameBatch(ctx, []int{p.ID}, contentLimit)
	if err != nil {
		slog.Warn("galgame 富化失败", "galgame_id", p.ID, "error", err)
		if contentLimit != "" {
			return nil
		}
		return &card
	}
	if len(briefs) == 0 {
		return nil
	}
	applyGalgame(&card, &briefs[0])
	card.Creator = resolveUserPtr(ctx, users, p.CreatorID)
	return &card
}

type PatchDetailTag struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	Aliases      []string `json:"aliases,omitempty"`
	Category     string   `json:"category"`
	SpoilerLevel int      `json:"spoiler_level"`
}

type PatchDetailOfficial struct {
	ID       int      `json:"id"`
	Name     string   `json:"name"`
	Aliases  []string `json:"aliases,omitempty"`
	Category string   `json:"category"`
	Lang     string   `json:"lang"`
	LogoHash string   `json:"logo_hash"`
}

type PatchDetailCard struct {
	GalgameCard
	IntroductionMarkdown KunLanguage                       `json:"introduction_markdown"`
	IntroductionHTML     KunLanguage                       `json:"introduction_html"`
	Updated              time.Time                         `json:"updated"`
	Tags                 []PatchDetailTag                  `json:"tags"`
	Officials            []PatchDetailOfficial             `json:"officials"`
	Characters           []galgameClient.GalgameCharacter  `json:"characters"`
	Staff                []galgameClient.GalgameStaffGroup `json:"staff"`
	Ratings              []galgameClient.GalgameRating     `json:"ratings"`
	Series               []galgameClient.GalgameSeries     `json:"series"`
}

// applyCatalogEntities copies the catalog-owned entity graph onto the detail
// card. Every field it writes is a read-only projection of the registry: moyu
// stores none of it and never writes any of it back.
func applyCatalogEntities(base *PatchDetailCard, g *galgameClient.GalgameFull) {
	base.Characters = g.Characters
	base.Staff = g.Staff
	base.Ratings = g.Ratings
	base.Series = g.Series
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
			LogoHash: o.Official.LogoHash,
		})
	}
}

func EnrichPatchDetail(ctx context.Context, galgame *galgameClient.Client, users *userclient.Client, p *patchModel.Patch, contentLimit string) *PatchDetailCard {
	if p == nil {
		return nil
	}
	base := &PatchDetailCard{}
	base.GalgameCard = baseCard(p)
	base.Updated = p.Updated
	base.Tags = []PatchDetailTag{}
	base.Officials = []PatchDetailOfficial{}
	base.Characters = []galgameClient.GalgameCharacter{}
	base.Staff = []galgameClient.GalgameStaffGroup{}
	base.Ratings = []galgameClient.GalgameRating{}

	base.User = resolveUser(ctx, users, p.UserID)

	if galgame == nil || p.ID <= 0 {
		if contentLimit != "" {
			return nil
		}
		return base
	}
	env, err := galgame.GetGalgame(ctx, p.ID, contentLimit)
	if err != nil {
		slog.Warn("galgame 详情富化失败", "galgame_id", p.ID, "error", err)
		if contentLimit != "" {
			return nil
		}
		return base
	}

	g := &env.Galgame
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

	base.Galgame = &galgameClient.GalgameBrief{
		ID:                         g.ID,
		VndbID:                     g.VndbID,
		NameEnUs:                   g.NameEnUs,
		NameZhCn:                   g.NameZhCn,
		NameJaJp:                   g.NameJaJp,
		NameZhTw:                   g.NameZhTw,
		Banner:                     g.Banner,
		ContentLimit:               g.ContentLimit,
		AgeLimit:                   g.AgeLimit,
		OriginalLanguage:           g.OriginalLanguage,
		ReleaseDate:                g.ReleaseDate,
		EffectiveBannerHash:        g.EffectiveBannerHash,
		EffectiveBannerWidth:       g.EffectiveBannerWidth,
		EffectiveBannerHeight:      g.EffectiveBannerHeight,
		EffectiveBannerThumbhash:   g.EffectiveBannerThumbhash,
		EffectivePortraitHash:      g.EffectivePortraitHash,
		EffectivePortraitWidth:     g.EffectivePortraitWidth,
		EffectivePortraitHeight:    g.EffectivePortraitHeight,
		EffectivePortraitThumbhash: g.EffectivePortraitThumbhash,
		Covers:                     g.Covers,
		Screenshots:                g.Screenshots,
	}

	applyCatalogEntities(base, g)
	return base
}

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
		IsOnForum:          true,
		Created:            p.Created,
		ResourceUpdateTime: p.ResourceUpdateTime,
		ReleaseDate:        p.ReleaseDate,
		Count: Counts{
			FavoriteBy:   p.FavoriteCount,
			ContributeBy: p.ContributeCount,
			Resource:     p.ResourceCount,
			Comment:      p.CommentCount,
		},
	}
}

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

func CardFromBrief(g *galgameClient.GalgameBrief) GalgameCard {
	if g == nil {
		return GalgameCard{}
	}
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

type CalendarCard struct {
	GalgameCard
	HasPatch   bool   `json:"has_patch"`
	IsFavorite bool   `json:"is_favorite"`
	ClaimState string `json:"claim_state"`
}

func EnrichCalendarBriefs(briefs []galgameClient.GalgameBrief, hasPatch map[int]bool) []CalendarCard {
	cards := make([]CalendarCard, 0, len(briefs))
	for i := range briefs {
		cards = append(cards, CalendarCard{
			GalgameCard: CardFromBrief(&briefs[i]),
			HasPatch:    hasPatch[briefs[i].ID],
			ClaimState:  briefs[i].ClaimState,
		})
	}
	return cards
}

func GalgameOnlyCard(ctx context.Context, galgame *galgameClient.Client, users *userclient.Client, gid int, contentLimit string) *GalgameCard {
	if galgame == nil || gid <= 0 {
		return nil
	}
	briefs, err := galgame.GalgameBatch(ctx, []int{gid}, contentLimit)
	if err != nil {
		return nil
	}
	var brief *galgameClient.GalgameBrief
	for i := range briefs {
		if briefs[i].ID == gid {
			brief = &briefs[i]
			break
		}
	}
	if brief == nil {
		return nil
	}
	card := CardFromBrief(brief)
	return &card
}

func GalgameOnlyDetail(ctx context.Context, galgame *galgameClient.Client, users *userclient.Client, gid int, contentLimit string) *PatchDetailCard {
	if galgame == nil || gid <= 0 {
		return nil
	}
	env, err := galgame.GetGalgame(ctx, gid, contentLimit)
	if err != nil {
		return nil
	}
	g := &env.Galgame
	base := &PatchDetailCard{}
	base.GalgameCard = GalgameCard{
		ID:       g.ID,
		VndbID:   g.VndbID,
		Type:     patchModel.JSONArray{},
		Language: patchModel.JSONArray{},
		Platform: patchModel.JSONArray{},
	}
	base.Name = KunLanguage{EnUs: g.NameEnUs, JaJp: g.NameJaJp, ZhCn: g.NameZhCn, ZhTw: g.NameZhTw}
	base.Banner = g.Banner
	base.ContentLimit = g.ContentLimit
	if t, perr := time.Parse(time.RFC3339, g.Created); perr == nil {
		base.Created = t
	}
	if t, perr := time.Parse(time.RFC3339, g.Updated); perr == nil {
		base.Updated = t
	}
	base.Tags = []PatchDetailTag{}
	base.Officials = []PatchDetailOfficial{}
	base.Characters = []galgameClient.GalgameCharacter{}
	base.Staff = []galgameClient.GalgameStaffGroup{}
	base.Ratings = []galgameClient.GalgameRating{}
	base.IntroductionMarkdown = KunLanguage{EnUs: g.IntroEnUs, JaJp: g.IntroJaJp, ZhCn: g.IntroZhCn, ZhTw: g.IntroZhTw}
	base.IntroductionHTML = KunLanguage{
		EnUs: markdown.MustRender(g.IntroEnUs),
		JaJp: markdown.MustRender(g.IntroJaJp),
		ZhCn: markdown.MustRender(g.IntroZhCn),
		ZhTw: markdown.MustRender(g.IntroZhTw),
	}
	base.Galgame = &galgameClient.GalgameBrief{
		ID:                         g.ID,
		VndbID:                     g.VndbID,
		NameEnUs:                   g.NameEnUs,
		NameZhCn:                   g.NameZhCn,
		NameJaJp:                   g.NameJaJp,
		NameZhTw:                   g.NameZhTw,
		Banner:                     g.Banner,
		ContentLimit:               g.ContentLimit,
		AgeLimit:                   g.AgeLimit,
		OriginalLanguage:           g.OriginalLanguage,
		ReleaseDate:                g.ReleaseDate,
		EffectiveBannerHash:        g.EffectiveBannerHash,
		EffectiveBannerWidth:       g.EffectiveBannerWidth,
		EffectiveBannerHeight:      g.EffectiveBannerHeight,
		EffectiveBannerThumbhash:   g.EffectiveBannerThumbhash,
		EffectivePortraitHash:      g.EffectivePortraitHash,
		EffectivePortraitWidth:     g.EffectivePortraitWidth,
		EffectivePortraitHeight:    g.EffectivePortraitHeight,
		EffectivePortraitThumbhash: g.EffectivePortraitThumbhash,
		Covers:                     g.Covers,
		Screenshots:                g.Screenshots,
	}
	applyCatalogEntities(base, g)
	return base
}
