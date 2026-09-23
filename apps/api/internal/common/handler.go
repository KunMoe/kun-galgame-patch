package common

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"slices"
	"strconv"
	"strings"

	commentService "kun-galgame-patch-api/internal/comment/service"
	"kun-galgame-patch-api/internal/favorite"
	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/galgame/enricher"
	"kun-galgame-patch-api/internal/infrastructure/markdown"
	"kun-galgame-patch-api/internal/middleware"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/artifactclient"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/userclient"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

type CommonHandler struct {
	db      *gorm.DB
	galgame *galgameClient.Client
	users   *userclient.Client
	art     *artifactclient.Client
	// comments answers the two things this file still needs from the comment
	// walls now that they live in the community primitive: the home page's
	// newest rows, and how many comments a listed user has written.
	comments CommentSource
}

// CommentSource is the community-backed comment service, narrowed to what the
// mixed lists here ask of it.
type CommentSource interface {
	SiteFeed(ctx context.Context, cursor string, limit int, cl string, db commentService.PatchSummaryDB) (*commentService.FeedPage, *errors.AppError)
	AuthorBoard(ctx context.Context, limit int) []int
	AuthorCounts(ctx context.Context, userIDs []int) map[int]int64
}

func NewHandler(db *gorm.DB, galgame *galgameClient.Client, users *userclient.Client, art *artifactclient.Client, comments CommentSource) *CommonHandler {
	return &CommonHandler{db: db, galgame: galgame, users: users, art: art, comments: comments}
}

func (h *CommonHandler) attachResourceUsers(ctx context.Context, rs []patchModel.PatchResource) {
	if len(rs) == 0 {
		return
	}
	uids := make([]int, 0, len(rs))
	for _, r := range rs {
		uids = append(uids, r.UserID)
	}
	briefs := userclient.BriefMapByInt(ctx, h.users, uids)
	for i := range rs {
		if b := briefs[rs[i].UserID]; b != nil {
			rs[i].User = patchModel.NewPatchUser(b)
		}
	}
}

type patchSummaryFinder struct{ db *gorm.DB }

func (p patchSummaryFinder) LookupPatchesByIDs(ids []int) ([]patchModel.Patch, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []patchModel.Patch
	err := p.db.Select("id", "vndb_id").
		Where("id IN ?", ids).Find(&rows).Error
	return rows, err
}

type homeResponse struct {
	Galgames  []enricher.GalgameCard     `json:"galgames"`
	Resources []patchModel.PatchResource `json:"resources"`
	Comments  []*commentService.FeedItem `json:"comments"`
}

const homeGalgameCount = 12

// EnrichPatches drops the rows the catalog answers as outside the reader's
// content limit, and it runs after the SQL LIMIT — so asking for 12 rendered 6
// under the default SFW gate. ScopePatchContentLimit now filters the axis in
// SQL, but a row catalog has not been mirrored for yet still passes it and can
// still be dropped, so the strip keeps reading wider and cutting. The extra
// rows cost nothing: enrichment batches up to 100 ids in one catalog call
// either way.
const homeGalgameOverfetch = 4

const homeCommentCount = 6

func (h *CommonHandler) GetHome(c fiber.Ctx) error {
	cl := utils.ContentLimitForListBrowse(c)

	var patches []patchModel.Patch
	var resources []patchModel.PatchResource

	patchQuery := h.db.Model(&patchModel.Patch{}).Order("created DESC, id DESC").
		Limit(homeGalgameCount * homeGalgameOverfetch)
	if !utils.IncludeEmptyGalgames(c) {
		patchQuery = patchQuery.Where("resource_count > 0")
	}
	patchQuery = utils.ScopePatchContentLimit(patchQuery, cl)
	patchQuery.Find(&patches)
	h.db.Model(&patchModel.PatchResource{}).Where("status = 0").Order("created DESC, id DESC").Limit(6).Find(&resources)

	resources = enricher.FilterByGalgameContentLimit(c.Context(), h.galgame, resources, func(r patchModel.PatchResource) int { return r.GalgameID }, cl)

	patchModel.RenderResourceNotes(resources)
	h.attachResourceUsers(c.Context(), resources)
	h.attachPatchSummaries(c.Context(), resources)
	patchModel.StripResourceSecrets(resources)

	// The comment strip comes from the community primitive's site feed, which
	// keysets on creation time rather than id — the import gives historical
	// comments fresh ids, so id order is import order.
	comments := []*commentService.FeedItem{}
	if page, appErr := h.comments.SiteFeed(c.Context(), "", homeCommentCount, cl, patchSummaryFinder{db: h.db}); appErr == nil {
		comments = page.Items
	}

	galgames := enricher.EnrichPatchCards(c.Context(), h.galgame, h.users, patches, cl)
	if len(galgames) > homeGalgameCount {
		galgames = galgames[:homeGalgameCount]
	}

	return response.OK(c, homeResponse{
		Galgames:  galgames,
		Resources: resources,
		Comments:  comments,
	})
}

type galgameListRequest struct {
	SelectedType   string `query:"selected_type" validate:"required,min=1,max=107"`
	SortField      string `query:"sort_field" validate:"required,oneof=resource_update_time created view download release_date popularity updated"`
	SortOrder      string `query:"sort_order" validate:"required,oneof=asc desc"`
	Page           int    `query:"page" validate:"required,min=1"`
	Limit          int    `query:"limit" validate:"required,min=1,max=24"`
	ReleasedFrom   string `query:"released_from"`
	ReleasedTo     string `query:"released_to"`
	ReleasedMonths string `query:"released_months"`
	Indexed        bool   `query:"indexed"`
	Library        bool   `query:"library"`
	Language       string `query:"language" validate:"omitempty,max=107"`
	Platform       string `query:"platform" validate:"omitempty,max=107"`
	CompanyID      int    `query:"company_id" validate:"omitempty,min=1"`
	TagIDs         string `query:"tag_ids" validate:"omitempty,max=107"`
}

type commentListRequest struct {
	SortField string `query:"sort_field" validate:"required,oneof=created like_count"`
	SortOrder string `query:"sort_order" validate:"required,oneof=asc desc"`
	Page      int    `query:"page" validate:"required,min=1"`
	Limit     int    `query:"limit" validate:"required,min=1,max=50"`
}

func (h *CommonHandler) attachPatchSummaries(ctx context.Context, resources []patchModel.PatchResource) {
	if len(resources) == 0 {
		return
	}

	idSet := make(map[int]struct{}, len(resources))
	for _, r := range resources {
		idSet[r.GalgameID] = struct{}{}
	}
	if len(idSet) == 0 {
		return
	}
	ids := make([]int, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}

	summaries := enricher.BuildPatchSummaryMap(ctx, h.galgame, patchSummaryFinder{db: h.db}, ids)
	for i := range resources {
		if s, ok := summaries[resources[i].GalgameID]; ok {
			summary := s
			resources[i].Patch = &summary
		}
	}
}

type resourceListRequest struct {
	SortField string `query:"sort_field" validate:"required,oneof=update_time created download like_count"`
	SortOrder string `query:"sort_order" validate:"required,oneof=asc desc"`
	Model     string `query:"model" validate:"omitempty,max=100"`
	Page      int    `query:"page" validate:"required,min=1"`
	Limit     int    `query:"limit" validate:"required,min=1,max=50"`
}

func (h *CommonHandler) GetGlobalResources(c fiber.Ctx) error {
	var req resourceListRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	cl := utils.ContentLimitForListBrowse(c)

	var resources []patchModel.PatchResource
	var total int64

	base := h.db.Model(&patchModel.PatchResource{}).Where("status = 0")
	if m := strings.TrimSpace(req.Model); m != "" {
		base = base.Where("model_name ILIKE ?", "%"+m+"%")
	}
	base.Session(&gorm.Session{}).Count(&total)

	sortField := req.SortField
	if sortField == "like" {
		sortField = "like_count"
	}

	err := base.Session(&gorm.Session{}).Order(fmt.Sprintf("patch_resource.%s %s, patch_resource.id DESC", sortField, req.SortOrder)).
		Offset((req.Page - 1) * req.Limit).Limit(req.Limit).
		Find(&resources).Error

	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	resources = enricher.FilterByGalgameContentLimit(c.Context(), h.galgame, resources, func(r patchModel.PatchResource) int { return r.GalgameID }, cl)
	patchModel.RenderResourceNotes(resources)
	h.attachResourceUsers(c.Context(), resources)
	h.attachPatchSummaries(c.Context(), resources)
	patchModel.StripResourceSecrets(resources)
	return response.Paginated(c, resources, total)
}

func (h *CommonHandler) GetResourceDetail(c fiber.Ctx) error {
	cl := utils.ContentLimitForListBrowse(c)

	// GORM's inline condition only binds a primary key when the value is numeric;
	// a non-numeric string is spliced into the WHERE clause as raw SQL. The page
	// sends `/resource/${Number(route.params.id)}`, so a bad route reached Postgres
	// as `column "nan" does not exist (SQLSTATE 42703)`.
	resourceID, idErr := strconv.Atoi(c.Params("id"))
	if idErr != nil || resourceID < 1 {
		return response.Error(c, errors.ErrBadRequest("invalid resource id"))
	}
	var resource patchModel.PatchResource
	if dbErr := h.db.First(&resource, resourceID).Error; dbErr != nil {
		return response.Error(c, errors.ErrNotFound("resource not found"))
	}
	if resource.Status == 2 {
		return response.Error(c, errors.ErrNotFound("resource not found"))
	}

	var patch patchModel.Patch
	var patchCard *enricher.GalgameCard
	if err := h.db.First(&patch, resource.GalgameID).Error; err == nil {
		patchCard = enricher.EnrichPatch(c.Context(), h.galgame, h.users, &patch, cl)
	}
	if patchCard == nil {
		return response.Error(c, errors.ErrNotFound("resource not found"))
	}

	const recTarget = 5
	var recs []patchModel.PatchResource
	h.db.Where("galgame_id = ? AND id != ? AND status = 0", resource.GalgameID, resource.ID).
		Order("download DESC, id DESC").Limit(recTarget).Find(&recs)

	if len(recs) < recTarget {
		var pool []patchModel.PatchResource
		h.db.Where("id != ? AND galgame_id != ? AND status = 0 AND download > ?",
			resource.ID, resource.GalgameID, 500).
			Limit(20).Find(&pool)
		seen := make(map[int]bool, len(recs))
		for _, r := range recs {
			seen[r.ID] = true
		}
		extras := pool[:0]
		for _, r := range pool {
			if !seen[r.ID] {
				extras = append(extras, r)
			}
		}
		rand.Shuffle(len(extras), func(i, j int) {
			extras[i], extras[j] = extras[j], extras[i]
		})
		if need := recTarget - len(recs); need > 0 && len(extras) > 0 {
			if need > len(extras) {
				need = len(extras)
			}
			recs = append(recs, extras[:need]...)
		}
	}

	recs = enricher.FilterByGalgameContentLimit(c.Context(), h.galgame, recs, func(r patchModel.PatchResource) int { return r.GalgameID }, cl)

	resource.NoteHTML = markdown.MustRender(resource.Note)
	patchModel.RenderResourceNotes(recs)

	if resource.Status != 0 {
		resource.Content = ""
		resource.S3Key = ""
		resource.Code = ""
		resource.Password = ""
	} else if resource.ArtifactUUID != "" && h.art != nil {
		if dl, derr := h.art.Download(c.Context(), resource.ArtifactUUID); derr == nil {
			resource.DownloadURL = dl.Url
		}
	}

	one := []patchModel.PatchResource{resource}
	h.attachResourceUsers(c.Context(), one)
	resource = one[0]
	h.attachResourceUsers(c.Context(), recs)
	patchModel.StripResourceSecrets(recs)

	patchFavorited := false
	if u := middleware.GetUser(c); u != nil && u.ID > 0 {
		ids := make([]int, 0, len(recs)+1)
		ids = append(ids, resource.ID)
		for i := range recs {
			ids = append(ids, recs[i].ID)
		}
		var likedIDs []int
		h.db.Model(&patchModel.UserPatchResourceLikeRelation{}).
			Where("user_id = ? AND resource_id IN ?", u.ID, ids).
			Pluck("resource_id", &likedIDs)
		likedSet := make(map[int]bool, len(likedIDs))
		for _, id := range likedIDs {
			likedSet[id] = true
		}
		resource.IsLiked = likedSet[resource.ID]
		for i := range recs {
			recs[i].IsLiked = likedSet[recs[i].ID]
		}

		patchFavorited = h.holdsPatch(c, resource.GalgameID)

		var resFavCount int64
		h.db.Model(&patchModel.UserPatchResourceFavoriteRelation{}).
			Where("user_id = ? AND resource_id = ?", u.ID, resource.ID).
			Count(&resFavCount)
		resource.IsFavorite = resFavCount > 0
	}

	return response.OK(c, map[string]any{
		"resource":          resource,
		"patch":             patchCard,
		"recommendations":   recs,
		"patch_is_favorite": patchFavorited,
	})
}

type rankingUser struct {
	ID            int                   `json:"id"`
	Name          string                `json:"name"`
	Avatar        string                `json:"avatar"`
	Cosmetics     *userclient.Cosmetics `json:"cosmetics,omitempty"`
	Moemoepoint   int                   `json:"moemoepoint"`
	PatchCount    int64                 `json:"patch_count"`
	ResourceCount int64                 `json:"resource_count"`
	CommentCount  int64                 `json:"comment_count"`
}

func (h *CommonHandler) GetUserRanking(c fiber.Ctx) error {
	sortBy := c.Query("sort_by", c.Query("sortBy", "moemoepoint"))

	const limit = 60
	type row struct {
		ID            int   `gorm:"column:id"`
		Moemoepoint   int   `gorm:"column:moemoepoint"`
		PatchCount    int64 `gorm:"column:patch_count"`
		ResourceCount int64 `gorm:"column:resource_count"`
	}

	orderBy := "u.moemoepoint DESC, u.id DESC"
	switch sortBy {
	case "patch", "patch_count":
		orderBy = "patch_count DESC, u.moemoepoint DESC, u.id DESC"
	case "resource", "resource_count":
		orderBy = "resource_count DESC, u.moemoepoint DESC, u.id DESC"
	}

	// Comments cannot be ordered in SQL — they live in the community primitive —
	// so that one sort asks the primitive who the top authors are and this query
	// only fills in the rest of their row.
	var board []int
	if sortBy == "comment" || sortBy == "comment_count" {
		if board = h.comments.AuthorBoard(c.Context(), limit); len(board) == 0 {
			return response.OK(c, []rankingUser{})
		}
	}

	q := h.db.Table(`"user" u`).
		Select(`u.id, u.moemoepoint,
			COALESCE((SELECT COUNT(*) FROM patch p WHERE p.user_id = u.id), 0) AS patch_count,
			COALESCE((SELECT COUNT(*) FROM patch_resource pr WHERE pr.user_id = u.id), 0) AS resource_count`)
	if board != nil {
		q = q.Where("u.id IN ?", board)
	} else {
		q = q.Order(orderBy).Limit(limit)
	}

	var rows []row
	if err := q.Find(&rows).Error; err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	if board != nil {
		rank := make(map[int]int, len(board))
		for i, id := range board {
			rank[id] = i
		}
		slices.SortFunc(rows, func(a, b row) int { return rank[a.ID] - rank[b.ID] })
	}

	uids := make([]int, 0, len(rows))
	for _, r := range rows {
		uids = append(uids, r.ID)
	}
	briefs := userclient.BriefMapByInt(c.Context(), h.users, uids)
	commentCounts := h.comments.AuthorCounts(c.Context(), uids)

	out := make([]rankingUser, 0, len(rows))
	for _, r := range rows {
		ru := rankingUser{
			ID:            r.ID,
			Moemoepoint:   r.Moemoepoint,
			PatchCount:    r.PatchCount,
			ResourceCount: r.ResourceCount,
			CommentCount:  commentCounts[r.ID],
		}
		if b := briefs[r.ID]; b != nil {
			if b.Status != 0 {
				continue
			}
			ru.Name = b.Name
			ru.Avatar = b.Avatar
			ru.Cosmetics = b.Cosmetics
		}
		out = append(out, ru)
	}
	return response.OK(c, out)
}

func (h *CommonHandler) GetPatchRanking(c fiber.Ctx) error {
	cl := utils.ContentLimitForListBrowse(c)
	sortBy := c.Query("sort_by", c.Query("sortBy", "view"))

	// The sort select offers comment and resource too. Without a case here they
	// fell through to the default and the page served the view ranking under the
	// other label, which reads as "the rankings are all the same".
	column := "view"
	switch sortBy {
	case "download":
		column = "download"
	case "favorite", "favorite_by", "favorite_count":
		column = "favorite_count"
	case "comment", "comment_count":
		column = "comment_count"
	case "resource", "resource_count":
		column = "resource_count"
	}

	var patches []patchModel.Patch
	q := h.db.Model(&patchModel.Patch{}).Where("status = 0")
	if !utils.IncludeEmptyGalgames(c) {
		q = q.Where("resource_count > 0")
	}
	q = utils.ScopePatchContentLimit(q, cl)
	err := q.
		Order(fmt.Sprintf("%s DESC, id DESC", column)).
		Limit(60).
		Find(&patches).Error
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.OK(c, enricher.EnrichPatches(c.Context(), h.galgame, h.users, patches, cl))
}

func (h *CommonHandler) GetMoyuHasPatch(c fiber.Ctx) error {
	var vndbIDs []string
	h.db.Model(&patchModel.Patch{}).
		Joins("JOIN patch_resource ON patch_resource.galgame_id = patch.id").
		Where("patch.vndb_id IS NOT NULL").
		Distinct("patch.vndb_id").
		Pluck("patch.vndb_id", &vndbIDs)

	return response.OK(c, vndbIDs)
}

func calendarContentLimits(cl string) []string {
	switch cl {
	case "nsfw":
		return []string{"nsfw"}
	case "all":
		return []string{"sfw", "nsfw"}
	default:
		return []string{"sfw"}
	}
}

func (h *CommonHandler) calendarPatchRows(ids []int) map[int]patchModel.Patch {
	rows := make(map[int]patchModel.Patch, len(ids))
	if len(ids) == 0 {
		return rows
	}
	var patches []patchModel.Patch
	h.db.Where("id IN ?", ids).Find(&patches)
	for _, p := range patches {
		rows[p.ID] = p
	}
	return rows
}

func (h *CommonHandler) enrichCalendarItems(c fiber.Ctx, briefs []galgameClient.GalgameBrief) []enricher.CalendarCard {
	ids := make([]int, 0, len(briefs))
	for i := range briefs {
		if briefs[i].ID > 0 {
			ids = append(ids, briefs[i].ID)
		}
	}
	cards := enricher.EnrichCalendarBriefs(briefs, h.calendarPatchRows(ids))

	if uid := middleware.GetUserID(c); uid > 0 {
		fav := h.calendarFavoriteSet(c, ids)
		for i := range cards {
			if fav[cards[i].ID] {
				cards[i].IsFavorite = true
			}
		}
	}
	return cards
}

// holdsPatch and calendarFavoriteSet both used to count
// user_patch_favorite_relation, which the 2026-09-07 cutover froze: these two
// hearts went on showing whatever was true that day while the game page beside
// them read the catalog and disagreed.
//
// The resource page asks about one game, so it asks the catalog about one work.
func (h *CommonHandler) holdsPatch(c fiber.Ctx, patchID int) bool {
	if patchID <= 0 || patchModel.IsLocalOnly(patchID) {
		return false
	}
	held, err := favorite.Holds(c.Context(), h.galgame, middleware.GetAccessToken(c), int64(patchID))
	return err == nil && held
}

// The calendar asks about a whole month of games, so it asks the catalog about
// them in one batch.
func (h *CommonHandler) calendarFavoriteSet(c fiber.Ctx, ids []int) map[int]bool {
	set := make(map[int]bool, len(ids))
	token := middleware.GetAccessToken(c)
	if token == "" || len(ids) == 0 {
		return set
	}
	works := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 && !patchModel.IsLocalOnly(id) {
			works = append(works, int64(id))
		}
	}
	held, err := favorite.HoldsAll(c.Context(), h.galgame, token, works)
	if err != nil {
		slog.Warn("calendar favorite set failed", "error", err)
		return set
	}
	for _, id := range ids {
		if held[int64(id)] {
			set[id] = true
		}
	}
	return set
}

func (h *CommonHandler) GetGalgameCalendar(c fiber.Ctx) error {
	cl := utils.ContentLimitForListBrowse(c)
	month := strings.TrimSpace(c.Query("month"))

	merged, err := h.fetchCalendarMonth(c.Context(), month, cl)
	if err != nil {
		if gerr, ok := galgameClient.AsBadRequest(err); ok {
			return response.Error(c, errors.ErrBadRequest(gerr.Message))
		}
		return response.Error(c, errors.ErrInternal("调用 Galgame 资料库失败"))
	}
	if merged == nil {
		return response.Error(c, errors.ErrInternal("调用 Galgame 资料库失败"))
	}

	return response.OK(c, fiber.Map{
		"month": merged.Month,
		"today": merged.Today,
		"items": h.enrichCalendarItems(c, merged.Items),
		"meta":  merged.Meta,
	})
}

func (h *CommonHandler) fetchCalendarMonth(ctx context.Context, month, cl string) (*galgameClient.GalgameCalendar, error) {
	var merged *galgameClient.GalgameCalendar
	for _, lim := range calendarContentLimits(cl) {
		cal, err := h.galgame.GetGalgameCalendar(ctx, month, lim)
		if err != nil {
			return nil, err
		}
		if merged == nil {
			merged = cal
			continue
		}
		merged.Items = append(merged.Items, cal.Items...)
		merged.Meta.Count += cal.Meta.Count
		merged.Meta.HasPrev = merged.Meta.HasPrev || cal.Meta.HasPrev
		merged.Meta.HasNext = merged.Meta.HasNext || cal.Meta.HasNext
		merged.Meta.MinMonth = minMonthStr(merged.Meta.MinMonth, cal.Meta.MinMonth)
		merged.Meta.MaxMonth = maxMonthStr(merged.Meta.MaxMonth, cal.Meta.MaxMonth)
	}
	return merged, nil
}

func minMonthStr(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	case b < a:
		return b
	default:
		return a
	}
}

func maxMonthStr(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	case b > a:
		return b
	default:
		return a
	}
}
