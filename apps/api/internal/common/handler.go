package common

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"kun-galgame-patch-api/internal/favorite"
	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/galgame/enricher"
	"kun-galgame-patch-api/internal/infrastructure/markdown"
	"kun-galgame-patch-api/internal/middleware"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/artifactclient"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/imageclient"
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
	img     *imageclient.Client
}

func NewHandler(db *gorm.DB, galgame *galgameClient.Client, users *userclient.Client, art *artifactclient.Client, img *imageclient.Client) *CommonHandler {
	return &CommonHandler{db: db, galgame: galgame, users: users, art: art, img: img}
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
			rs[i].User = &patchModel.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar, AvatarImageHash: b.AvatarImageHash, Roles: b.Roles, SiteRoles: b.SiteRoles}
		}
	}
}

func (h *CommonHandler) attachCommentUsers(ctx context.Context, cs []patchModel.PatchComment) {
	if len(cs) == 0 {
		return
	}
	uids := make([]int, 0, len(cs))
	for _, c := range cs {
		uids = append(uids, c.UserID)
	}
	briefs := userclient.BriefMapByInt(ctx, h.users, uids)
	for i := range cs {
		if b := briefs[cs[i].UserID]; b != nil {
			cs[i].User = &patchModel.PatchUser{ID: int(b.ID), Name: b.Name, Avatar: b.Avatar, AvatarImageHash: b.AvatarImageHash, Roles: b.Roles, SiteRoles: b.SiteRoles}
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
	Comments  []patchModel.PatchComment  `json:"comments"`
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

func (h *CommonHandler) GetHome(c fiber.Ctx) error {
	cl := utils.ContentLimitForListBrowse(c)

	var patches []patchModel.Patch
	var resources []patchModel.PatchResource
	var comments []patchModel.PatchComment

	patchQuery := h.db.Model(&patchModel.Patch{}).Order("created DESC, id DESC").
		Limit(homeGalgameCount * homeGalgameOverfetch)
	if !utils.IncludeEmptyGalgames(c) {
		patchQuery = patchQuery.Where("resource_count > 0")
	}
	patchQuery = utils.ScopePatchContentLimit(patchQuery, cl)
	patchQuery.Find(&patches)
	h.db.Model(&patchModel.PatchResource{}).Where("status = 0").Order("created DESC, id DESC").Limit(6).Find(&resources)
	h.db.Model(&patchModel.PatchComment{}).Where("status = 0").Order("created DESC, id DESC").Limit(6).Find(&comments)

	resources = enricher.FilterByGalgameContentLimit(c.Context(), h.galgame, resources, func(r patchModel.PatchResource) int { return r.GalgameID }, cl)
	comments = enricher.FilterByGalgameContentLimit(c.Context(), h.galgame, comments, func(m patchModel.PatchComment) int { return m.GalgameID }, cl)

	patchModel.RenderResourceNotes(resources)
	for i := range comments {
		comments[i].ContentHTML = markdown.MustRender(comments[i].Content)
	}
	h.attachResourceUsers(c.Context(), resources)
	h.attachCommentUsers(c.Context(), comments)
	h.attachPatchSummaries(c.Context(), comments, resources)
	patchModel.StripResourceSecrets(resources)

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

func (h *CommonHandler) GetGlobalComments(c fiber.Ctx) error {
	var req commentListRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	cl := utils.ContentLimitForListBrowse(c)

	var comments []patchModel.PatchComment
	var total int64

	base := h.db.Model(&patchModel.PatchComment{}).Where("status = 0")
	base.Session(&gorm.Session{}).Count(&total)

	err := base.Session(&gorm.Session{}).Order(fmt.Sprintf("%s %s, id DESC", req.SortField, req.SortOrder)).
		Offset((req.Page - 1) * req.Limit).Limit(req.Limit).
		Find(&comments).Error

	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	comments = enricher.FilterByGalgameContentLimit(c.Context(), h.galgame, comments, func(m patchModel.PatchComment) int { return m.GalgameID }, cl)

	for i := range comments {
		comments[i].ContentHTML = markdown.MustRender(comments[i].Content)
	}
	h.attachCommentUsers(c.Context(), comments)
	h.attachPatchSummaries(c.Context(), comments, nil)
	return response.Paginated(c, comments, total)
}

func (h *CommonHandler) attachPatchSummaries(ctx context.Context, comments []patchModel.PatchComment, resources []patchModel.PatchResource) {
	if len(comments) == 0 && len(resources) == 0 {
		return
	}

	idSet := make(map[int]struct{}, len(comments)+len(resources))
	for _, m := range comments {
		idSet[m.GalgameID] = struct{}{}
	}
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
	for i := range comments {
		if s, ok := summaries[comments[i].GalgameID]; ok {
			summary := s
			comments[i].Patch = &summary
		}
	}
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
	h.attachPatchSummaries(c.Context(), nil, resources)
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

type hikariEnvelope struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type hikariUser struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

type hikariResource struct {
	ID         int                  `json:"id"`
	Storage    string               `json:"storage"`
	Name       string               `json:"name"`
	ModelName  string               `json:"model_name"`
	Size       string               `json:"size"`
	Note       string               `json:"note"`
	Hash       string               `json:"hash"`
	Type       patchModel.JSONArray `json:"type"`
	Language   patchModel.JSONArray `json:"language"`
	Platform   patchModel.JSONArray `json:"platform"`
	Download   int                  `json:"download"`
	Status     int                  `json:"status"`
	UpdateTime time.Time            `json:"update_time"`
	UserID     int                  `json:"user_id"`
	PatchID    int                  `json:"patch_id"`
	Created    time.Time            `json:"created"`
	User       hikariUser           `json:"user"`
}

type hikariPatch struct {
	ID                 int                  `json:"id"`
	VndbID             string               `json:"vndb_id"`
	Released           string               `json:"released"`
	Status             int                  `json:"status"`
	Download           int                  `json:"download"`
	View               int                  `json:"view"`
	ResourceUpdateTime time.Time            `json:"resource_update_time"`
	Type               patchModel.JSONArray `json:"type"`
	Language           patchModel.JSONArray `json:"language"`
	Platform           patchModel.JSONArray `json:"platform"`
	UserID             int                  `json:"user_id"`
	Created            time.Time            `json:"created"`
	Updated            time.Time            `json:"updated"`
	User               hikariUser           `json:"user"`
	Resource           []hikariResource     `json:"resource"`
}

func hikariFail(c fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(hikariEnvelope{Success: false, Message: message, Data: nil})
}

func (h *CommonHandler) hikariAvatarURL(b *userclient.Brief) string {
	if b == nil {
		return ""
	}
	if b.AvatarImageHash != "" && h.img != nil {
		if u := h.img.MainURL(b.AvatarImageHash); u != "" {
			return u
		}
	}
	return b.Avatar
}

func (h *CommonHandler) GetHikari(c fiber.Ctx) error {
	vndbID := c.Query("vndb_id")
	if vndbID == "" {
		return hikariFail(c, fiber.StatusBadRequest, "Missing required parameter: vndb_id")
	}

	var patch patchModel.Patch
	if err := h.db.Where("vndb_id = ?", vndbID).First(&patch).Error; err != nil {
		return hikariFail(c, fiber.StatusNotFound, "No patch found for VNDB ID: "+vndbID)
	}

	var resources []patchModel.PatchResource
	h.db.Where("galgame_id = ? AND status = 0", patch.ID).Find(&resources)

	uids := make([]int, 0, len(resources)+1)
	uids = append(uids, patch.UserID)
	for i := range resources {
		uids = append(uids, resources[i].UserID)
	}
	briefs := userclient.BriefMapByInt(c.Context(), h.users, uids)
	toUser := func(uid int) hikariUser {
		if b := briefs[uid]; b != nil {
			return hikariUser{ID: int(b.ID), Name: b.Name, Avatar: h.hikariAvatarURL(b)}
		}
		return hikariUser{ID: uid}
	}

	out := make([]hikariResource, 0, len(resources))
	for i := range resources {
		r := &resources[i]
		out = append(out, hikariResource{
			ID:         r.ID,
			Storage:    r.Storage,
			Name:       r.Name,
			ModelName:  r.ModelName,
			Size:       r.Size,
			Note:       markdown.ResolveContentImageTokens(r.Note),
			Hash:       r.Blake3,
			Type:       r.Type,
			Language:   r.Language,
			Platform:   r.Platform,
			Download:   r.Download,
			Status:     r.Status,
			UpdateTime: r.UpdateTime,
			UserID:     r.UserID,
			PatchID:    r.GalgameID,
			Created:    r.Created,
			User:       toUser(r.UserID),
		})
	}

	released := ""
	if patch.ReleaseDate != nil {
		released = patch.ReleaseDate.Format("2006-01-02")
	}

	return c.JSON(hikariEnvelope{
		Success: true,
		Message: "Patch found successfully",
		Data: hikariPatch{
			ID:                 patch.ID,
			VndbID:             patch.VndbID,
			Released:           released,
			Status:             patch.Status,
			Download:           patch.Download,
			View:               patch.View,
			ResourceUpdateTime: patch.ResourceUpdateTime,
			Type:               patch.Type,
			Language:           patch.Language,
			Platform:           patch.Platform,
			UserID:             patch.UserID,
			Created:            patch.Created,
			Updated:            patch.Updated,
			User:               toUser(patch.UserID),
			Resource:           out,
		},
	})
}

type rankingUser struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Avatar        string `json:"avatar"`
	Moemoepoint   int    `json:"moemoepoint"`
	PatchCount    int64  `json:"patch_count"`
	ResourceCount int64  `json:"resource_count"`
	CommentCount  int64  `json:"comment_count"`
}

func (h *CommonHandler) GetUserRanking(c fiber.Ctx) error {
	sortBy := c.Query("sort_by", c.Query("sortBy", "moemoepoint"))

	const limit = 60
	type row struct {
		ID            int   `gorm:"column:id"`
		Moemoepoint   int   `gorm:"column:moemoepoint"`
		PatchCount    int64 `gorm:"column:patch_count"`
		ResourceCount int64 `gorm:"column:resource_count"`
		CommentCount  int64 `gorm:"column:comment_count"`
	}

	orderBy := "u.moemoepoint DESC, u.id DESC"
	switch sortBy {
	case "patch", "patch_count":
		orderBy = "patch_count DESC, u.moemoepoint DESC, u.id DESC"
	case "resource", "resource_count":
		orderBy = "resource_count DESC, u.moemoepoint DESC, u.id DESC"
	case "comment", "comment_count":
		orderBy = "comment_count DESC, u.moemoepoint DESC, u.id DESC"
	}

	var rows []row
	err := h.db.Table(`"user" u`).
		Select(`u.id, u.moemoepoint,
			COALESCE((SELECT COUNT(*) FROM patch p WHERE p.user_id = u.id), 0) AS patch_count,
			COALESCE((SELECT COUNT(*) FROM patch_resource pr WHERE pr.user_id = u.id), 0) AS resource_count,
			COALESCE((SELECT COUNT(*) FROM patch_comment pc WHERE pc.user_id = u.id), 0) AS comment_count`).
		Order(orderBy).
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	uids := make([]int, 0, len(rows))
	for _, r := range rows {
		uids = append(uids, r.ID)
	}
	briefs := userclient.BriefMapByInt(c.Context(), h.users, uids)

	out := make([]rankingUser, 0, len(rows))
	for _, r := range rows {
		ru := rankingUser{
			ID:            r.ID,
			Moemoepoint:   r.Moemoepoint,
			PatchCount:    r.PatchCount,
			ResourceCount: r.ResourceCount,
			CommentCount:  r.CommentCount,
		}
		if b := briefs[r.ID]; b != nil {
			if b.Status != 0 {
				continue
			}
			ru.Name = b.Name
			ru.Avatar = b.Avatar
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
	byPatch, err := utils.WorkIDsByPatchIDs(h.db, []int{patchID})
	if err != nil {
		return false
	}
	held, err := favorite.Holds(c.Context(), h.galgame, middleware.GetAccessToken(c), byPatch[patchID])
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
	byPatch, err := utils.WorkIDsByPatchIDs(h.db, ids)
	if err != nil {
		return set
	}
	works := make([]int64, 0, len(byPatch))
	for _, workID := range byPatch {
		works = append(works, workID)
	}
	held, err := favorite.HoldsAll(c.Context(), h.galgame, token, works)
	if err != nil {
		slog.Warn("calendar favorite set failed", "error", err)
		return set
	}
	for id, workID := range byPatch {
		if held[workID] {
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
