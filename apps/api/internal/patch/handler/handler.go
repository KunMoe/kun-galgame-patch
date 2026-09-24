package handler

import (
	stderrors "errors"
	"log/slog"
	"net/http"
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
	"kun-galgame-patch-api/pkg/catalogv2"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/userclient"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

var vndbIDRegex = regexp.MustCompile(`^v\d+$`)

type PatchHandler struct {
	service    *service.PatchService
	galgame    *galgameClient.Client
	users      *userclient.Client
	storeLinks *storelink.Resolver
}

func New(
	svc *service.PatchService,
	galgame *galgameClient.Client,
	users *userclient.Client,
	storeLinks *storelink.Resolver,
) *PatchHandler {
	return &PatchHandler{service: svc, galgame: galgame, users: users, storeLinks: storeLinks}
}

func catalogUserToken(c fiber.Ctx) (string, *errors.AppError) {
	token := middleware.GetAccessToken(c)
	if token == "" {
		return "", errors.ErrUnauthorized()
	}
	return token, nil
}

func catalogErr(c fiber.Ctx, err error, fallback string) error {
	if stderrors.Is(err, catalogv2.ErrNotConfigured) {
		return response.Error(c, errors.ErrInternal("资料库客户端未配置"))
	}
	if stderrors.Is(err, service.ErrNoCatalogWork) {
		return response.Error(c, errors.ErrValidation(
			"这个游戏还没有收录进资料库，暂时无法收藏或加入收藏夹"))
	}
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return response.Error(c, errors.ErrNotFound("patch not found"))
	}
	if stderrors.Is(err, catalogv2.ErrNoAccessToken) {
		return response.Error(c, errors.ErrUnauthorized())
	}
	if stderrors.Is(err, catalogv2.ErrUnauthorized) {
		return response.Error(c, errors.ErrCatalogReauthRequired(
			"资料库拒绝了当前登录凭证，请退出登录后重新登录一次"))
	}
	if stderrors.Is(err, catalogv2.ErrNotFound) {
		return response.Error(c, errors.ErrNotFound("资料库中没有这个条目"))
	}
	if stderrors.Is(err, catalogv2.ErrForbidden) {
		return response.Error(c, errors.New(40300,
			"你没有权限修改该条目（编辑资料需要相应的社区权限）", fiber.StatusForbidden))
	}
	var p *catalogv2.Problem
	if stderrors.As(err, &p) {
		switch {
		case p.Code == "SCOPE_REQUIRED" || p.Status == http.StatusUnauthorized:
			return response.Error(c, errors.ErrCatalogReauthRequired(""))
		case p.Status == http.StatusForbidden:
			return response.Error(c, errors.New(40300, p.Error(), fiber.StatusForbidden))
		case p.Status == http.StatusNotFound:
			return response.Error(c, errors.ErrNotFound("资料库中没有这个条目"))
		case p.Status == http.StatusUnprocessableEntity:
			return response.Error(c, errors.ErrValidation(p.Error()))
		case p.Status == http.StatusConflict, p.Status == http.StatusPreconditionFailed,
			p.Status == http.StatusPreconditionRequired:
			return response.Error(c, errors.ErrConflict(p.Error()))
		}
	}
	return response.Error(c, errors.ErrInternal(fallback))
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
	IsFavorite bool `json:"is_favorite"`
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

	card := h.withPurchaseLinks(headerCard{GalgameCard: *enriched})
	if user := middleware.GetUser(c); user != nil {
		card.IsFavorite = h.service.IsFavoritedInCatalog(c.Context(), middleware.GetAccessToken(c), id)
	}
	return response.OK(c, card)
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

	h.claimOnFirstResource(c, patchID)
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
		return catalogErr(c, err, "收藏失败，请稍后重试")
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

func (h *PatchHandler) SubmitGalgame(c fiber.Ctx) error {
	if appErr := h.ensureCanPublishGalgame(c); appErr != nil {
		return response.Error(c, appErr)
	}
	v2 := h.catalogV2()
	if v2 == nil || !v2.Configured() {
		return response.Error(c, errors.ErrInternal("资料库客户端未配置"))
	}
	var form SubmissionForm
	if err := c.Bind().Body(&form); err != nil {
		return response.Error(c, errors.ErrBadRequest("无法解析请求体"))
	}
	fields, fErr := form.SubmissionFields()
	if fErr != nil {
		return response.Error(c, errors.ErrBadRequest(fErr.Error()))
	}
	token, tErr := catalogUserToken(c)
	if tErr != nil {
		return response.Error(c, tErr)
	}
	out, err := v2.MintClaim(c.Context(), token, fields)
	if err != nil {
		return catalogErr(c, err, "提交到资料库失败")
	}
	return c.JSON(response.Response{
		Code: 0, Message: "OK",
		Data: fiber.Map{"id": out.WorkID(), "claim_state": out.State},
	})
}

func (h *PatchHandler) ClaimGalgame(c fiber.Ctx) error {
	if appErr := h.ensureCanPublishGalgame(c); appErr != nil {
		return response.Error(c, appErr)
	}
	gid, idErr := getIDParam(c, "gid")
	if idErr != nil {
		return response.Error(c, idErr.(*errors.AppError))
	}
	workID := int64(gid)
	token, tErr := catalogUserToken(c)
	if tErr != nil {
		return response.Error(c, tErr)
	}
	if err := h.patchClaim(c, token, workID, catalogv2.ClaimStateLive); err != nil {
		return catalogErr(c, err, "调用资料库失败")
	}

	vndbID := ""
	if briefs, bErr := h.galgame.GalgameBatch(c.Context(), []int{gid}, ""); bErr == nil {
		for i := range briefs {
			if briefs[i].ID == gid {
				vndbID = briefs[i].VndbID
				break
			}
		}
	}

	userID := middleware.MustGetUser(c).ID
	patchID, regErr := h.service.RegisterClaimedGalgame(userID, gid, vndbID)
	if regErr != nil {
		return response.Error(c, errors.ErrInternal("认领成功，但本站登记失败，请稍后重试"))
	}

	return c.JSON(response.Response{
		Code:    0,
		Message: "OK",
		Data:    fiber.Map{"id": patchID},
	})
}

func (h *PatchHandler) WithdrawGalgameSubmission(c fiber.Ctx) error {
	gid, idErr := getIDParam(c, "gid")
	if idErr != nil {
		return response.Error(c, idErr.(*errors.AppError))
	}
	workID := int64(gid)
	token, tErr := catalogUserToken(c)
	if tErr != nil {
		return response.Error(c, tErr)
	}
	if err := h.withdrawClaim(c, token, workID); err != nil {
		return catalogErr(c, err, "调用资料库失败")
	}
	return response.OKMessage(c, "OK")
}

func (h *PatchHandler) ListMyGalgames(c fiber.Ctx) error {
	v2 := h.catalogV2()
	if v2 == nil || !v2.Configured() {
		return response.Error(c, errors.ErrInternal("资料库客户端未配置"))
	}
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if limit < 1 || limit > 50 {
		limit = 20
	}

	states := mySubmissionStates
	if raw := strings.TrimSpace(c.Query("claim_state", "")); raw != "" {
		states = nil
		for _, s := range strings.Split(raw, ",") {
			if s = strings.TrimSpace(s); s != "" {
				states = append(states, s)
			}
		}
	}
	token, tErr := catalogUserToken(c)
	if tErr != nil {
		return response.Error(c, tErr)
	}
	page, err := v2.MyClaims(c.Context(), token, catalogv2.MyClaimsQuery{
		ClaimStates: states, Site: catalogv2.SiteKungal,
		Cursor: c.Query("cursor", ""), Limit: limit,
	})
	if err != nil {
		return catalogErr(c, err, "调用资料库失败")
	}
	items := make([]mySubmission, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, submissionRow(&page.Items[i]))
	}
	return response.OK(c, fiber.Map{
		"items": items, "next_cursor": page.Next(), "total": page.Count(),
	})
}

var mySubmissionStates = []string{
	catalogv2.ClaimStatePending,
	catalogv2.ClaimStateDeclined,
}

type mySubmission struct {
	WorkID        int64   `json:"work_id"`
	DisplayName   string  `json:"display_name"`
	ClaimState    string  `json:"claim_state"`
	ProductWorkID *int64  `json:"product_work_id"`
	LastReason    *string `json:"last_reason"`
	FirstActedAt  string  `json:"first_acted_at"`
}

func submissionRow(r *catalogv2.ClaimRecord) mySubmission {
	row := mySubmission{
		WorkID: r.WorkID(), DisplayName: r.DisplayName, ClaimState: r.State,
		ProductWorkID: r.ProductID(), LastReason: r.LastReason(),
	}
	if r.FirstActedAt != nil {
		row.FirstActedAt = *r.FirstActedAt
	}
	return row
}

type wizardPendingHit struct {
	ID          int    `json:"id"`
	DisplayName string `json:"display_name"`
	ClaimState  string `json:"claim_state"`
	Reason      string `json:"reason,omitempty"`
}

func (h *PatchHandler) SearchGalgameForPublish(c fiber.Ctx) error {
	q := c.Query("q", "")
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	if limit < 1 || limit > 24 {
		limit = 10
	}
	items, total, err := h.galgame.SearchPublishItems(c.Context(), q, limit)
	if err != nil {
		if werr, ok := err.(*galgameClient.GalgameError); ok {
			return response.Error(c, errors.New(werr.Code, werr.Message, fiber.StatusBadRequest))
		}
		return response.Error(c, errors.ErrInternal("调用 Galgame 资料库失败"))
	}
	pending := make([]wizardPendingHit, 0)
	if v2 := h.catalogV2(); v2 != nil && v2.Configured() {
		pending = append(pending, h.ownPendingSubmissions(c, q)...)
	}
	return response.OK(c, fiber.Map{"items": items, "pending": pending, "total": total})
}

func (h *PatchHandler) ownPendingSubmissions(c fiber.Ctx, q string) []wizardPendingHit {
	token := middleware.GetAccessToken(c)
	if token == "" {
		return nil
	}
	page, err := h.catalogV2().MyClaims(c.Context(), token, catalogv2.MyClaimsQuery{
		ClaimStates: mySubmissionStates, Site: catalogv2.SiteKungal, Limit: 50,
	})
	if err != nil {
		slog.Warn("读取本人投稿列表失败，向导仅显示公开结果", "error", err)
		return nil
	}
	needle := strings.ToLower(strings.TrimSpace(q))
	out := make([]wizardPendingHit, 0, len(page.Items))
	for i := range page.Items {
		it := submissionRow(&page.Items[i])
		if needle != "" && !strings.Contains(strings.ToLower(it.DisplayName), needle) {
			continue
		}
		hit := wizardPendingHit{
			ID: int(it.WorkID), DisplayName: it.DisplayName, ClaimState: it.ClaimState,
		}
		if it.ProductWorkID != nil && *it.ProductWorkID > 0 {
			hit.ID = int(*it.ProductWorkID)
		}
		if it.LastReason != nil {
			hit.Reason = *it.LastReason
		}
		out = append(out, hit)
	}
	return out
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
