package handler

import (
	stderrors "errors"
	"log/slog"
	"slices"
	"strconv"

	"kun-galgame-patch-api/internal/middleware"
	"kun-galgame-patch-api/pkg/catalogv2"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/upstream"

	"github.com/gofiber/fiber/v3"
)

// The catalog edit schema registers far more than these four (covers,
// screenshots, credits, the `.suppressed` companions …). Widening moyu's edit
// surface is a product decision, so the exposed set is this list and nothing
// else — it filters what the bootstrap hands the page, and the named request
// below is what keeps anything else out of the patch.
var catalogEditFieldKeys = []string{
	fieldWorkDisplayName,
	fieldWorkOLang,
	fieldWorkContentRating,
	fieldWorkTitles,
}

const (
	catalogEditNoteMax       = 2000
	catalogEditProposalLimit = 20
)

// The shape the edit page reads; catalog's schema_field is the source it is
// projected from.
type editSchemaField struct {
	Key            string `json:"key"`
	Kind           string `json:"kind"`
	DiffHint       string `json:"diff_hint"`
	Deprecated     bool   `json:"deprecated,omitempty"`
	Locked         bool   `json:"locked"`
	CanPropose     bool   `json:"can_propose"`
	CanReview      bool   `json:"can_review"`
	WouldAutomerge bool   `json:"would_automerge"`
	MaxSuppressed  int    `json:"max_suppressed,omitempty"`
	MaxElements    int    `json:"max_elements,omitempty"`
}

type catalogEditTitle struct {
	Lang  string `json:"lang"`
	Title string `json:"title"`
	Latin string `json:"latin"`
	Kind  int16  `json:"kind"`
}

type catalogEditValues struct {
	DisplayName   string             `json:"display_name"`
	OLang         string             `json:"olang"`
	ContentRating int16              `json:"content_rating"`
	Titles        []catalogEditTitle `json:"titles"`
}

type catalogEditRequest struct {
	DisplayName   *string             `json:"display_name"`
	OLang         *string             `json:"olang"`
	ContentRating *int16              `json:"content_rating"`
	Titles        *[]catalogEditTitle `json:"titles"`
	Note          string              `json:"note"`
	SubmitKey     string              `json:"submit_key"`
}

// The shape @nextmoe/edit-ui-core's parseEditProblem reads. The envelope's
// Message is one string, so a rejection that names its fields has to travel in
// Data or the page can only toast the engine's prose and leave the reader to
// work out which field — which title row — it is about.
func problemFields(p *catalogv2.Problem) fiber.Map {
	if len(p.Errors) == 0 && p.Detail == "" {
		return nil
	}
	return fiber.Map{"errors": p.Errors, "detail": p.Detail}
}

// Only a refusal that names its fields carries catalog's problem to the page.
// Until 2026-09-24 every 403 did, so SITE_NOT_BOUND reached readers as English
// prose, and a scope moyu's own key lacked told them to sign in again.
func catalogEditErr(c fiber.Ctx, err error) error {
	switch {
	case stderrors.Is(err, catalogv2.ErrNotConfigured):
		return response.Error(c, errors.ErrCatalogUnavailable(""))
	case stderrors.Is(err, catalogv2.ErrNoAccessToken):
		return response.Error(c, errors.ErrUnauthorized())
	case catalogv2.ReauthRequired(err):
		return response.Error(c, catalogReauth(err))
	}

	switch upstream.KindOf(err) {
	case upstream.NotFound:
		return response.Upstream(c, err, "资料库中没有这个条目")
	case upstream.Conflict:
		return response.Upstream(c, err, "条目已被他人修改，或该提案已经关闭，请刷新后重试")
	case upstream.Rejected:
		p, _ := catalogv2.ProblemOf(err)
		if p != nil && p.Status == fiber.StatusUnprocessableEntity {
			return response.ErrorData(c, errors.ErrValidation(p.Error()), problemFields(p))
		}
		const denied = "你没有权限修改该条目（编辑资料需要相应的社区权限）"
		if p != nil && p.Status == fiber.StatusForbidden && len(p.Errors) > 0 {
			return response.ErrorData(c, errors.New(40300, denied, fiber.StatusForbidden), fiber.Map{"errors": p.Errors})
		}
		if p != nil && p.Status == fiber.StatusForbidden {
			return response.Upstream(c, err, denied)
		}
		return response.Upstream(c, err, "资料库没有接受这次修改")
	}
	return response.Upstream(c, err, "")
}

func (h *PatchHandler) catalogV2() *catalogv2.Client {
	if h.galgame == nil {
		return nil
	}
	return h.galgame.V2()
}

func (h *PatchHandler) catalogEditContext(c fiber.Ctx) (int64, string, error) {
	if h.catalogV2() == nil || !h.catalogV2().Configured() {
		return 0, "", response.Error(c, errors.ErrCatalogUnavailable(""))
	}
	id, idErr := getIDParam(c, "id")
	if idErr != nil {
		return 0, "", response.Error(c, idErr.(*errors.AppError))
	}
	workID := int64(id)
	token, tErr := catalogUserToken(c)
	if tErr != nil {
		return 0, "", response.Error(c, tErr)
	}
	return workID, token, nil
}

func catalogEditSnapshotValues(values map[string]any) catalogEditValues {
	out := catalogEditValues{Titles: []catalogEditTitle{}}
	if s, ok := values[fieldWorkDisplayName].(string); ok {
		out.DisplayName = s
	}
	if s, ok := values[fieldWorkOLang].(string); ok {
		out.OLang = s
	}
	if n, ok := values[fieldWorkContentRating].(float64); ok {
		out.ContentRating = int16(n)
	}
	rows, _ := values[fieldWorkTitles].([]any)
	for _, el := range rows {
		row, ok := el.(map[string]any)
		if !ok {
			continue
		}
		title := catalogEditTitle{}
		title.Lang, _ = row["lang"].(string)
		title.Title, _ = row["title"].(string)
		title.Latin, _ = row["latin"].(string)
		if n, ok := row["kind"].(float64); ok {
			title.Kind = int16(n)
		}
		out.Titles = append(out.Titles, title)
	}
	return out
}

func (h *PatchHandler) CatalogEditBootstrap(c fiber.Ctx) error {
	workID, token, done := h.catalogEditContext(c)
	if done != nil {
		return done
	}

	v2 := h.catalogV2()
	schema, err := v2.GetSchema(c.Context(), "work")
	if err != nil {
		return catalogEditErr(c, err)
	}
	snapshot, err := v2.Snapshot(c.Context(), token, "work", workID)
	if err != nil {
		return catalogEditErr(c, err)
	}

	fields := make([]editSchemaField, 0, len(catalogEditFieldKeys))
	canEdit := false
	for _, f := range schema.Fields {
		if !slices.Contains(catalogEditFieldKeys, f.Key) {
			continue
		}
		row := editSchemaField{
			Key: f.Key, Kind: f.FieldType, DiffHint: f.DiffHint,
			Deprecated: f.Deprecated, MaxSuppressed: f.MaxSuppressed, MaxElements: f.MaxElements,
			CanPropose: !f.Deprecated, Locked: f.Deprecated,
		}
		fields = append(fields, row)
		if row.CanPropose && !row.Locked {
			canEdit = true
		}
	}

	return response.OK(c, fiber.Map{
		"work_id":  workID,
		"can_edit": canEdit,
		"values":   catalogEditSnapshotValues(snapshot.FieldValues),
		"fields":   fields,
	})
}

func (h *PatchHandler) CatalogEditSubmit(c fiber.Ctx) error {
	workID, token, done := h.catalogEditContext(c)
	if done != nil {
		return done
	}

	var req catalogEditRequest
	if err := c.Bind().Body(&req); err != nil {
		return response.Error(c, errors.ErrBadRequest("无法解析请求体"))
	}
	if len([]rune(req.Note)) > catalogEditNoteMax {
		return response.Error(c, errors.ErrValidation("修改说明过长"))
	}

	patch := map[string]any{}
	if req.DisplayName != nil {
		patch[fieldWorkDisplayName] = *req.DisplayName
	}
	if req.OLang != nil {
		patch[fieldWorkOLang] = *req.OLang
	}
	if req.ContentRating != nil {
		patch[fieldWorkContentRating] = *req.ContentRating
	}
	if req.Titles != nil {
		titles := make([]any, 0, len(*req.Titles))
		for _, t := range *req.Titles {
			el := map[string]any{"lang": t.Lang, "title": t.Title, "kind": t.Kind}
			if t.Latin != "" {
				el["latin"] = t.Latin
			}
			titles = append(titles, el)
		}
		patch[fieldWorkTitles] = titles
	}
	if len(patch) == 0 {
		return response.Error(c, errors.ErrValidation("没有需要保存的修改"))
	}

	result, err := h.catalogV2().CreateProposal(c.Context(), token,
		pressKey(middleware.MustGetUser(c).ID, "catalog.proposal."+strconv.FormatInt(workID, 10), req.SubmitKey),
		catalogv2.EntityTypeWork, workID, patch, req.Note)
	if err != nil {
		return catalogEditErr(c, err)
	}
	return response.OK(c, fiber.Map{
		"merged":   result.State == "merged",
		"proposal": proposalView(result),
	})
}

func (h *PatchHandler) CatalogEditProposals(c fiber.Ctx) error {
	workID, token, done := h.catalogEditContext(c)
	if done != nil {
		return done
	}
	page, err := h.catalogV2().ListMyProposals(c.Context(), token, workID, catalogEditProposalLimit)
	if err != nil {
		return catalogEditErr(c, err)
	}
	items := make([]fiber.Map, 0, len(page.Items))
	for i := range page.Items {
		items = append(items, proposalView(h.hydrateProposalPatch(c, token, &page.Items[i])))
	}
	return response.OK(c, fiber.Map{"items": items})
}

// The list face carries no patch — its spec advertises no include at all — so
// "你改了哪些字段" needs the detail face, one call per row. Bounded by the page
// limit, and the list is already narrowed to a single work, so this is a handful
// of proposals at most. A row whose detail read fails keeps its listed shape
// rather than sinking the whole list.
func (h *PatchHandler) hydrateProposalPatch(c fiber.Ctx, token string, p *catalogv2.ProposalRecord) *catalogv2.ProposalRecord {
	id, ok := catalogv2.ParseID(p.ID)
	if !ok || len(p.Patch) > 0 {
		return p
	}
	full, _, err := h.catalogV2().GetProposal(c.Context(), token, id)
	if err != nil {
		slog.Warn("catalog edit: proposal patch not hydrated", "id", p.ID, "error", err)
		return p
	}
	return full
}

func proposalView(p *catalogv2.ProposalRecord) fiber.Map {
	id, _ := catalogv2.ParseID(p.ID)
	entityID, _ := catalogv2.ParseID(p.EntityID)
	proposerUID, _ := catalogv2.ParseID(p.ProposerUID)
	view := fiber.Map{
		"id": id, "status": p.State, "state": p.State, "note": p.Note,
		"entity_type": p.EntityType, "entity_id": entityID, "patch": p.Patch,
		"effective_patch": p.EffectivePatch, "proposer_uid": proposerUID,
		"site": p.Site, "base_revision_seq": p.BaseRevisionSeq,
		"created_at": p.CreatedAt, "updated_at": p.UpdatedAt,
	}
	if p.DecidedByUID != nil {
		decidedBy, _ := catalogv2.ParseID(*p.DecidedByUID)
		view["decided_by_uid"] = decidedBy
	}
	if p.DecidedAt != nil {
		view["decided_at"] = *p.DecidedAt
	}
	return view
}

func (h *PatchHandler) CatalogProposalWithdraw(c fiber.Ctx) error {
	if h.catalogV2() == nil || !h.catalogV2().Configured() {
		return response.Error(c, errors.ErrCatalogUnavailable(""))
	}
	id, idErr := getIDParam(c, "id")
	if idErr != nil {
		return response.Error(c, idErr.(*errors.AppError))
	}
	token, tErr := catalogUserToken(c)
	if tErr != nil {
		return response.Error(c, tErr)
	}
	prop, err := h.catalogV2().WithdrawProposal(c.Context(), token, int64(id))
	if err != nil {
		return catalogEditErr(c, err)
	}
	return response.OK(c, proposalView(prop))
}
