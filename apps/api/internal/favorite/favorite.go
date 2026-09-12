// Package favorite answers "which games has this person favourited", and its
// reverse "who has favourited this game", out of the catalog folders that have
// owned both answers since the 2026-09-07 cutover.
//
// It exists because five surfaces went on reading user_patch_favorite_relation
// after the cutover froze it. Nothing writes that table any more, so the
// profile counter stopped moving the day the new binary deployed: user 121089
// was shown "10 收藏" over an empty list, and the calendar and the resource page
// drew their hearts from the same snapshot.
package favorite

import (
	"context"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/pkg/catalogv2"
)

// WorkIDs is every catalog work in the folders this reader may see, in folder
// order and deduplicated — a game filed in two folders is one favourite.
//
// The visibility rule is the catalog's own: /v2/me/folders answers private
// folders and only to their owner, /v2/folders answers the public ones to
// anybody. So a person reading their own shelf sees all of it and a visitor
// sees what the owner published. Every caller must therefore reach here with
// the reader's token, which is what the /user/:id/favorite route was missing.
func WorkIDs(ctx context.Context, gal *galgameClient.Client, ownerUID int, token string, isOwner bool) ([]int64, error) {
	if gal == nil {
		return nil, nil
	}
	v2 := gal.V2()
	own := isOwner && token != ""

	var (
		folders []catalogv2.Folder
		err     error
	)
	if own {
		folders, err = v2.MyFolders(ctx, token)
	} else {
		folders, err = v2.PublicFolders(ctx, int64(ownerUID))
	}
	if err != nil {
		return nil, err
	}

	seen := map[int64]bool{}
	out := []int64{}
	for _, f := range folders {
		if f.ItemCount == 0 {
			continue
		}
		var items []catalogv2.FolderItem
		if own {
			items, err = v2.MyFolderItems(ctx, token, f.ID)
		} else {
			items, err = v2.PublicFolderItems(ctx, f.ID)
		}
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			if seen[it.WorkID] {
				continue
			}
			seen[it.WorkID] = true
			out = append(out, it.WorkID)
		}
	}
	return out, nil
}

// Holds is the heart button on one game's page.
func Holds(ctx context.Context, gal *galgameClient.Client, token string, workID int64) (bool, error) {
	held, err := HoldsAll(ctx, gal, token, []int64{workID})
	if err != nil {
		return false, err
	}
	return held[workID], nil
}

// HoldsAll is the same question for a whole page of games at once. A work the
// reader holds nowhere is absent from the map rather than present as false, so
// read it with the zero value and never with a length check.
func HoldsAll(ctx context.Context, gal *galgameClient.Client, token string, workIDs []int64) (map[int64]bool, error) {
	out := map[int64]bool{}
	if gal == nil || token == "" {
		return out, nil
	}
	ids := make([]int64, 0, len(workIDs))
	for _, id := range workIDs {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return out, nil
	}
	holdings, err := gal.V2().MyFolderHoldings(ctx, token, ids)
	if err != nil {
		return nil, err
	}
	for _, h := range holdings {
		out[h.WorkID] = true
	}
	return out, nil
}

// Holders is the other direction, and the only one that does not take a
// reader's token: it answers every account holding the work, private folders
// included, off the application key. Callers on this site must intersect the
// answer with the local user table before writing anything keyed on it — the
// folders are shared with the forum, and 5308 of the 11005 accounts holding
// one have no row here at all.
func Holders(ctx context.Context, gal *galgameClient.Client, workID int64) ([]int64, error) {
	if gal == nil || workID <= 0 {
		return nil, nil
	}
	return gal.V2().FolderHolders(ctx, workID)
}
