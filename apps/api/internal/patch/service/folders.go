package service

import (
	"context"
	"fmt"
	"sort"

	"kun-galgame-patch-api/internal/favorite"
	"kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/internal/patch/repository"
	"kun-galgame-patch-api/pkg/catalogv2"
)

// ErrNoCatalogWork is the repository's sentinel, re-exported so the handler can
// map it without reaching past the service. 84 patches carry no work — 78
// `pending-<n>` placeholders plus 13 v-numbers the catalog has never anchored —
// and 76 of them are published with resources, so this is a real button people
// press. Left unmapped it fell through to a 500 "please try again later" for a
// condition that will never succeed on a retry.
var ErrNoCatalogWork = repository.ErrNoCatalogWork

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

func (s *PatchService) workIDOf(patchID int) (int64, error) {
	return s.repo.CatalogWorkID(patchID)
}

func (s *PatchService) MyFolders(ctx context.Context, token string) ([]FolderView, error) {
	rows, err := s.galgame.V2().MyFolders(ctx, token)
	if err != nil {
		return nil, err
	}
	sortFolders(rows)
	out := make([]FolderView, 0, len(rows))
	for _, f := range rows {
		out = append(out, folderView(f))
	}
	return out, nil
}

func (s *PatchService) PublicFolders(ctx context.Context, ownerUID int) ([]FolderView, error) {
	rows, err := s.galgame.V2().PublicFolders(ctx, int64(ownerUID))
	if err != nil {
		return nil, err
	}
	sortFolders(rows)
	out := make([]FolderView, 0, len(rows))
	for _, f := range rows {
		out = append(out, folderView(f))
	}
	return out, nil
}

func (s *PatchService) CreateFolder(ctx context.Context, token, name, description, visibility string) (*FolderView, error) {
	f, err := s.galgame.V2().CreateFolder(ctx, token, catalogv2.FolderWrite{
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
func (s *PatchService) FolderPatches(ctx context.Context, token string, folderID int64, viewerIsOwner bool) (*FolderView, []model.Patch, error) {
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
		return nil, nil, err
	}

	// Newest added first, which is what a shelf shows. The catalog's own order
	// is the updated_at watermark and is not a reading order.
	sort.SliceStable(items, func(i, j int) bool { return items[i].CreatedAt > items[j].CreatedAt })
	workIDs := make([]int64, 0, len(items))
	for _, it := range items {
		workIDs = append(workIDs, it.WorkID)
	}
	byWork, mErr := s.repo.PatchIDsByWorkIDs(workIDs)
	if mErr != nil {
		return nil, nil, mErr
	}
	ids := make([]int, 0, len(workIDs))
	for _, w := range workIDs {
		if pid, ok := byWork[w]; ok {
			ids = append(ids, pid)
		}
	}
	patches, pErr := s.repo.PatchesByIDsOrdered(ids)
	if pErr != nil {
		return nil, nil, pErr
	}
	v := folderView(*folder)
	return &v, patches, nil
}

// FoldersForPatch is the add-to-folder picker: every folder the person owns,
// each flagged with whether it already holds this game. The person's first
// visit creates their default folder, because a site with a heart button and
// no folder to put things in is not a state anyone chose.
func (s *PatchService) FoldersForPatch(ctx context.Context, token string, patchID int) ([]FolderMembership, error) {
	workID, err := s.workIDOf(patchID)
	if err != nil {
		return nil, err
	}
	folders, err := s.galgame.V2().MyFolders(ctx, token)
	if err != nil {
		return nil, err
	}
	if len(folders) == 0 {
		created, cErr := s.ensureDefaultFolder(ctx, token)
		if cErr != nil {
			return nil, cErr
		}
		folders = []catalogv2.Folder{*created}
	}
	holding, err := s.galgame.V2().MyFoldersHolding(ctx, token, workID)
	if err != nil {
		return nil, err
	}
	has := map[int64]bool{}
	for _, f := range holding {
		has[f.ID] = true
	}
	sortFolders(folders)
	out := make([]FolderMembership, 0, len(folders))
	for _, f := range folders {
		out = append(out, FolderMembership{FolderView: folderView(f), Contains: has[f.ID]})
	}
	return out, nil
}

// SetPatchFolders makes the person's folders holding this game exactly the set
// they asked for.
func (s *PatchService) SetPatchFolders(ctx context.Context, token string, patchID int, targets []int64) error {
	workID, err := s.workIDOf(patchID)
	if err != nil {
		return err
	}
	owned, err := s.galgame.V2().MyFolders(ctx, token)
	if err != nil {
		return err
	}
	mine := map[int64]bool{}
	for _, f := range owned {
		mine[f.ID] = true
	}
	want := map[int64]bool{}
	for _, id := range targets {
		if !mine[id] {
			return fmt.Errorf("folder %d is not yours", id)
		}
		want[id] = true
	}

	holding, err := s.galgame.V2().MyFoldersHolding(ctx, token, workID)
	if err != nil {
		return err
	}
	current := map[int64]bool{}
	for _, f := range holding {
		current[f.ID] = true
	}
	for id := range want {
		if !current[id] {
			if err := s.galgame.V2().PutFolderItem(ctx, token, id, workID); err != nil {
				return err
			}
		}
	}
	for id := range current {
		if !want[id] {
			if err := s.galgame.V2().DeleteFolderItem(ctx, token, id, workID); err != nil {
				return err
			}
		}
	}
	s.settleFavoriteSideEffects(ctx, patchID, len(current) == 0 && len(want) > 0, len(current) > 0 && len(want) == 0)
	return nil
}

func (s *PatchService) ensureDefaultFolder(ctx context.Context, token string) (*catalogv2.Folder, error) {
	// Deliberately unnamed: the backfill wrote empty names for every default
	// folder it created, and clients derive the label from the owner's name.
	// Inventing one here would freeze a language into somebody else's view.
	empty, isDefault := "", true
	pub := catalogv2.FolderVisibilityPublic
	return s.galgame.V2().CreateFolder(ctx, token, catalogv2.FolderWrite{
		Name: &empty, Description: &empty, Visibility: &pub, IsDefault: &isDefault,
	})
}

func (s *PatchService) defaultFolderID(ctx context.Context, token string) (int64, error) {
	folders, err := s.galgame.V2().MyFolders(ctx, token)
	if err != nil {
		return 0, err
	}
	for _, f := range folders {
		if f.IsDefault {
			return f.ID, nil
		}
	}
	// A person can end up with folders but no default — the flag moves, and a
	// moderator may have deleted the folder that held it. Making one is
	// cheaper than refusing the heart button.
	created, cErr := s.ensureDefaultFolder(ctx, token)
	if cErr != nil {
		return 0, cErr
	}
	return created.ID, nil
}

// ToggleFavoriteInCatalog is the heart button. In means "in my default
// folder"; out means "in none of my folders", so unfavouriting a game the
// person also filed by hand removes it from those folders too — the button
// says favourited, and it has to be able to make that false.
func (s *PatchService) ToggleFavoriteInCatalog(ctx context.Context, token string, patchID, userID int) (bool, error) {
	workID, err := s.workIDOf(patchID)
	if err != nil {
		return false, err
	}
	if _, err := s.ensureLocalPatch(ctx, patchID, userID); err != nil {
		return false, fmt.Errorf("patch not found")
	}

	holding, err := s.galgame.V2().MyFoldersHolding(ctx, token, workID)
	if err != nil {
		return false, err
	}
	if len(holding) > 0 {
		for _, f := range holding {
			if dErr := s.galgame.V2().DeleteFolderItem(ctx, token, f.ID, workID); dErr != nil {
				return false, dErr
			}
		}
		s.settleFavoriteSideEffects(ctx, patchID, false, true)
		return false, nil
	}

	folderID, err := s.defaultFolderID(ctx, token)
	if err != nil {
		return false, err
	}
	if err := s.galgame.V2().PutFolderItem(ctx, token, folderID, workID); err != nil {
		return false, err
	}
	s.settleFavoriteSideEffects(ctx, patchID, true, false)
	return true, nil
}

func (s *PatchService) IsFavoritedInCatalog(ctx context.Context, token string, patchID int) bool {
	if token == "" {
		return false
	}
	workID, err := s.workIDOf(patchID)
	if err != nil {
		return false
	}
	held, err := favorite.Holds(ctx, s.galgame, token, workID)
	return err == nil && held
}

// The local counter and the author's moemoepoints follow the upstream write,
// never precede it. patch.favorite_count backs this site's own sorting; the
// number a reader sees on a game page comes from the catalog's
// nextmoe/favorites row, which counts people across every site.
func (s *PatchService) settleFavoriteSideEffects(ctx context.Context, patchID int, added, removed bool) {
	if !added && !removed {
		return
	}
	delta := 1
	if removed {
		delta = -1
	}
	s.repo.UpdateCount(patchID, "favorite_count", delta)

	patch, err := s.repo.GetPatchDetail(patchID)
	if err != nil || patch == nil || patch.UserID == 0 {
		return
	}
	go s.mp.Award(context.WithoutCancel(ctx), patch.UserID, delta, "liked",
		fmt.Sprintf("galgame:%d", patchID), fmt.Sprintf("moyu:favorite:%d:%d", patchID, delta))
}
