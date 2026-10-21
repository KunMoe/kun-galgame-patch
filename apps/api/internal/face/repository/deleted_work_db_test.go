package repository

import "testing"

// letmoe merges these resources into its packs through this face; a work
// catalog deleted leaves it with every route, and comes back with the work.
func TestTheFaceDropsADeletedWork(t *testing.T) {
	db := feedDB(t)
	repo := New(db)
	mustExec(t, db, `INSERT INTO patch_resource (id, storage, user_id, galgame_id, updated)
		VALUES (?, 'user', ?, ?, now()), (?, 'user', ?, ?, now())`,
		feedRes, feedUser, feedPatch, feedRes2, feedUser, feedOther)
	mustExec(t, db, `UPDATE patch SET catalog_deleted_at = now() WHERE id = ?`, feedPatch)

	if _, err := repo.GetPatch(feedPatch); err == nil {
		t.Error("GetPatch served a deleted work")
	}
	if _, err := repo.GetResource(feedRes); err == nil {
		t.Error("GetResource served a deleted work's resource, and with it the file route")
	}
	rows, err := repo.ResourcesForPatches([]int{feedPatch, feedOther})
	if err != nil || len(rows) != 1 || rows[0].ID != feedRes2 {
		t.Errorf("include=resources = %+v (%v), want only the live work's", rows, err)
	}

	mustExec(t, db, `UPDATE patch SET catalog_deleted_at = NULL WHERE id = ?`, feedPatch)
	if _, err := repo.GetResource(feedRes); err != nil {
		t.Errorf("the restored work's resource is still gone: %v", err)
	}
}
