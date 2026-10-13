package rating

import (
	"context"
	stderrors "errors"
	"slices"
	"strconv"
	"unicode/utf8"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/catalogv2"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/upstream"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

const (
	summaryMaxRunes = 2000
	minutesMax      = 60000
	listLimit       = 10
)

var (
	recordStates  = []string{Wish, Doing, Done, DoneOneRoute, DoneMain, DoneAll, OnHold, Dropped}
	recommends    = []string{"strong_no", "no", "neutral", "yes", "strong_yes"}
	spoilerLevels = []string{"none", "portion", "serious"}
)

type Galgames interface {
	GalgameBatch(ctx context.Context, ids []int, contentLimit string) ([]galgameClient.GalgameBrief, error)
}

type Handler struct {
	service *Service
	galgame Galgames
}

func NewHandler(service *Service, galgame Galgames) *Handler {
	return &Handler{service: service, galgame: galgame}
}

type listQuery struct {
	Page  int `query:"page" validate:"omitempty,min=1,max=1000"`
	Limit int `query:"limit" validate:"omitempty,min=1,max=50"`
}

type ratingRequest struct {
	Overall      int                      `json:"overall"`
	Recommend    string                   `json:"recommend"`
	SpoilerLevel string                   `json:"spoiler_level"`
	ShortSummary string                   `json:"short_summary"`
	AspectScores *catalogv2.RatingAspects `json:"aspect_scores"`
}

type recordRequest struct {
	PlayState string         `json:"play_state"`
	Minutes   int            `json:"minutes"`
	Rating    *ratingRequest `json:"rating"`
}

func workIDParam(c fiber.Ctx) (int64, *errors.AppError) {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.ErrBadRequest("无效的游戏 ID")
	}
	return id, nil
}

// Catalog folds the token's site roles into its own, so a site-only moyu
// moderator passes catalog.rating.moderate: as IsModerator this handed 39 of
// production's 41 a delete over every site's ratings. Only admin and ren are
// staff here, the forum's rating.delete_any.
func viewerOf(c fiber.Ctx) Viewer {
	return Viewer{ID: middleware.GetUserID(c), Admin: middleware.IsAdmin(c)}
}

// visible answers the work page's own question: a reader whose content limit
// hides the work does not see its ratings either.
func (h *Handler) visible(c fiber.Ctx, workID int64) bool {
	cl := utils.ContentLimitForListBrowse(c)
	if cl == "" || h.galgame == nil {
		return true
	}
	briefs, err := h.galgame.GalgameBatch(c.Context(), []int{int(workID)}, cl)
	return err == nil && len(briefs) > 0
}

func (h *Handler) List(c fiber.Ctx) error {
	workID, appErr := workIDParam(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	if !h.visible(c, workID) {
		return response.Error(c, errors.ErrNotFound("游戏不存在"))
	}
	var q listQuery
	if err := utils.ParseQueryAndValidate(c, &q); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	page, limit := max(q.Page, 1), q.Limit
	if limit == 0 {
		limit = listLimit
	}
	items, total, err := h.service.List(c.Context(), workID, viewerOf(c), (page-1)*limit, limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, items, total)
}

func (h *Handler) GetRecord(c fiber.Ctx) error {
	workID, appErr := workIDParam(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	rec, err := h.service.Record(c.Context(), middleware.GetAccessToken(c), workID)
	if err != nil {
		return fail(c, err)
	}
	return response.OK(c, rec)
}

func (h *Handler) PutRecord(c fiber.Ctx) error {
	workID, appErr := workIDParam(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req recordRequest
	if err := c.Bind().Body(&req); err != nil {
		return response.Error(c, errors.ErrBadRequest("请求参数错误"))
	}
	in, appErr := recordInputOf(req)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	rec, err := h.service.PutRecord(c.Context(), middleware.GetAccessToken(c), middleware.MustGetUser(c).ID, workID, in)
	if err != nil {
		return fail(c, err)
	}
	return response.OK(c, rec)
}

func (h *Handler) Delete(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("ratingId"), 10, 64)
	if err != nil || id <= 0 {
		return response.Error(c, errors.ErrBadRequest("无效的评分 ID"))
	}
	if err := h.service.Delete(c.Context(), middleware.GetAccessToken(c), viewerOf(c), id); err != nil {
		return fail(c, err)
	}
	return response.OKMessage(c, "已删除评分")
}

func recordInputOf(req recordRequest) (RecordInput, *errors.AppError) {
	in := RecordInput{PlayState: req.PlayState, Minutes: req.Minutes}
	if in.PlayState != "" && !slices.Contains(recordStates, in.PlayState) {
		return in, errors.ErrValidation("未知的游玩状态")
	}
	if in.Minutes < 0 || in.Minutes > minutesMax {
		return in, errors.ErrValidation("游玩时长超出范围")
	}
	r := req.Rating
	if r == nil {
		return in, nil
	}
	if r.Overall < 1 || r.Overall > 10 {
		return in, errors.ErrValidation("总体评分需在 1 到 10 之间")
	}
	if !slices.Contains(recommends, r.Recommend) {
		return in, errors.ErrValidation("未知的推荐程度")
	}
	if r.SpoilerLevel == "" {
		r.SpoilerLevel = "none"
	}
	if !slices.Contains(spoilerLevels, r.SpoilerLevel) {
		return in, errors.ErrValidation("未知的剧透程度")
	}
	if !utf8.ValidString(r.ShortSummary) || utf8.RuneCountInString(r.ShortSummary) > summaryMaxRunes {
		return in, errors.ErrValidation("短评最多 2000 字")
	}
	if a := r.AspectScores; a != nil {
		for _, v := range []*int{a.Art, a.Story, a.Music, a.Character, a.Route, a.System, a.Voice, a.ReplayValue} {
			if v != nil && (*v < 1 || *v > 10) {
				return in, errors.ErrValidation("分项评分需在 1 到 10 之间")
			}
		}
	}
	in.Rating = &catalogv2.RatingInput{
		Overall: r.Overall, AspectScores: r.AspectScores, Recommend: r.Recommend,
		SpoilerLevel: r.SpoilerLevel, ShortSummary: r.ShortSummary,
	}
	return in, nil
}

func fail(c fiber.Ctx, err error) error {
	switch {
	case stderrors.Is(err, ErrNotFound):
		return response.Error(c, errors.ErrNotFound("评分不存在或已被删除"))
	case stderrors.Is(err, ErrNotAuthor):
		return response.Error(c, errors.ErrForbidden())
	case stderrors.Is(err, ErrContentRejected):
		return response.Error(c, errors.ErrValidation("短评未通过内容审核，请修改后再提交"))
	case stderrors.Is(err, catalogv2.ErrNoAccessToken):
		return response.Error(c, errors.ErrUnauthorized())
	case catalogv2.ReauthRequired(err):
		return response.Error(c, errors.ErrCatalogReauthRequired("资料库拒绝了当前登录凭证，请退出登录后重新登录一次"))
	}
	if e, ok := upstream.As(err); ok && e.Code == "PERMISSION_REQUIRED" {
		return response.Error(c, errors.New(40300,
			"无权限：删除他人的评分需要 NextMoe 资料库的评分管理权限", fiber.StatusForbidden))
	}
	if upstream.KindOf(err) == upstream.NotFound {
		return response.Error(c, errors.ErrNotFound("资料库里没有这个游戏"))
	}
	return response.Upstream(c, err, "")
}
