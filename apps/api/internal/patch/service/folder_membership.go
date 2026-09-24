package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"kun-galgame-patch-api/pkg/catalogv2"
	"kun-galgame-patch-api/pkg/upstream"
)

var ErrFolderNotYours = errors.New("folder is not yours")

// Catalog refuses a blank name on POST /v2/me/folders — only its backfill wrote
// unnamed defaults — so the unnamed default this used to create answered 422
// "folder name is required." to every reader who had no folder yet.
const defaultFolderName = "默认收藏夹"

// FoldersForPatch is the add-to-folder picker: every folder the person owns,
// each flagged with whether it already holds this game.
func (s *PatchService) FoldersForPatch(ctx context.Context, token string, patchID, userID int) ([]FolderMembership, error) {
	workID, err := s.workIDOf(patchID)
	if err != nil {
		return nil, err
	}
	folders, err := s.favorites.OwnFolders(ctx, userID, token)
	if err != nil {
		return nil, err
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
func (s *PatchService) SetPatchFolders(ctx context.Context, token string, patchID, userID int, targets []int64) error {
	workID, err := s.workIDOf(patchID)
	if err != nil {
		return err
	}
	defer s.favorites.Forget(ctx, userID)
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
			return ErrFolderNotYours
		}
		want[id] = true
	}

	holding, err := s.galgame.V2().MyFoldersHolding(ctx, token, workID)
	if err != nil {
		return err
	}
	held := map[int64]bool{}
	for _, f := range holding {
		held[f.ID] = true
	}
	wasHeld := len(held) > 0
	moveErr := s.moveFolderItems(ctx, token, workID, held, want)
	s.settleFavoriteSideEffects(ctx, patchID, userID, !wasHeld && len(held) > 0, wasHeld && len(held) == 0)
	return moveErr
}

// moveFolderItems stops at the first refusal and leaves held as catalog now has
// it, so the counter and the author's moemoepoints settle on what the writes
// that went through did. Until 2026-09-24 a refusal returned before the
// settle, and the writes ahead of it stayed off the count for good.
func (s *PatchService) moveFolderItems(ctx context.Context, token string, workID int64, held, want map[int64]bool) error {
	for id := range want {
		if held[id] {
			continue
		}
		if err := s.galgame.V2().PutFolderItem(ctx, token, id, workID); err != nil {
			return err
		}
		held[id] = true
	}
	for id := range held {
		if want[id] {
			continue
		}
		if err := s.galgame.V2().DeleteFolderItem(ctx, token, id, workID); err != nil {
			return err
		}
		delete(held, id)
	}
	return nil
}

// A second create with is_default does not conflict in catalog, it demotes the
// first, so two hearts racing on an empty shelf made two folders. The key names
// the shelf the heart saw, so racing hearts send one create between them and a
// default deleted later is made again rather than replayed.
func defaultFolderKey(userID int, seen []catalogv2.Folder) string {
	parts := []string{strconv.Itoa(userID), "catalog.default-folder"}
	for _, f := range seen {
		parts = append(parts, strconv.FormatInt(f.ID, 10))
	}
	return upstream.IdempotencyKey(parts...)
}

func defaultOf(folders []catalogv2.Folder) (int64, bool) {
	for _, f := range folders {
		if f.IsDefault {
			return f.ID, true
		}
	}
	return 0, false
}

const defaultFolderAttempts = 3

var defaultFolderRetryDelay = 250 * time.Millisecond

// A person can end up with folders but no default — the flag moves, and a
// moderator may have deleted the folder that held it — so the heart button
// makes one rather than refusing. The racing heart that loses is answered
// IDEMPOTENCY_REQUEST_IN_PROGRESS while the winner's create runs; sending the
// same key again once it has finished replays the winner's folder.
func (s *PatchService) defaultFolderID(ctx context.Context, token string, userID int) (int64, error) {
	folders, err := s.galgame.V2().MyFolders(ctx, token)
	if err != nil {
		return 0, err
	}
	if id, ok := defaultOf(folders); ok {
		return id, nil
	}
	name, empty, isDefault := defaultFolderName, "", true
	pub := catalogv2.FolderVisibilityPublic
	in := catalogv2.FolderWrite{Name: &name, Description: &empty, Visibility: &pub, IsDefault: &isDefault}
	key := defaultFolderKey(userID, folders)
	for attempt := 1; ; attempt++ {
		created, err := s.galgame.V2().CreateFolder(ctx, token, key, in)
		if err == nil {
			return created.ID, nil
		}
		if upstream.KindOf(err) != upstream.Conflict || attempt == defaultFolderAttempts {
			return 0, err
		}
		select {
		case <-ctx.Done():
			return 0, err
		case <-time.After(defaultFolderRetryDelay):
		}
	}
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
		return false, err
	}
	defer s.favorites.Forget(ctx, userID)

	holding, err := s.galgame.V2().MyFoldersHolding(ctx, token, workID)
	if err != nil {
		return false, err
	}
	if len(holding) > 0 {
		held := map[int64]bool{}
		for _, f := range holding {
			held[f.ID] = true
		}
		if err := s.moveFolderItems(ctx, token, workID, held, nil); err != nil {
			return true, err
		}
		s.settleFavoriteSideEffects(ctx, patchID, userID, false, true)
		return false, nil
	}

	folderID, err := s.defaultFolderID(ctx, token, userID)
	if err != nil {
		return false, err
	}
	if err := s.galgame.V2().PutFolderItem(ctx, token, folderID, workID); err != nil {
		return false, err
	}
	s.settleFavoriteSideEffects(ctx, patchID, userID, true, false)
	return true, nil
}

func (s *PatchService) IsFavoritedInCatalog(ctx context.Context, userID int, token string, patchID int) (bool, error) {
	workID, err := s.workIDOf(patchID)
	if err != nil {
		return false, nil
	}
	return s.favorites.Holds(ctx, userID, token, workID)
}

// The local counter and the author's moemoepoints follow the upstream write,
// never precede it. patch.favorite_count backs this site's own sorting; the
// number a reader sees on a game page comes from the catalog's
// nextmoe/favorites row, which counts people across every site.
//
// The award key names the favouriter. Keyed on the game alone it paid the
// author for the first favourite a game ever got and replayed every later one
// as a no-op (2026-09-07 to 2026-09-24). Without a per-favourite row id to key
// on, a favourite that is taken back and given again pays nothing the second
// time.
func (s *PatchService) settleFavoriteSideEffects(ctx context.Context, patchID, userID int, added, removed bool) {
	if !added && !removed {
		return
	}
	delta, event := 1, "favorited"
	if removed {
		delta, event = -1, "unfavorited"
	}
	s.repo.UpdateCount(patchID, "favorite_count", delta)

	patch, err := s.repo.GetPatchDetail(patchID)
	if err != nil || patch == nil || patch.UserID == 0 || patch.UserID == userID {
		return
	}
	go s.mp.Award(context.WithoutCancel(ctx), patch.UserID, delta, "liked",
		fmt.Sprintf("galgame:%d", patchID), fmt.Sprintf("moyu:%s:%d:%d", event, patchID, userID))
}
