package repository

import (
	"kun-galgame-patch-api/internal/patch/model"
)

// ExistingPatchIDs narrows catalog work ids to the ones this site has a page
// for. A folder is shared with the forum, so a shelf routinely holds games that
// were never published here, and they have to be dropped before the count is
// taken or the last pages come back empty. This used to be a translation --
// patch.catalog_work_id was a second id space that had to be mapped back -- and
// is a primary-key lookup now.
func (r *PatchRepository) ExistingPatchIDs(ids []int) (map[int]bool, error) {
	out := make(map[int]bool, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var found []int
	if err := r.db.Table("patch").Where("id IN ?", ids).Pluck("id", &found).Error; err != nil {
		return nil, err
	}
	for _, id := range found {
		out[id] = true
	}
	return out, nil
}

// PatchesByIDsOrdered keeps the order the caller asked for. The folder decides
// the order — newest added first — and an `IN (?)` would hand back whatever
// the planner felt like.
func (r *PatchRepository) PatchesByIDsOrdered(ids []int) ([]model.Patch, error) {
	out := []model.Patch{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []model.Patch
	if err := r.db.Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	byID := make(map[int]model.Patch, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	for _, id := range ids {
		if row, ok := byID[id]; ok {
			out = append(out, row)
		}
	}
	return out, nil
}
