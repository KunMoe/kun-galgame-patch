// Package favorite answers "which games has this person favourited", and its
// reverse "who has favourited this game", out of the catalog folders that have
// owned both answers since the 2026-09-07 cutover.
//
// It exists because five surfaces went on reading user_patch_favorite_relation
// after the cutover froze it. Nothing writes that table any more, so the
// profile counter stopped moving the day the new binary deployed: user 121089
// was shown "10 收藏" over an empty list, and the calendar and the resource page
// drew their hearts from the same snapshot.
//
// A reader's own shelf is read with their token, which spends their catalog
// allowance shared with every other NextMoe site (see usercache), so it is kept
// per reader for shelfTTL. Favourites made on another site reach this site's
// hearts and counts within that window; this site's own writes call Forget.
package favorite

import (
	"context"
	"fmt"
	"slices"
	"time"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/usercache"
	"kun-galgame-patch-api/pkg/catalogv2"
)

const (
	scope    = "favorites"
	shelfTTL = 10 * time.Minute
	// Past this many item pages a heart is asked of catalog one question at a
	// time: a walk that size every shelfTTL costs more than the views it saves.
	shelfWalkMax = 20
	previewTTL   = 24 * time.Hour
)

type Service struct {
	gal   *galgameClient.Client
	cache *usercache.Cache
}

func New(gal *galgameClient.Client, cache *usercache.Cache) *Service {
	return &Service{gal: gal, cache: cache}
}

type shelf struct {
	Folders []catalogv2.Folder `json:"folders"`
	Works   []int64            `json:"works"`
	Walked  bool               `json:"walked"`
}

func (s *Service) Forget(ctx context.Context, uid int) {
	if s != nil {
		s.cache.Forget(ctx, uid, scope)
	}
}

// OwnFolders is every folder the reader owns, private ones included, in
// catalog's order.
func (s *Service) OwnFolders(ctx context.Context, uid int, token string) ([]catalogv2.Folder, error) {
	sh, err := s.shelf(ctx, uid, token, false)
	return slices.Clone(sh.Folders), err
}

// WorkIDs is every catalog work in the folders this reader may see,
// deduplicated — a game filed in two folders is one favourite.
//
// The visibility rule is the catalog's own: /v2/me/folders answers private
// folders and only to their owner, /v2/folders answers the public ones to
// anybody. So a person reading their own shelf sees all of it and a visitor
// sees what the owner published, off the application key. The owner's read
// must therefore arrive with the owner's token, which is what the
// /user/:id/favorite route was missing.
func (s *Service) WorkIDs(ctx context.Context, ownerUID int, token string, isOwner bool) ([]int64, error) {
	if s == nil || s.gal == nil {
		return nil, nil
	}
	if isOwner && token != "" {
		sh, err := s.shelf(ctx, ownerUID, token, true)
		return sh.Works, err
	}
	folders, err := s.gal.V2().PublicFolders(ctx, int64(ownerUID))
	if err != nil {
		return nil, err
	}
	return walkWorks(ctx, folders, s.gal.V2().PublicFolderItems)
}

// Holds is the heart button on one game's page.
func (s *Service) Holds(ctx context.Context, uid int, token string, workID int64) (bool, error) {
	held, err := s.HoldsAll(ctx, uid, token, []int64{workID})
	return held[workID], err
}

// HoldsAll is the same question for a whole page of games at once. A work the
// reader holds nowhere is absent from the map rather than present as false, so
// read it with the zero value and never with a length check.
func (s *Service) HoldsAll(ctx context.Context, uid int, token string, workIDs []int64) (map[int64]bool, error) {
	out := map[int64]bool{}
	want := make(map[int64]bool, len(workIDs))
	for _, id := range workIDs {
		if id > 0 {
			want[id] = true
		}
	}
	if s == nil || s.gal == nil || uid <= 0 || token == "" || len(want) == 0 {
		return out, nil
	}
	sh, err := s.shelf(ctx, uid, token, false)
	if err != nil {
		return nil, err
	}
	if !sh.Walked {
		ids := make([]int64, 0, len(want))
		for id := range want {
			ids = append(ids, id)
		}
		holdings, err := s.gal.V2().MyFolderHoldings(ctx, token, ids)
		if err != nil {
			return nil, err
		}
		for _, h := range holdings {
			out[h.WorkID] = true
		}
		return out, nil
	}
	for _, w := range sh.Works {
		if want[w] {
			out[w] = true
		}
	}
	return out, nil
}

// PreviewWorkIDs is the first few works of one folder, for its card's covers.
// Catalog moves a folder's updated_at on every item added or removed, so a key
// naming it goes stale by itself, whichever site made the change.
func (s *Service) PreviewWorkIDs(ctx context.Context, token string, folderID int64, updatedAt string, n int) ([]int64, error) {
	fill := func(ctx context.Context) ([]int64, error) {
		items, err := s.gal.V2().FolderPreviewItems(ctx, token, folderID, n)
		if err != nil {
			return nil, err
		}
		ids := make([]int64, 0, len(items))
		for _, it := range items {
			ids = append(ids, it.WorkID)
		}
		return ids, nil
	}
	if updatedAt == "" {
		return fill(ctx)
	}
	key := fmt.Sprintf("folder-preview:%d:%s:%d", folderID, updatedAt, n)
	return usercache.Fetch(ctx, s.cache.Key(key), previewTTL, fill)
}

// Holders is the other direction, and the only one that does not take a
// reader's token: it answers every account holding the work, private folders
// included, off the application key. Callers on this site must intersect the
// answer with the local user table before writing anything keyed on it — the
// folders are shared with the forum, and 5308 of the 11005 accounts holding one
// have no row here at all.
func Holders(ctx context.Context, gal *galgameClient.Client, workID int64) ([]int64, error) {
	if gal == nil || workID <= 0 {
		return nil, nil
	}
	return gal.V2().FolderHolders(ctx, workID)
}

func (s *Service) shelf(ctx context.Context, uid int, token string, complete bool) (shelf, error) {
	if s == nil || s.gal == nil {
		return shelf{}, catalogv2.ErrNotConfigured
	}
	if token == "" {
		return shelf{}, catalogv2.ErrNoAccessToken
	}
	slot := s.cache.Slot(ctx, uid, scope, "shelf")
	got, err := usercache.Fetch(ctx, slot, shelfTTL, func(ctx context.Context) (shelf, error) {
		return s.readShelf(ctx, token, complete)
	})
	if err != nil || got.Walked || !complete {
		return got, err
	}
	full, err := s.readShelf(ctx, token, true)
	if err == nil {
		slot.Store(ctx, full, shelfTTL)
	}
	return full, err
}

func (s *Service) readShelf(ctx context.Context, token string, complete bool) (shelf, error) {
	v2 := s.gal.V2()
	folders, err := v2.MyFolders(ctx, token)
	if err != nil {
		return shelf{}, err
	}
	sh := shelf{Folders: folders}
	if !complete && itemPages(folders) > shelfWalkMax {
		return sh, nil
	}
	sh.Works, err = walkWorks(ctx, folders, func(ctx context.Context, folderID int64) ([]catalogv2.FolderItem, error) {
		return v2.MyFolderItems(ctx, token, folderID)
	})
	if err != nil {
		return shelf{}, err
	}
	sh.Walked = true
	return sh, nil
}

func itemPages(folders []catalogv2.Folder) int {
	n := 0
	for _, f := range folders {
		n += (f.ItemCount + catalogv2.FolderPageMax - 1) / catalogv2.FolderPageMax
	}
	return n
}

func walkWorks(ctx context.Context, folders []catalogv2.Folder,
	items func(context.Context, int64) ([]catalogv2.FolderItem, error)) ([]int64, error) {
	seen := map[int64]bool{}
	out := []int64{}
	for _, f := range folders {
		if f.ItemCount == 0 {
			continue
		}
		rows, err := items(ctx, f.ID)
		if err != nil {
			return nil, err
		}
		for _, it := range rows {
			if !seen[it.WorkID] {
				seen[it.WorkID] = true
				out = append(out, it.WorkID)
			}
		}
	}
	return out, nil
}
