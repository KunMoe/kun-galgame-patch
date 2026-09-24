package common

import (
	"context"
	stderrors "errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/galgame/enricher"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/upstream"
	"kun-galgame-patch-api/pkg/userclient"
	"kun-galgame-patch-api/pkg/utils"

	"gorm.io/gorm"
)

// One keyword across the three things moyu holds: the games catalog answers
// for, the patch resources uploaded here, and the accounts that uploaded them.

const (
	// How many rows each lane contributes to the all-categories overview.
	// Games get a grid, the other two get rows.
	searchOverviewGalgameLimit  = 12
	searchOverviewResourceLimit = 6
	searchOverviewUserLimit     = 8
	searchOverviewEntityLimit   = 6

	// Per lane in the command palette, which is a preview of the search page
	// rather than a second one.
	searchQuickLimit = 5

	// OAuth's /users/search takes q and limit only — it has no offset, so the
	// page is cut here out of one capped fetch rather than asked for upstream.
	searchUserMax = 50

	// The ceiling on how many games one keyword can pull resources in by.
	searchResourceGalgameIDs = 60

	// The 补丁资源 lane's narrow match: the AI model the uploader recorded, and
	// nothing else.
	searchScopeModel = "model"
)

type searchUserItem struct {
	ID              int      `json:"id"`
	Name            string   `json:"name"`
	Avatar          string   `json:"avatar"`
	AvatarImageHash string   `json:"avatar_image_hash"`
	Bio             string   `json:"bio"`
	Roles           []string `json:"roles"`
	SiteRoles       []string `json:"site_roles"`
	Moemoepoint     int      `json:"moemoepoint"`
	PatchCount      int      `json:"patch_count"`
	ResourceCount   int      `json:"resource_count"`
	CommentCount    int      `json:"comment_count"`
}

type searchTotals struct {
	Galgame  int64 `json:"galgame"`
	Entity   int64 `json:"entity"`
	Resource int64 `json:"resource"`
	User     int64 `json:"user"`
}

type searchLanes struct {
	Galgames  []enricher.GalgameCard     `json:"galgames"`
	Entities  []searchEntityGroup        `json:"entities"`
	Resources []patchModel.PatchResource `json:"resources"`
	Users     []searchUserItem           `json:"users"`
	Totals    searchTotals               `json:"totals"`
}

// A zero entity limit skips the 资料库 lane: it is five upstream requests, which
// the command palette cannot afford on a debounced keystroke.
type searchLaneLimits struct {
	galgame  int
	entity   int
	resource int
	user     int
}

func searchKeywords(raw string) []string {
	return strings.Fields(strings.TrimSpace(raw))
}

// runSearchLanes answers the overview and the command palette: every lane at
// once, so the reader sees which categories their keyword actually lives in
// before choosing one. A lane that fails is dropped rather than failing the
// request — catalog and OAuth are both remote.
func (h *CommonHandler) runSearchLanes(
	ctx context.Context, raw, cl string, lim searchLaneLimits,
) searchLanes {
	out := searchLanes{
		Galgames:  []enricher.GalgameCard{},
		Entities:  []searchEntityGroup{},
		Resources: []patchModel.PatchResource{},
		Users:     []searchUserItem{},
	}

	var wg sync.WaitGroup
	run := func(name string, lane func()) {
		wg.Add(1)
		go func() {
			// fiber's recover middleware only wraps the handler goroutine, so a
			// panic in a lane would take the process down with it instead of
			// failing this one request.
			defer func() {
				if r := recover(); r != nil {
					slog.Error("站内搜索 lane panic", "lane", name, "panic", r)
				}
			}()
			defer wg.Done()
			lane()
		}()
	}

	run("galgame", func() {
		cards, total, err := h.searchGalgameLane(ctx, raw, 1, lim.galgame, galgameSearchFilter{})
		if err != nil {
			slog.Warn("站内搜索 galgame lane 失败", "error", err)
			return
		}
		out.Galgames, out.Totals.Galgame = cards, total
	})
	if lim.entity > 0 {
		run("entity", func() {
			groups, appErr := h.searchEntityLane(ctx, raw, "", 1, lim.entity, cl)
			if appErr != nil {
				slog.Warn("站内搜索 entity lane 失败", "error", appErr.Message)
				return
			}
			out.Entities = groups
			for _, group := range groups {
				out.Totals.Entity += group.Total
			}
		})
	}
	run("resource", func() {
		rows, total, appErr := h.searchResourceLane(ctx, raw, 1, lim.resource, cl, "")
		if appErr != nil {
			slog.Warn("站内搜索 resource lane 失败", "error", appErr.Message)
			return
		}
		out.Resources, out.Totals.Resource = rows, total
	})
	run("user", func() {
		users, total, appErr := h.searchUserLane(ctx, raw, 1, lim.user)
		if appErr != nil {
			slog.Warn("站内搜索 user lane 失败", "error", appErr.Message)
			return
		}
		out.Users, out.Totals.User = users, total
	})
	wg.Wait()

	return out
}

// galgameSearchFilter is what the 高级筛选 panel adds to a keyword. Catalog keeps
// company_id / tag_id / released_* live on the search index, so a filtered
// search is the same single request an unfiltered one is. There is no rating
// here because the index carries no rating attribute: filtering or sorting by
// 评分 is an infra change, not a parameter this side can add.
type galgameSearchFilter struct {
	TagIDs       []int
	CompanyID    int
	ReleasedFrom int
	ReleasedTo   int
	Sort         string
}

// searchGalgameLane names every game the catalog holds, NSFW included: what a
// reader's gate hides is the download, not the title, and /patch/:id runs its
// own gate when a card is opened. kungal's search page draws the same line.
func (h *CommonHandler) searchGalgameLane(
	ctx context.Context, raw string, page, limit int, f galgameSearchFilter,
) ([]enricher.GalgameCard, int64, error) {
	if h.galgame == nil {
		return nil, 0, stderrors.New("galgame client is not configured")
	}
	res, err := h.galgame.SearchGalgame(ctx, galgameClient.SearchGalgameParams{
		Q:            raw,
		ContentLimit: utils.ContentLimitAll,
		Page:         page,
		Limit:        limit,
		Sort:         f.Sort,
		TagIDs:       f.TagIDs,
		OfficialIDs:  companyIDs(f.CompanyID),
		ReleasedFrom: f.ReleasedFrom,
		ReleasedTo:   f.ReleasedTo,
	})
	if err != nil {
		return nil, 0, err
	}

	ids := make([]int, 0, len(res.Items))
	for i := range res.Items {
		if res.Items[i].ID > 0 {
			ids = append(ids, res.Items[i].ID)
		}
	}
	return overlayCatalogHits(res.Items, h.localPatchMap(ids), ""), res.Total, nil
}

func companyIDs(id int) []int {
	if id <= 0 {
		return nil
	}
	return []int{id}
}

// searchResourceLane matches a resource through what its uploader typed and
// through the game it hangs off. The second half arrives as galgame ids because
// moyu keeps no local copy of a title, and it is what makes this lane useful at
// all: on a patch site "搜索资源" almost always means "找某个游戏的补丁".
//
// searchScopeModel narrows all of that to the AI model the uploader recorded.
// The wide lane already matches model_name, but it also matches the game and
// the note, so "claude" answers every 汉化 whose note mentions it; the narrow
// lane is what "按模型搜索资源" meant before the search page was rewritten.
func (h *CommonHandler) searchResourceLane(
	ctx context.Context, raw string, page, limit int, cl, scope string,
) ([]patchModel.PatchResource, int64, *errors.AppError) {
	keywords := searchKeywords(raw)
	if len(keywords) == 0 {
		return nil, 0, errors.ErrBadRequest("搜索关键词不能为空")
	}
	modelOnly := scope == searchScopeModel

	var gids []int
	if h.galgame != nil && !modelOnly {
		matched, err := h.galgame.SearchGalgameIDs(ctx, raw, cl, searchResourceGalgameIDs)
		if err != nil {
			slog.Warn("资源搜索的游戏名匹配失败，降级为仅匹配上传者填写的内容", "error", err)
		}
		gids = matched
	}

	// LEFT JOIN, not INNER: a resource whose patch row went missing is still a
	// row every other list shows, and ScopePatchContentLimit already reads a
	// NULL content_limit as "not mirrored yet, let it through".
	base := h.db.WithContext(ctx).Model(&patchModel.PatchResource{}).
		Joins("LEFT JOIN patch ON patch.id = patch_resource.galgame_id").
		Where("patch_resource.status = 0")
	for _, kw := range keywords {
		like := "%" + kw + "%"
		if modelOnly {
			base = base.Where("patch_resource.model_name ILIKE ?", like)
			continue
		}
		if len(gids) > 0 {
			base = base.Where(`(patch_resource.name ILIKE ? OR patch_resource.note ILIKE ?
				OR patch_resource.model_name ILIKE ? OR patch_resource.localization_group_name ILIKE ?
				OR patch_resource.galgame_id IN ?)`, like, like, like, like, gids)
			continue
		}
		base = base.Where(`(patch_resource.name ILIKE ? OR patch_resource.note ILIKE ?
			OR patch_resource.model_name ILIKE ? OR patch_resource.localization_group_name ILIKE ?)`,
			like, like, like, like)
	}
	base = utils.ScopePatchContentLimit(base, cl)

	var total int64
	base.Session(&gorm.Session{}).Count(&total)

	selection := "patch_resource.*"
	order := "patch_resource.download DESC, patch_resource.id DESC"
	var args []any
	if len(gids) > 0 {
		selection += ", (CASE WHEN patch_resource.galgame_id IN ? THEN 1 ELSE 0 END) AS relevance"
		args = append(args, gids)
		order = "relevance DESC, " + order
	}

	var resources []patchModel.PatchResource
	if err := base.Session(&gorm.Session{}).
		Select(selection, args...).
		Order(order).
		Offset((page - 1) * limit).Limit(limit).
		Find(&resources).Error; err != nil {
		return nil, 0, errors.ErrInternal("")
	}

	resources = enricher.FilterByGalgameContentLimit(ctx, h.galgame, resources,
		func(r patchModel.PatchResource) int { return r.GalgameID }, cl)
	patchModel.RenderResourceNotes(resources)
	h.attachResourceUsers(ctx, resources)
	h.attachPatchSummaries(ctx, resources)
	patchModel.StripResourceSecrets(resources)
	return resources, total, nil
}

func (h *CommonHandler) searchUserLane(
	ctx context.Context, raw string, page, limit int,
) ([]searchUserItem, int64, *errors.AppError) {
	if h.users == nil {
		return nil, 0, errors.ErrInternal("用户搜索未启用")
	}
	briefs, err := h.users.Search(ctx, raw, searchUserMax)
	if err != nil {
		userclient.LogFailure(ctx, "站内搜索 OAuth users/search failed", err)
		switch upstream.KindOf(err) {
		case upstream.Unavailable:
			return nil, 0, errors.New(50300, "登录服务暂不可用，请稍后再试", http.StatusServiceUnavailable)
		case upstream.RateLimited:
			return nil, 0, errors.ErrTooManyRequests("操作过于频繁，请稍后再试")
		}
		return nil, 0, errors.ErrInternal("用户搜索失败")
	}

	items := make([]searchUserItem, 0, len(briefs))
	for _, b := range briefs {
		if b == nil || b.Status != 0 {
			continue
		}
		items = append(items, searchUserItem{
			ID:              int(b.ID),
			Name:            b.Name,
			Avatar:          b.Avatar,
			AvatarImageHash: b.AvatarImageHash,
			Bio:             b.Bio,
			Roles:           b.Roles,
			SiteRoles:       b.SiteRoles,
		})
	}

	total := int64(len(items))
	start := (page - 1) * limit
	if start < 0 || start >= len(items) {
		return []searchUserItem{}, total, nil
	}
	items = items[start:min(start+limit, len(items))]
	h.attachSearchUserCounts(ctx, items)
	return items, total, nil
}

// attachSearchUserCounts fills the same four numbers /ranking/user prints, from
// the same subqueries, so a user never carries two different tallies.
func (h *CommonHandler) attachSearchUserCounts(ctx context.Context, items []searchUserItem) {
	if len(items) == 0 {
		return
	}
	ids := make([]int, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}

	type row struct {
		ID            int `gorm:"column:id"`
		Moemoepoint   int `gorm:"column:moemoepoint"`
		PatchCount    int `gorm:"column:patch_count"`
		ResourceCount int `gorm:"column:resource_count"`
	}
	var rows []row
	if err := h.db.WithContext(ctx).Table(`"user" u`).
		Select(`u.id, u.moemoepoint,
			COALESCE((SELECT COUNT(*) FROM patch p WHERE p.user_id = u.id), 0) AS patch_count,
			COALESCE((SELECT COUNT(*) FROM patch_resource pr WHERE pr.user_id = u.id), 0) AS resource_count`).
		Where("u.id IN ?", ids).Find(&rows).Error; err != nil {
		slog.Warn("用户搜索的贡献统计失败", "error", err)
		return
	}

	byID := make(map[int]row, len(rows))
	for _, r := range rows {
		byID[r.ID] = r
	}
	// The comment count is the community primitive's, not a local COUNT: this
	// site stores no comments any more. The batch caps at 100 ids, which is
	// above this lane's own ceiling of 50.
	commentCounts := h.comments.AuthorCounts(ctx, ids)
	for i := range items {
		r, ok := byID[items[i].ID]
		if !ok {
			continue
		}
		items[i].Moemoepoint = r.Moemoepoint
		items[i].PatchCount = r.PatchCount
		items[i].ResourceCount = r.ResourceCount
		items[i].CommentCount = int(commentCounts[items[i].ID])
	}
}
