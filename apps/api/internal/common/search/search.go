// Package search implements POST /api/search: delegate full-text search to the
// Galgame Wiki, then look up which patches exist locally for the returned vndb_ids.
//
// Design (D11, 2026-04-21):
//   - Search/retrieval is fully delegated to Wiki (60k galgame + Meilisearch + CJK tokenization)
//   - This service only answers "do these galgames have patches here"
//   - No local index, no local sync
package search

import (
	"log/slog"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Handler handles /api/search requests.
type Handler struct {
	db   *gorm.DB
	wiki *galgameClient.Client
}

// New constructs a Handler.
func New(db *gorm.DB, wiki *galgameClient.Client) *Handler {
	return &Handler{db: db, wiki: wiki}
}

// SearchRequest is the search request body.
// It supports most of Wiki's filter parameters, passed through directly.
type SearchRequest struct {
	Q            string `json:"q" validate:"max=200"`
	TagIDs       []int  `json:"tag_ids" validate:"omitempty,max=20,dive,min=1"`
	OfficialIDs  []int  `json:"official_ids" validate:"omitempty,max=20,dive,min=1"`
	EngineIDs    []int  `json:"engine_ids" validate:"omitempty,max=20,dive,min=1"`
	OriginalLang string `json:"original_language" validate:"max=100"`
	AgeLimit     string `json:"age_limit" validate:"omitempty,oneof=all r18"`
	ReleasedFrom int    `json:"released_from" validate:"omitempty,min=1970,max=2200"`
	ReleasedTo   int    `json:"released_to" validate:"omitempty,min=1970,max=2200"`
	IncludeIntro bool   `json:"include_intro"`
	Sort         string `json:"sort" validate:"omitempty,oneof=relevance released_desc released_asc view updated"`
	Page         int    `json:"page" validate:"required,min=1"`
	Limit        int    `json:"limit" validate:"required,min=1,max=50"`
}

// SearchHit is a single result returned to the frontend: Wiki galgame info plus whether this service has a patch.
type SearchHit struct {
	galgameClient.GalgameHit
	HasPatch bool                   `json:"has_patch"`
	Patch    *patchModel.Patch      `json:"patch,omitempty"`
}

// Search POST /api/search
func (h *Handler) Search(c *fiber.Ctx) error {
	var req SearchRequest
	if err := utils.ParseAndValidate(c, &req); err != nil {
		return response.Error(c, errors.ErrBadRequest(err.Error()))
	}

	// NSFW filter via wiki content_limit protocol — read from URL query, not
	// from a custom header. The legacy X-NSFW-Header path was wrong per
	// docs/galgame_wiki/00-handbook §16.6: wiki spec explicitly forbids
	// custom headers / JSON-as-header for the NSFW gate. Frontend now appends
	// ?content_limit=all when the user has opted in to NSFW; missing /
	// unrecognised falls back to "sfw".
	contentLimit := utils.ContentLimitForListBrowse(c)

	// Call Wiki search
	params := galgameClient.SearchGalgameParams{
		Q: req.Q,
		// Public search only surfaces published galgames. Unpublished states
		// (1 banned / 2 VNDB draft / 3 pending / 4 declined) are excluded.
		// The publish wizard uses a separate SearchGalgameForPublish call
		// that intentionally includes the caller's own pending drafts.
		Status:        "0",
		ContentLimit:  contentLimit,
		AgeLimit:      req.AgeLimit,
		OriginalLang:  req.OriginalLang,
		TagIDs:        req.TagIDs,
		OfficialIDs:   req.OfficialIDs,
		EngineIDs:     req.EngineIDs,
		ReleasedFrom:  req.ReleasedFrom,
		ReleasedTo:    req.ReleasedTo,
		IncludeIntro:  req.IncludeIntro,
		Sort:          req.Sort,
		Page:          req.Page,
		Limit:         req.Limit,
	}
	wikiResult, err := h.wiki.SearchGalgame(c.Context(), params)
	if err != nil {
		slog.Error("Wiki 搜索失败", "error", err)
		return response.Error(c, errors.ErrInternal("搜索服务暂不可用"))
	}

	// Extract vndb_ids and look up the local patch table
	vndbIDs := make([]string, 0, len(wikiResult.Items))
	for _, item := range wikiResult.Items {
		if item.VndbID != "" {
			vndbIDs = append(vndbIDs, item.VndbID)
		}
	}

	patchMap := map[string]*patchModel.Patch{}
	if len(vndbIDs) > 0 {
		var patches []patchModel.Patch
		if err := h.db.WithContext(c.Context()).
			Where("vndb_id IN ?", vndbIDs).
			Find(&patches).Error; err != nil {
			slog.Error("查询本地 patch 失败", "error", err)
			return response.Error(c, errors.ErrInternal(""))
		}
		for i := range patches {
			patchMap[patches[i].VndbID] = &patches[i]
		}
	}

	// Merge: preserve Wiki's relevance order; stamp each hit with has_patch + patch details
	hits := make([]SearchHit, 0, len(wikiResult.Items))
	for _, item := range wikiResult.Items {
		h := SearchHit{GalgameHit: item}
		if p, ok := patchMap[item.VndbID]; ok {
			h.HasPatch = true
			h.Patch = p
		}
		hits = append(hits, h)
	}

	return response.Paginated(c, hits, wikiResult.Total)
}

