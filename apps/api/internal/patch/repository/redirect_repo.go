package repository

// A page id can move twice for unrelated reasons, and the two must not be
// looked up on the same URL. See migration 037: `legacy` rows name a pre-037
// page number and are honoured only by the /patch/<n> shim, because that same
// number is usually also a live catalog work with a legitimate /galgame/<n> of
// its own. `merge` rows name a catalog work that catalog merged away, which
// stops being a work at all and is therefore unambiguous on the new path.
const (
	RedirectScopeLegacy = "legacy"
	RedirectScopeMerge  = "merge"
)

func (r *PatchRepository) redirectTarget(scope string, oldID int) (int, bool) {
	if oldID <= 0 {
		return 0, false
	}
	var newID []int
	r.db.Table("patch_redirect").
		Where("old_id = ? AND scope = ?", oldID, scope).
		Pluck("new_id", &newID)
	if len(newID) == 0 || newID[0] <= 0 {
		return 0, false
	}
	return newID[0], true
}

// LegacyRedirect answers the /patch/<n> shim: where the page that used to carry
// this number lives now.
func (r *PatchRepository) LegacyRedirect(oldID int) (int, bool) {
	return r.redirectTarget(RedirectScopeLegacy, oldID)
}

// MergeRedirect answers /galgame/<id> once catalog has merged that work away.
func (r *PatchRepository) MergeRedirect(workID int) (int, bool) {
	return r.redirectTarget(RedirectScopeMerge, workID)
}
