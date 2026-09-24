package service

import (
	"context"
	"errors"
	"log/slog"
	"sort"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/catalogv2"
)

// ErrNoCatalogWork is what the heart button gets on a page catalog cannot name.
// Those pages live in the local-only id band since migration 037 and there are
// 20 of them, all already unreachable. Left unmapped this fell through to a 500
// "please try again later" for a condition that never succeeds on a retry.
var ErrNoCatalogWork = errors.New("patch has no catalog work")

// Favourites live in the catalog. This site never had folders — a favourite
// was one row in user_patch_favorite_relation — and the 2026-09-07 backfill
// moved all 61,796 of them into each person's DEFAULT catalog folder. So the
// heart button keeps meaning exactly what it meant: in or out of the default
// folder. Folders are the new part on top.
//
// user_patch_favorite_relation is frozen from the cutover as rollback material
// and is read by nothing.

type FolderView struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Visibility  string `json:"visibility"`
	IsDefault   bool   `json:"is_default"`
	ItemCount   int    `json:"item_count"`
	Created     string `json:"created"`
	Updated     string `json:"updated"`
	// Image hashes, not URLs: this site's clients build the image_service URL.
	PreviewCovers []string `json:"preview_covers"`
}

type FolderMembership struct {
	FolderView
	Contains bool `json:"contains"`
}

func folderView(f catalogv2.Folder) FolderView {
	return FolderView{
		ID: f.ID, Name: f.Name, Description: f.Description, Visibility: f.Visibility,
		IsDefault: f.IsDefault, ItemCount: f.ItemCount,
		Created: f.CreatedAt, Updated: f.UpdatedAt,
		// Empty, not nil: a nil slice marshals to `null` and the card reads
		// .length off it.
		PreviewCovers: []string{},
	}
}

// sortFolders puts the default folder first and then the most recently
// touched, which is the order a person reads their own shelf in. The catalog
// answers id-ascending because that keyset is a sync watermark.
func sortFolders(rows []catalogv2.Folder) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].IsDefault != rows[j].IsDefault {
			return rows[i].IsDefault
		}
		return rows[i].UpdatedAt > rows[j].UpdatedAt
	})
}

func (s *PatchService) LegacyRedirect(oldID int) (int, bool) {
	return s.repo.LegacyRedirect(oldID)
}

func (s *PatchService) MergeRedirect(workID int) (int, bool) {
	return s.repo.MergeRedirect(workID)
}

func (s *PatchService) workIDOf(patchID int) (int64, error) {
	if patchID <= 0 || model.IsLocalOnly(patchID) {
		return 0, ErrNoCatalogWork
	}
	return int64(patchID), nil
}

func (s *PatchService) MyFolders(ctx context.Context, token, contentLimit string) ([]FolderView, error) {
	rows, err := s.galgame.V2().MyFolders(ctx, token)
	if err != nil {
		return nil, err
	}
	out := folderViews(rows)
	s.attachPreviewCovers(ctx, out, token, contentLimit)
	return out, nil
}

func (s *PatchService) PublicFolders(ctx context.Context, ownerUID int, contentLimit string) ([]FolderView, error) {
	rows, err := s.galgame.V2().PublicFolders(ctx, int64(ownerUID))
	if err != nil {
		return nil, err
	}
	out := folderViews(rows)
	// No token: a stranger's covers come off the public items lane, which
	// answers public folders only — which is all this list holds.
	s.attachPreviewCovers(ctx, out, "", contentLimit)
	return out, nil
}

func folderViews(rows []catalogv2.Folder) []FolderView {
	sortFolders(rows)
	out := make([]FolderView, 0, len(rows))
	for _, f := range rows {
		out = append(out, folderView(f))
	}
	return out
}

// previewCoversPerFolder is the mosaic a shelf card draws, and it costs one
// request per non-empty folder. Measured 2026-09-12: p99 is 3 folders per
// person and the largest shelf in production has 27, so the loop is not worth
// paging or parallelising.
const previewCoversPerFolder = 4

// attachPreviewCovers fills in the covers a folder card draws.
//
// A card can come back with fewer covers than the folder has items, or with
// none: the shelf is shared with kungal and can hold games this site has no
// page for, and the reader's NSFW gate drops rows here exactly as it does on
// every other list. Both are the answer, not a failure — which is why a folder
// that cannot be read at all only loses its covers and still renders.
func (s *PatchService) attachPreviewCovers(ctx context.Context, views []FolderView, token, contentLimit string) {
	if s.galgame == nil {
		return
	}
	byFolder := make(map[int64][]int64, len(views))
	var works []int64
	for _, v := range views {
		if v.ItemCount == 0 {
			continue
		}
		items, err := s.galgame.V2().FolderPreviewItems(ctx, token, v.ID, previewCoversPerFolder)
		if err != nil {
			slog.Warn("收藏夹封面：读取条目失败", "folder_id", v.ID, "error", err)
			continue
		}
		for _, it := range items {
			byFolder[v.ID] = append(byFolder[v.ID], it.WorkID)
			works = append(works, it.WorkID)
		}
	}
	if len(works) == 0 {
		return
	}
	hashes, err := s.bannerHashesByWork(ctx, works, contentLimit)
	if err != nil {
		slog.Warn("收藏夹封面：富化失败", "error", err)
		return
	}
	for i := range views {
		for _, workID := range byFolder[views[i].ID] {
			if hash := hashes[workID]; hash != "" {
				views[i].PreviewCovers = append(views[i].PreviewCovers, hash)
			}
		}
	}
}

func (s *PatchService) bannerHashesByWork(ctx context.Context, workIDs []int64, contentLimit string) (map[int64]string, error) {
	ids := patchIDsOf(workIDs)
	out := make(map[int64]string, len(ids))
	for start := 0; start < len(ids); start += galgameClient.BatchMaxIDs {
		briefs, bErr := s.galgame.GalgameBatch(ctx, ids[start:min(start+galgameClient.BatchMaxIDs, len(ids))], contentLimit)
		if bErr != nil {
			return nil, bErr
		}
		for i := range briefs {
			if h := briefs[i].EffectiveBannerHash; h != "" {
				out[int64(briefs[i].ID)] = h
			}
		}
	}
	return out, nil
}

func (s *PatchService) CreateFolder(ctx context.Context, token string, userID int, name, description, visibility string) (*FolderView, error) {
	f, err := s.galgame.V2().CreateFolder(ctx, token, userID, catalogv2.FolderWrite{
		Name: &name, Description: &description, Visibility: &visibility,
	})
	if err != nil {
		return nil, err
	}
	v := folderView(*f)
	return &v, nil
}

func (s *PatchService) UpdateFolder(ctx context.Context, token string, folderID int64, in catalogv2.FolderWrite) (*FolderView, error) {
	f, err := s.galgame.V2().PatchFolder(ctx, token, folderID, in)
	if err != nil {
		return nil, err
	}
	v := folderView(*f)
	return &v, nil
}

func (s *PatchService) DeleteFolder(ctx context.Context, token string, folderID int64) error {
	return s.galgame.V2().DeleteFolder(ctx, token, folderID)
}

// FolderPatches turns a folder's work ids back into this site's patches. Works
// this site does not carry are dropped rather than rendered as holes: a person
// can favourite a game on the forum that has no patch page here, and the
// folder is shared between the two.
func (s *PatchService) FolderPatches(ctx context.Context, token string, folderID int64, viewerIsOwner bool, page, limit int) (*FolderView, []model.Patch, int, error) {
	var (
		folder *catalogv2.Folder
		items  []catalogv2.FolderItem
		err    error
	)
	if viewerIsOwner && token != "" {
		if folder, err = s.galgame.V2().MyFolder(ctx, token, folderID); err == nil {
			items, err = s.galgame.V2().MyFolderItems(ctx, token, folderID)
		}
	} else {
		if folder, err = s.galgame.V2().PublicFolder(ctx, folderID); err == nil {
			items, err = s.galgame.V2().PublicFolderItems(ctx, folderID)
		}
	}
	if err != nil {
		return nil, nil, 0, err
	}

	// Newest added first, which is what a shelf shows. The catalog's own order
	// is the updated_at watermark and is not a reading order.
	sort.SliceStable(items, func(i, j int) bool { return items[i].CreatedAt > items[j].CreatedAt })
	workIDs := make([]int64, 0, len(items))
	for _, it := range items {
		workIDs = append(workIDs, it.WorkID)
	}
	here, mErr := s.repo.ExistingPatchIDs(patchIDsOf(workIDs))
	if mErr != nil {
		return nil, nil, 0, mErr
	}
	ids := make([]int, 0, len(workIDs))
	for _, w := range workIDs {
		if pid := int(w); here[pid] {
			ids = append(ids, pid)
		}
	}
	// The total is what this site can draw, not folder.ItemCount: a shelf is
	// shared with kungal and the games it holds that have no page here are
	// dropped above, so paging on the catalog's count would leave trailing
	// pages empty.
	total := len(ids)
	patches, pErr := s.repo.PatchesByIDsOrdered(pageOfIDs(ids, page, limit))
	if pErr != nil {
		return nil, nil, 0, pErr
	}
	v := folderView(*folder)
	return &v, patches, total, nil
}

// A folder item names a catalog work and a page id IS that number, so the two
// lists differ only in type.
func patchIDsOf(workIDs []int64) []int {
	out := make([]int, 0, len(workIDs))
	for _, w := range workIDs {
		if w > 0 {
			out = append(out, int(w))
		}
	}
	return out
}

func pageOfIDs(ids []int, page, limit int) []int {
	start := (max(page, 1) - 1) * limit
	if start >= len(ids) {
		return nil
	}
	return ids[start:min(start+limit, len(ids))]
}
