package common

import (
	"log/slog"
	"strconv"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/galgame/enricher"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

// The works an entity is attached to are a card each, and a card carries the
// attachment's own facts beside the galgame's: which roles this name had here,
// who voiced this character in this release.
type entityWorkCard struct {
	enricher.GalgameCard
	RosterRole string                           `json:"roster_role,omitempty"`
	Spoiler    int                              `json:"spoiler,omitempty"`
	Voices     []galgameClient.GalgamePersonRef `json:"voices,omitempty"`
	Roles      []galgameClient.GalgameStaffRole `json:"roles,omitempty"`
}

type characterDetailResponse struct {
	*galgameClient.GalgameCharacterDetail
	Works      []entityWorkCard `json:"works"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

type staffDetailResponse struct {
	*galgameClient.GalgameStaffDetail
	Works      []entityWorkCard `json:"works"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

type entityWorksResponse struct {
	Works      []entityWorkCard `json:"works"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

func (h *CommonHandler) GetGalgameCharacter(c fiber.Ctx) error {
	id, err := entityID(c)
	if err != nil {
		return response.Error(c, err)
	}
	cl := utils.ContentLimitForListBrowse(c)
	detail, cerr := h.galgame.GetCharacter(c.Context(), id, cl)
	if cerr != nil {
		return catalogEntityError(c, cerr, "角色")
	}
	page, werr := h.galgame.CharacterWorks(c.Context(), id, cl, "", entityWorksLimit(c))
	if werr != nil {
		slog.Warn("角色登场作品加载失败", "error", werr, "character", id)
		return response.OK(c, characterDetailResponse{GalgameCharacterDetail: detail, Works: []entityWorkCard{}})
	}
	works, next := h.entityWorkCards(page)
	return response.OK(c, characterDetailResponse{
		GalgameCharacterDetail: detail, Works: works, NextCursor: next,
	})
}

func (h *CommonHandler) GetGalgameCharacterWorks(c fiber.Ctx) error {
	id, err := entityID(c)
	if err != nil {
		return response.Error(c, err)
	}
	page, cerr := h.galgame.CharacterWorks(
		c.Context(), id, utils.ContentLimitForListBrowse(c), c.Query("cursor"), entityWorksLimit(c),
	)
	if cerr != nil {
		return catalogEntityError(c, cerr, "角色")
	}
	works, next := h.entityWorkCards(page)
	return response.OK(c, entityWorksResponse{Works: works, NextCursor: next})
}

func (h *CommonHandler) GetGalgameStaff(c fiber.Ctx) error {
	id, err := entityID(c)
	if err != nil {
		return response.Error(c, err)
	}
	cl := utils.ContentLimitForListBrowse(c)
	detail, cerr := h.galgame.GetStaff(c.Context(), id)
	if cerr != nil {
		return catalogEntityError(c, cerr, "制作人员")
	}
	page, werr := h.galgame.StaffWorks(c.Context(), id, cl, "", entityWorksLimit(c))
	if werr != nil {
		slog.Warn("制作人员参与作品加载失败", "error", werr, "name", id)
		return response.OK(c, staffDetailResponse{GalgameStaffDetail: detail, Works: []entityWorkCard{}})
	}
	works, next := h.entityWorkCards(page)
	return response.OK(c, staffDetailResponse{
		GalgameStaffDetail: detail, Works: works, NextCursor: next,
	})
}

func (h *CommonHandler) GetGalgameStaffWorks(c fiber.Ctx) error {
	id, err := entityID(c)
	if err != nil {
		return response.Error(c, err)
	}
	page, cerr := h.galgame.StaffWorks(
		c.Context(), id, utils.ContentLimitForListBrowse(c), c.Query("cursor"), entityWorksLimit(c),
	)
	if cerr != nil {
		return catalogEntityError(c, cerr, "制作人员")
	}
	works, next := h.entityWorkCards(page)
	return response.OK(c, entityWorksResponse{Works: works, NextCursor: next})
}

func entityWorksLimit(c fiber.Ctx) int {
	return fiber.Query(c, "limit", 0)
}

func (h *CommonHandler) entityWorkCards(page *galgameClient.GalgameEntityWorkPage) ([]entityWorkCard, string) {
	out := make([]entityWorkCard, 0, len(page.Items))
	ids := make([]int, 0, len(page.Items))
	for i := range page.Items {
		ids = append(ids, page.Items[i].Galgame.ID)
	}
	var rows []patchModel.Patch
	if len(ids) > 0 {
		h.db.Where("id IN ?", ids).Find(&rows)
	}
	local := make(map[int]*patchModel.Patch, len(rows))
	for i := range rows {
		local[rows[i].ID] = &rows[i]
	}
	for i := range page.Items {
		w := &page.Items[i]
		out = append(out, entityWorkCard{
			GalgameCard: enricher.CardFromBriefAndPatch(&w.Galgame, local[w.Galgame.ID]),
			RosterRole:  w.RosterRole,
			Spoiler:     w.Spoiler,
			Voices:      w.Voices,
			Roles:       w.Roles,
		})
	}
	return out, page.NextCursor
}

func entityID(c fiber.Ctx) (int, *errors.AppError) {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return 0, errors.ErrBadRequest("无效的 ID")
	}
	return id, nil
}

func catalogEntityError(c fiber.Ctx, err error, subject string) error {
	if _, ok := galgameClient.MovedTarget(err); ok {
		return response.Error(c, errors.ErrNotFound(subject+"已被合并"))
	}
	return response.Upstream(c, err, subject+"不存在")
}
