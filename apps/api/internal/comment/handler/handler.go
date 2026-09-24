package handler

import (
	"strconv"
	"strings"

	"kun-galgame-patch-api/internal/comment/service"
	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/middleware"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

type Handler struct {
	service *service.Service
	galgame *galgameClient.Client
	db      *gorm.DB
}

func New(svc *service.Service, galgame *galgameClient.Client, db *gorm.DB) *Handler {
	return &Handler{service: svc, galgame: galgame, db: db}
}

type wallRequest struct {
	// After is a post_number, not an opaque cursor: a comment wall keysets on
	// the thread's own numbering.
	After string `query:"after" validate:"omitempty,max=32"`
	Limit int    `query:"limit" validate:"omitempty,min=1,max=50"`
}

type createRequest struct {
	Content       string `json:"content" validate:"required,min=1,max=10007"`
	ReplyToPostID *int64 `json:"reply_to_post_id"`
	SubmitKey     string `json:"submit_key" validate:"omitempty,max=64"`
}

type updateRequest struct {
	Content string `json:"content" validate:"required,min=1,max=10007"`
	Reason  string `json:"reason"`
}

type flagRequest struct {
	Reason int32  `json:"reason" validate:"min=0,max=4"`
	Note   string `json:"note" validate:"omitempty,max=500"`
}

func (h *Handler) GetPatchComments(c fiber.Ctx) error {
	patchID, appErr := idParam(c, "id")
	if appErr != nil {
		return response.Error(c, appErr)
	}
	if !h.gate(c, patchID) {
		return response.Error(c, errors.ErrNotFound("patch not found"))
	}
	var req wallRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	page, err := h.service.Wall(c.Context(), service.PatchSurface(patchID),
		middleware.GetUserID(c), req.After, req.Limit)
	if err != nil {
		return fail(c, err, onRead)
	}
	return response.OK(c, page)
}

func (h *Handler) GetResourceComments(c fiber.Ctx) error {
	surface, appErr := h.resourceSurface(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req wallRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	page, err := h.service.Wall(c.Context(), surface, middleware.GetUserID(c), req.After, req.Limit)
	if err != nil {
		return fail(c, err, onRead)
	}
	return response.OK(c, page)
}

func (h *Handler) CreatePatchComment(c fiber.Ctx) error {
	patchID, appErr := idParam(c, "id")
	if appErr != nil {
		return response.Error(c, appErr)
	}
	if !h.gate(c, patchID) {
		return response.Error(c, errors.ErrNotFound("patch not found"))
	}
	var req createRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	user := middleware.MustGetUser(c)
	item, err := h.service.Create(c.Context(), service.PatchSurface(patchID), user.ID, req.Content, req.ReplyToPostID, req.SubmitKey)
	if err != nil {
		return fail(c, err, onCreate)
	}
	return response.OK(c, item)
}

func (h *Handler) CreateResourceComment(c fiber.Ctx) error {
	surface, appErr := h.resourceSurface(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req createRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	user := middleware.MustGetUser(c)
	item, err := h.service.Create(c.Context(), surface, user.ID, req.Content, req.ReplyToPostID, req.SubmitKey)
	if err != nil {
		return fail(c, err, onCreate)
	}
	return response.OK(c, item)
}

func (h *Handler) UpdateComment(c fiber.Ctx) error {
	postID, appErr := postIDParam(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req updateRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	user := middleware.MustGetUser(c)
	item, err := h.service.Update(c.Context(), postID, user.ID, middleware.IsModerator(c), req.Content, clampReason(req.Reason))
	if err != nil {
		return fail(c, err, onEdit)
	}
	return response.OK(c, item)
}

func (h *Handler) DeleteComment(c fiber.Ctx) error {
	postID, appErr := postIDParam(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	user := middleware.MustGetUser(c)
	if err := h.service.Delete(c.Context(), postID, user.ID, middleware.IsModerator(c), deleteReason(c)); err != nil {
		return fail(c, err, onPost)
	}
	return response.OKMessage(c, "Comment deleted")
}

func (h *Handler) LikeComment(c fiber.Ctx) error { return h.setLike(c, true) }

func (h *Handler) UnlikeComment(c fiber.Ctx) error { return h.setLike(c, false) }

func (h *Handler) setLike(c fiber.Ctx, liked bool) error {
	postID, appErr := postIDParam(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	user := middleware.MustGetUser(c)
	res, err := h.service.SetLike(c.Context(), postID, user.ID, liked)
	if err != nil {
		return fail(c, err, onPost)
	}
	return response.OK(c, res)
}

func (h *Handler) FlagComment(c fiber.Ctx) error {
	postID, appErr := postIDParam(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req flagRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	user := middleware.MustGetUser(c)
	if err := h.service.Flag(c.Context(), postID, user.ID, req.Reason, req.Note); err != nil {
		return fail(c, err, onPost)
	}
	return response.OKMessage(c, "举报已提交")
}

func (h *Handler) GetCommentMarkdown(c fiber.Ctx) error {
	postID, appErr := postIDParam(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	md, err := h.service.Markdown(c.Context(), postID)
	if err != nil {
		return fail(c, err, onPost)
	}
	return response.OK(c, fiber.Map{"markdown": md})
}

// Locate resolves a PRE-CUTOVER comment id. It is addressed by query rather
// than by path so it can never be read as a post id: the two id spaces overlap.
func (h *Handler) LocateComment(c fiber.Ctx) error {
	legacyID, err := strconv.Atoi(c.Query("legacy_id"))
	if err != nil || legacyID <= 0 {
		return response.Error(c, errors.ErrBadRequest("legacy_id is required"))
	}
	res, appErr := h.service.Locate(legacyID)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	if !h.gate(c, res.GalgameID) {
		return response.Error(c, errors.ErrNotFound("comment not found"))
	}
	return response.OK(c, res)
}

// GetGlobalComments is the site-wide newest-comments feed.
func (h *Handler) GetGlobalComments(c fiber.Ctx) error {
	var req struct {
		Cursor string `query:"cursor" validate:"omitempty,max=256"`
		Limit  int    `query:"limit" validate:"omitempty,min=1,max=50"`
	}
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	page, err := h.service.SiteFeed(c.Context(), req.Cursor, req.Limit,
		utils.ContentLimitForListBrowse(c), h.summaryDB())
	if err != nil {
		return fail(c, err, onRead)
	}
	return response.OK(c, page)
}

func (h *Handler) SearchComments(c fiber.Ctx) error {
	var req struct {
		Q      string `query:"q" validate:"required,max=100"`
		Cursor string `query:"cursor" validate:"omitempty,max=256"`
		Limit  int    `query:"limit" validate:"omitempty,min=1,max=50"`
	}
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	page, err := h.service.Search(c.Context(), req.Q, req.Cursor, req.Limit,
		utils.ContentLimitForListBrowse(c), h.summaryDB())
	if err != nil {
		return fail(c, err, onRead)
	}
	return response.OK(c, page)
}

func (h *Handler) GetUserComments(c fiber.Ctx) error {
	userID, appErr := idParam(c, "id")
	if appErr != nil {
		return response.Error(c, appErr)
	}
	// `cursor`, like every other mixed feed: useCommentFeed sends nothing else,
	// and reading `after` here served page one on every 加载更多.
	var req struct {
		Cursor string `query:"cursor" validate:"omitempty,max=32"`
		Limit  int    `query:"limit" validate:"omitempty,min=1,max=50"`
	}
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	page, err := h.service.AuthorFeed(c.Context(), userID, req.Cursor, req.Limit,
		utils.ContentLimitForListBrowse(c), h.summaryDB())
	if err != nil {
		return fail(c, err, onRead)
	}
	return response.OK(c, page)
}

func (h *Handler) summaryDB() service.PatchSummaryDB { return patchSummaryFinder{db: h.db} }

// gate keeps a wall behind the same content limit the page it belongs to is
// behind: a comment read must not be the way an NSFW game leaks.
func (h *Handler) gate(c fiber.Ctx, patchID int) bool {
	cl := utils.ContentLimitForListBrowse(c)
	if cl == "" || h.galgame == nil {
		return true
	}
	briefs, err := h.galgame.GalgameBatch(c.Context(), []int{patchID}, cl)
	if err != nil {
		return false
	}
	return len(briefs) > 0
}

func (h *Handler) resourceSurface(c fiber.Ctx) (service.Surface, *errors.AppError) {
	resourceID, appErr := idParam(c, "resourceId")
	if appErr != nil {
		return service.Surface{}, appErr
	}
	ref, err := h.service.ResourceRef(resourceID)
	if err != nil || ref.Status == 2 {
		return service.Surface{}, errors.ErrNotFound("resource not found")
	}
	if !h.gate(c, ref.GalgameID) {
		return service.Surface{}, errors.ErrNotFound("resource not found")
	}
	return service.ResourceSurface(resourceID, ref.GalgameID), nil
}

func idParam(c fiber.Ctx, name string) (int, *errors.AppError) {
	id, err := strconv.Atoi(c.Params(name))
	if err != nil || id <= 0 {
		return 0, errors.ErrBadRequest("Invalid id")
	}
	return id, nil
}

func postIDParam(c fiber.Ctx) (int64, *errors.AppError) {
	id, err := strconv.ParseInt(c.Params("postId"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.ErrBadRequest("Invalid comment id")
	}
	return id, nil
}

func clampReason(r string) string {
	r = strings.TrimSpace(r)
	if rs := []rune(r); len(rs) > 500 {
		r = string(rs[:500])
	}
	return r
}

func deleteReason(c fiber.Ctx) string {
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.Bind().Body(&body)
	return clampReason(body.Reason)
}

// patchSummaryFinder is the vndb-id lookup the feed rows' game names are built
// from. Same shape as the one in internal/common: the summary builder takes the
// interface, not the table.
type patchSummaryFinder struct{ db *gorm.DB }

func (p patchSummaryFinder) LookupPatchesByIDs(ids []int) ([]patchModel.Patch, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []patchModel.Patch
	err := p.db.Select("id", "vndb_id").Where("id IN ?", ids).Find(&rows).Error
	return rows, err
}
