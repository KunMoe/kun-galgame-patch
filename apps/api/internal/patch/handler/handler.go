package handler

import (
	stderrors "errors"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/galgame/enricher"
	"kun-galgame-patch-api/internal/infrastructure/storelink"
	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/internal/patch/dto"
	"kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/internal/patch/service"
	"kun-galgame-patch-api/internal/usercache"
	"kun-galgame-patch-api/pkg/catalogv2"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/upstream"
	"kun-galgame-patch-api/pkg/userclient"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

var vndbIDRegex = regexp.MustCompile(`^v\d+$`)

type PatchHandler struct {
	service    *service.PatchService
	galgame    *galgameClient.Client
	mine       *usercache.Cache
	users      *userclient.Client
	storeLinks *storelink.Resolver
}

func New(
	svc *service.PatchService,
	galgame *galgameClient.Client,
	mine *usercache.Cache,
	users *userclient.Client,
	storeLinks *storelink.Resolver,
) *PatchHandler {
	return &PatchHandler{service: svc, galgame: galgame, mine: mine, users: users, storeLinks: storeLinks}
}

func catalogUserToken(c fiber.Ctx) (string, *errors.AppError) {
	token := middleware.GetAccessToken(c)
	if token == "" {
		return "", errors.ErrUnauthorized()
	}
	return token, nil
}

func catalogErr(c fiber.Ctx, err error, readerMsg string) error {
	switch {
	case stderrors.Is(err, service.ErrNoCatalogWork):
		return response.Error(c, errors.ErrValidation(
			"这个游戏还没有收录进资料库，暂时无法收藏或加入收藏夹"))
	case stderrors.Is(err, service.ErrFolderNotYours):
		return response.Error(c, errors.ErrNotFound("收藏夹不存在或已被删除，请刷新后重试"))
	case stderrors.Is(err, gorm.ErrRecordNotFound):
		return response.Error(c, errors.ErrNotFound("patch not found"))
	case stderrors.Is(err, catalogv2.ErrNoAccessToken):
		return response.Error(c, errors.ErrUnauthorized())
	case catalogv2.ReauthRequired(err):
		return response.Error(c, catalogReauth(err))
	}
	return response.Upstream(c, err, readerMsg)
}

// pressKey is the Idempotency-Key for one press of a create button: the page
// mints submit_key per press and keeps it across that press's retries. Without
// one nothing is sent, which only costs the retry safety.
func pressKey(userID int, op, submitKey string) string {
	if submitKey == "" {
		return ""
	}
	return upstream.IdempotencyKey(strconv.Itoa(userID), op, submitKey)
}

func catalogReauth(err error) *errors.AppError {
	if e, _ := upstream.As(err); e != nil && e.Code == catalogv2.CodeInvalidCredential {
		return errors.ErrCatalogReauthRequired("资料库拒绝了当前登录凭证，请退出登录后重新登录一次")
	}
	return errors.ErrCatalogReauthRequired("")
}

func getIDParam(c fiber.Ctx, name string) (int, error) {
	id, err := strconv.Atoi(c.Params(name))
	if err != nil || id < 1 {
		return 0, errors.ErrBadRequest("invalid ID")
	}
	return id, nil
}

func (h *PatchHandler) gatePatchByContentLimit(c fiber.Ctx, patchID int) bool {
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

func (h *PatchHandler) ensureCanPublishGalgame(c fiber.Ctx) *errors.AppError {
	if h.service.IsCreatorOnlyEnabled() && !middleware.IsModerator(c) && !middleware.HasRole(c, "creator") {
		return errors.New(40300, "本站当前仅允许创作者 / 版主 / 管理员发布 Galgame", fiber.StatusForbidden)
	}
	return nil
}

func (h *PatchHandler) CreatePatch(c fiber.Ctx) error {
	if appErr := h.ensureCanPublishGalgame(c); appErr != nil {
		return response.Error(c, appErr)
	}
	user := middleware.MustGetUser(c)

	var req dto.PatchCreateRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if req.GalgameID <= 0 {
		return response.Error(c, errors.ErrBadRequest("请提供 galgame_id"))
	}

	id, err := h.service.CreatePatchByGalgameID(c.Context(), user.ID, req.GalgameID)
	if err != nil {
		if stderrors.Is(err, service.ErrGalgameMissing) {
			return response.Error(c, errors.ErrGalgameNotFound(""))
		}
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OK(c, map[string]int{"id": id})
}

type headerCard struct {
	enricher.GalgameCard
	// The DLsite purchase entry. Absent whenever the work has no buyable workno
	// or the feature is unconfigured, and the button is not rendered at all.
	DlsitePurchaseURL  string `json:"dlsite_purchase_url,omitempty"`
	DlsiteCouponURL    string `json:"dlsite_coupon_url,omitempty"`
	DlsiteCampaignName string `json:"dlsite_campaign_name,omitempty"`
}

func (h *PatchHandler) withPurchaseLinks(card headerCard) headerCard {
	if card.Galgame == nil {
		return card
	}
	links := h.storeLinks.Resolve(card.Galgame.DlsiteWorkno)
	card.DlsitePurchaseURL = links.PurchaseURL
	card.DlsiteCouponURL = links.CouponURL
	card.DlsiteCampaignName = links.CampaignName
	return card
}

func (h *PatchHandler) GetPatch(c fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	cl := utils.ContentLimitForListBrowse(c)
	patch, err := h.service.GetPatch(c.Context(), id)
	if err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			if card := enricher.GalgameOnlyCard(c.Context(), h.galgame, h.users, id, cl); card != nil {
				return response.OK(c, h.withPurchaseLinks(headerCard{GalgameCard: *card}))
			}
		}
		return h.patchGone(c, id)
	}

	enriched := enricher.EnrichPatch(c.Context(), h.galgame, h.users, patch, cl)
	if enriched == nil {
		return h.patchGone(c, id)
	}

	return response.OK(c, h.withPurchaseLinks(headerCard{GalgameCard: *enriched}))
}

func (h *PatchHandler) GetPatchDetail(c fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	cl := utils.ContentLimitForListBrowse(c)
	patch, err := h.service.GetPatchDetail(c.Context(), id)
	if err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			if detail := enricher.GalgameOnlyDetail(c.Context(), h.galgame, h.users, id, cl); detail != nil {
				return response.OK(c, detail)
			}
		}
		return response.Error(c, errors.ErrNotFound("patch not found"))
	}
	enriched := enricher.EnrichPatchDetail(c.Context(), h.galgame, h.users, patch, cl)
	if enriched == nil {
		return response.Error(c, errors.ErrNotFound("patch not found"))
	}
	return response.OK(c, enriched)
}

func (h *PatchHandler) UpdatePatch(c fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.PatchUpdateRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if !vndbIDRegex.MatchString(req.VndbID) {
		return response.Error(c, errors.ErrBadRequest("vndb_id 格式不合法"))
	}

	user := middleware.MustGetUser(c)
	isPrivileged := middleware.IsModerator(c)
	if err := h.service.UpdatePatch(c.Context(), id, user.ID, isPrivileged, req.VndbID); err != nil {
		if _, ok := upstream.As(err); ok {
			return response.Upstream(c, err, "")
		}
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	return response.OKMessage(c, "Patch updated")
}

func (h *PatchHandler) DeletePatch(c fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	user := middleware.MustGetUser(c)
	isAdmin := middleware.IsAdmin(c)
	if err := h.service.DeletePatch(id, user.ID, isAdmin); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	return response.OKMessage(c, "Patch deleted")
}

func (h *PatchHandler) IncrementView(c fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	h.service.IncrementView(id)
	return response.OKMessage(c, "OK")
}

func (h *PatchHandler) GetResources(c fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	if !h.gatePatchByContentLimit(c, id) {
		return response.Error(c, errors.ErrNotFound("patch not found"))
	}

	currentUID := middleware.GetUserID(c)
	resources, err := h.service.GetResources(c.Context(), id, currentUID)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	return response.OK(c, resources)
}

func (h *PatchHandler) CreateResource(c fiber.Ctx) error {
	patchID, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.PatchResourceCreateRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if msg := validateResourceVocab(&req); msg != "" {
		return response.Error(c, errors.ErrBadRequest(msg))
	}

	user := middleware.MustGetUser(c)
	resource := &model.PatchResource{
		GalgameID:    patchID,
		Storage:      req.Storage,
		Name:         req.Name,
		ModelName:    req.ModelName,
		ArtifactUUID: req.ArtifactUUID,
		S3Key:        req.S3Key,
		Content:      req.Content,
		Size:         req.Size,
		Code:         req.Code,
		Password:     req.Password,
		Note:         req.Note,
		Type:         model.JSONArray(req.Type),
		Language:     model.JSONArray(req.Language),
		Platform:     model.JSONArray(req.Platform),
	}

	if err := h.service.CreateResource(c.Context(), resource, user.ID); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	h.adoptUnlessLive(c, patchID)
	return response.OK(c, resource)
}

func (h *PatchHandler) UpdateResource(c fiber.Ctx) error {
	resourceID, err := getIDParam(c, "resourceId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	var req dto.PatchResourceUpdateRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	if msg := validateResourceVocab(&req.PatchResourceCreateRequest); msg != "" {
		return response.Error(c, errors.ErrBadRequest(msg))
	}

	user := middleware.MustGetUser(c)
	update := &model.PatchResource{
		Storage:      req.Storage,
		Name:         req.Name,
		ModelName:    req.ModelName,
		ArtifactUUID: req.ArtifactUUID,
		S3Key:        req.S3Key,
		Content:      req.Content,
		Size:         req.Size,
		Code:         req.Code,
		Password:     req.Password,
		Note:         req.Note,
		Type:         model.JSONArray(req.Type),
		Language:     model.JSONArray(req.Language),
		Platform:     model.JSONArray(req.Platform),
	}

	actorRole := 1
	if middleware.IsAdmin(c) {
		actorRole = 3
	} else if middleware.HasRole(c, "moderator") {
		actorRole = 2
	}

	updated, err := h.service.UpdateResource(c.Context(), resourceID, user.ID, update, req.Reason, actorRole)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	return response.OK(c, updated)
}

func (h *PatchHandler) DeleteResource(c fiber.Ctx) error {
	resourceID, err := getIDParam(c, "resourceId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	user := middleware.MustGetUser(c)
	isPrivileged := middleware.IsModerator(c)
	if err := h.service.DeleteResource(resourceID, user.ID, isPrivileged, parseDeleteReason(c)); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	return response.OKMessage(c, "Resource deleted")
}

func (h *PatchHandler) ToggleResourceDisable(c fiber.Ctx) error {
	resourceID, err := getIDParam(c, "resourceId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	user := middleware.MustGetUser(c)
	isPrivileged := middleware.IsModerator(c)
	status, err := h.service.ToggleResourceDisable(resourceID, user.ID, isPrivileged)
	if err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	return response.OK(c, fiber.Map{"status": status})
}

func (h *PatchHandler) GetResourceDownloadInfo(c fiber.Ctx) error {
	resourceID, err := getIDParam(c, "resourceId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	r, gErr := h.service.GetResourceDownloadInfo(resourceID)
	if gErr != nil {
		return response.Error(c, errors.ErrNotFound("resource not found"))
	}
	if !h.gatePatchByContentLimit(c, r.GalgameID) {
		return response.Error(c, errors.ErrNotFound("resource not found"))
	}
	if r.Status == 2 {
		return response.Error(c, errors.ErrNotFound("resource not found"))
	}
	if r.Status != 0 {
		return response.Error(c, errors.New(40310, "该资源已被禁用，暂时无法下载", fiber.StatusForbidden))
	}
	if err := h.service.ResolveDownloadURL(c.Context(), r); err != nil {
		return response.Upstream(c, err, "资源文件不存在或已被删除")
	}
	return response.OK(c, fiber.Map{
		"storage":      r.Storage,
		"content":      r.Content,
		"download_url": r.DownloadURL,
		"code":         r.Code,
		"password":     r.Password,
	})
}

func (h *PatchHandler) IncrementResourceDownload(c fiber.Ctx) error {
	resourceID, err := getIDParam(c, "resourceId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	if err := h.service.IncrementResourceDownload(resourceID); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	return response.OKMessage(c, "OK")
}

func (h *PatchHandler) ToggleResourceLike(c fiber.Ctx) error {
	resourceID, err := getIDParam(c, "resourceId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	user := middleware.MustGetUser(c)
	liked, err := h.service.ToggleResourceLike(resourceID, user.ID)
	if stderrors.Is(err, service.ErrResourceNotFound) {
		return response.Error(c, errors.ErrNotFound("资源不存在"))
	}
	if err != nil {
		slog.Error("ToggleResourceLike failed", "resourceID", resourceID, "error", err)
		return response.Error(c, errors.ErrInternal("点赞失败，请稍后重试"))
	}

	return response.OK(c, map[string]bool{"liked": liked})
}

func (h *PatchHandler) ToggleResourceFavorite(c fiber.Ctx) error {
	resourceID, err := getIDParam(c, "resourceId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	user := middleware.MustGetUser(c)
	favorited, err := h.service.ToggleResourceFavorite(resourceID, user.ID)
	if err != nil {
		slog.Error("ToggleResourceFavorite failed", "resourceID", resourceID, "error", err)
		return response.Error(c, errors.ErrInternal("收藏失败，请稍后重试"))
	}

	return response.OK(c, map[string]bool{"favorited": favorited})
}

func (h *PatchHandler) ToggleFavorite(c fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}

	user := middleware.MustGetUser(c)
	token, appErr := catalogUserToken(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	favorited, err := h.service.ToggleFavoriteInCatalog(c.Context(), token, id, user.ID)
	if err != nil {
		return catalogErr(c, err, "收藏失败，请刷新后重试")
	}

	return response.OK(c, map[string]bool{"favorited": favorited})
}

// GetFavorite is the heart on a game page, asked once the page is in the
// browser rather than inside GET /patch/:id: that read runs on every
// navigation, server-side included, and whatever it asks with the reader's
// token spends their catalog allowance on the forum too.
func (h *PatchHandler) GetFavorite(c fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	user := middleware.MustGetUser(c)
	favorited, fErr := h.service.IsFavoritedInCatalog(c.Context(), user.ID, middleware.GetAccessToken(c), id)
	if fErr != nil {
		slog.Warn("patch favorite: shelf unreadable, heart drawn empty",
			"patch_id", id, "user_id", user.ID, "error", fErr)
	}
	return response.OK(c, map[string]bool{"favorited": favorited})
}

func (h *PatchHandler) GetContributors(c fiber.Ctx) error {
	id, err := getIDParam(c, "id")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	if !h.gatePatchByContentLimit(c, id) {
		return response.Error(c, errors.ErrNotFound("patch not found"))
	}

	ids, err := h.service.GetContributorIDs(id)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}

	briefs := userclient.BriefMapByInt(c.Context(), h.users, ids)
	out := make([]model.PatchUser, 0, len(ids))
	for _, userID := range ids {
		if b := briefs[userID]; b != nil {
			out = append(out, *model.NewPatchUser(b))
		}
	}
	return response.OK(c, out)
}

func (h *PatchHandler) GetRandomPatch(c fiber.Ctx) error {
	id, err := h.service.GetRandomPatchID(c.Context(), utils.ContentLimitForListBrowse(c), utils.IncludeEmptyGalgames(c))
	if err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return response.Error(c, errors.ErrNotFound("no patch available"))
		}
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.OK(c, map[string]int{"id": id})
}

func (h *PatchHandler) GetResourceRevisions(c fiber.Ctx) error {
	resourceID, err := getIDParam(c, "resourceId")
	if err != nil {
		return response.Error(c, err.(*errors.AppError))
	}
	var req dto.ResourceFileHistoryRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	items, total, gErr := h.service.GetResourceRevisions(resourceID, req.Page, req.Limit)
	if gErr != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.Paginated(c, items, total)
}

func parseDeleteReason(c fiber.Ctx) string {
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.Bind().Body(&body)
	r := strings.TrimSpace(body.Reason)
	if rs := []rune(r); len(rs) > 500 {
		r = string(rs[:500])
	}
	return r
}
