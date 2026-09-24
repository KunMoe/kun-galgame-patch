package handler

import (
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/catalogv2"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

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
	out, err := v2.MintClaim(c.Context(), token, pressKey(middleware.MustGetUser(c).ID, "catalog.mint", form.SubmitKey), fields)
	if err != nil {
		if p, ok := catalogv2.ProblemOf(err); ok && p.Code == catalogv2.CodeDuplicateSuspects {
			return response.Error(c, errors.ErrConflict(duplicateSuspectsMessage(p.Suspects)))
		}
		return catalogErr(c, err, "资料库没有接受这份投稿，请检查后重试")
	}
	return c.JSON(response.Response{
		Code: 0, Message: "OK",
		Data: fiber.Map{"id": out.WorkID(), "claim_state": out.State},
	})
}

func duplicateSuspectsMessage(suspects []catalogv2.Suspect) string {
	names := make([]string, 0, 3)
	for _, s := range suspects {
		if s.DisplayName != "" && len(names) < 3 {
			names = append(names, "《"+s.DisplayName+"》")
		}
	}
	if len(names) == 0 {
		return "资料库里已有同名作品，请先搜索确认是否重复"
	}
	return "资料库里已有同名作品：" + strings.Join(names, "、") + "，请先搜索确认是否重复"
}

func (h *PatchHandler) ClaimGalgame(c fiber.Ctx) error {
	if appErr := h.ensureCanPublishGalgame(c); appErr != nil {
		return response.Error(c, appErr)
	}
	gid, idErr := getIDParam(c, "gid")
	if idErr != nil {
		return response.Error(c, idErr.(*errors.AppError))
	}
	token, tErr := catalogUserToken(c)
	if tErr != nil {
		return response.Error(c, tErr)
	}
	if err := h.patchClaim(c, token, int64(gid), catalogv2.ClaimStateLive); err != nil {
		return catalogErr(c, err, "无法认领该游戏，请刷新后重试")
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

	// The claim is live in catalog by now, and a retry of this request answers
	// 409 there, so a failed local row is logged rather than handed back as a
	// 500 the reader cannot recover from. The page id is the work id either way.
	userID := middleware.MustGetUser(c).ID
	patchID, regErr := h.service.RegisterClaimedGalgame(userID, gid, vndbID)
	if regErr != nil {
		slog.Warn("claim: catalog claim is live but the local patch row failed",
			"gid", gid, "user_id", userID, "error", regErr)
		patchID = gid
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
	token, tErr := catalogUserToken(c)
	if tErr != nil {
		return response.Error(c, tErr)
	}
	if err := h.withdrawClaim(c, token, int64(gid)); err != nil {
		return catalogErr(c, err, "无法撤回该投稿，请刷新后重试")
	}
	return response.OKMessage(c, "OK")
}

var claimStates = []string{
	catalogv2.ClaimStateNone, catalogv2.ClaimStateLive, catalogv2.ClaimStateDraft,
	catalogv2.ClaimStatePending, catalogv2.ClaimStateDeclined, catalogv2.ClaimStateHidden,
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
			if s = strings.TrimSpace(s); s == "" {
				continue
			}
			if !slices.Contains(claimStates, s) {
				return response.Error(c, errors.ErrBadRequest("未知的投稿状态"))
			}
			states = append(states, s)
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
		return catalogErr(c, err, "")
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
		return response.Upstream(c, err, "")
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
