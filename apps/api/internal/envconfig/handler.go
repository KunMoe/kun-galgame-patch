package envconfig

import (
	"log/slog"
	"regexp"
	"time"

	"kun-galgame-patch-api/internal/middleware"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/userclient"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

type Handler struct {
	store *Store
	users *userclient.Client
}

func NewHandler(store *Store, users *userclient.Client) *Handler {
	return &Handler{store: store, users: users}
}

var variableName = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,127}$`)

func errStoreOff() *errors.AppError {
	return errors.New(50300, "moyu-api 启动时没有 "+config.KEKEnv+", 配置中心未开启", fiber.StatusServiceUnavailable)
}

func (h *Handler) variable(c fiber.Ctx) (string, *errors.AppError) {
	name := c.Params("variable")
	if !variableName.MatchString(name) {
		return "", errors.ErrBadRequest("变量名只能是大写字母、数字与下划线")
	}
	return name, nil
}

func (h *Handler) Get(c fiber.Ctx) error {
	snap, err := h.store.Snapshot(c.Context())
	if err != nil {
		slog.Error("read config store", "error", err)
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.OK(c, snap)
}

type putRequest struct {
	Value      string `json:"value" validate:"required,max=4096"`
	ChangeNote string `json:"change_note" validate:"max=200"`
}

func (h *Handler) Put(c fiber.Ctx) error {
	name, appErr := h.variable(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	if !config.Storable(name) {
		return response.Error(c, errors.ErrBadRequest("启动接线与拓扑变量留在 compose 文件里, 不能存入配置中心"))
	}
	if !h.store.Enabled() {
		return response.Error(c, errStoreOff())
	}
	var req putRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	uid := middleware.MustGetUser(c).ID
	if _, err := h.store.Set(c.Context(), name, req.Value, req.ChangeNote, ActionSet, &uid); err != nil {
		slog.Error("write config store", "variable", name, "error", err)
		return response.Error(c, errors.ErrInternal(""))
	}
	v, err := h.store.Variable(c.Context(), name)
	if err != nil {
		return response.Error(c, errors.ErrInternal(""))
	}
	return response.OK(c, v)
}

func (h *Handler) Delete(c fiber.Ctx) error {
	name, appErr := h.variable(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	if !h.store.Enabled() {
		return response.Error(c, errStoreOff())
	}
	note := c.Query("note")
	if len([]rune(note)) > 200 {
		return response.Error(c, errors.ErrBadRequest("备注最多 200 字"))
	}
	uid := middleware.MustGetUser(c).ID
	deleted, err := h.store.Delete(c.Context(), name, note, &uid)
	if err != nil {
		slog.Error("delete from config store", "variable", name, "error", err)
		return response.Error(c, errors.ErrInternal(""))
	}
	if !deleted {
		return response.Error(c, errors.ErrNotFound("配置中心没有这个变量的存储值"))
	}
	return response.OKMessage(c, "已删除存储值")
}

type changesRequest struct {
	Page  int `query:"page" validate:"required,min=1"`
	Limit int `query:"limit" validate:"required,min=1,max=100"`
}

type Change struct {
	ID          int64                 `json:"id"`
	Variable    string                `json:"variable"`
	ChangeKind  string                `json:"change_kind"`
	ShownBefore string                `json:"shown_before"`
	ShownAfter  string                `json:"shown_after"`
	ChangeNote  string                `json:"change_note"`
	ChangedBy   *patchModel.PatchUser `json:"changed_by"`
	CreatedAt   time.Time             `json:"created_at"`
}

func (h *Handler) Changes(c fiber.Ctx) error {
	var req changesRequest
	if err := utils.ParseQueryAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}
	rows, total, err := h.store.Changes(c.Context(), (req.Page-1)*req.Limit, req.Limit)
	if err != nil {
		slog.Error("list config store changes", "error", err)
		return response.Error(c, errors.ErrInternal(""))
	}
	ids := make([]int, 0, len(rows))
	for _, r := range rows {
		if r.UserID != nil {
			ids = append(ids, *r.UserID)
		}
	}
	briefs := userclient.BriefMapByInt(c.Context(), h.users, ids)
	items := make([]Change, 0, len(rows))
	for _, r := range rows {
		change := Change{
			ID: r.ID, Variable: r.Key, ChangeKind: r.Action, ShownBefore: r.OldShown,
			ShownAfter: r.NewShown, ChangeNote: r.Note, CreatedAt: r.Created,
		}
		if r.UserID != nil {
			change.ChangedBy = patchModel.NewPatchUser(briefs[*r.UserID])
			if change.ChangedBy == nil {
				change.ChangedBy = &patchModel.PatchUser{ID: *r.UserID}
			}
		}
		items = append(items, change)
	}
	return response.Paginated(c, items, total)
}
