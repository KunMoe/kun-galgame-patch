package repository

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	patchMerge "kun-galgame-patch-api/internal/patch/merge"

	"gorm.io/gorm"
)

var ErrNoFailedUndo = errors.New("this merge has no failed undo to retry")

// MergeRecord is one catalog merge the merge cron applied (052), with what an
// undo of it did. UndoReport is null until catalog revokes the merge.
type MergeRecord struct {
	ID         int64           `json:"id"`
	OldID      int             `json:"old_id"`
	NewID      int             `json:"new_id"`
	Action     string          `json:"action"`
	AppliedAt  time.Time       `json:"applied_at"`
	UndoneAt   *time.Time      `json:"undone_at"`
	UndoReport json.RawMessage `json:"undo_report"`
	Rows       int             `json:"rows"`
}

// ListMergeRecords pages the records newest first. issuesOnly keeps the undone
// ones whose report lists rows that could not go back.
func (r *AdminRepository) ListMergeRecords(offset, limit int, issuesOnly bool) ([]MergeRecord, int64, error) {
	where := "TRUE"
	if issuesOnly {
		where = `(undo_report->'error' IS NOT NULL OR undo_report->'conflicts' IS NOT NULL
			OR undo_report->'moved_elsewhere' IS NOT NULL OR undo_report->'notes' IS NOT NULL)`
	}
	var total int64
	if err := r.db.Raw(`SELECT count(*) FROM patch_merge_record WHERE ` + where).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []struct {
		MergeRecord
		Report *string
	}
	if err := r.db.Raw(`
		SELECT m.id, m.old_id, m.new_id, m.action, m.applied_at, m.undone_at,
		       m.undo_report::text AS report,
		       (SELECT count(*) FROM patch_merge_record_row x
		        WHERE x.record_id = m.id AND x.change IN ('moved', 'repointed')) AS rows
		FROM patch_merge_record m WHERE `+where+`
		ORDER BY m.id DESC OFFSET ? LIMIT ?`, offset, limit).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]MergeRecord, len(rows))
	for i, row := range rows {
		out[i] = row.MergeRecord
		if row.Report != nil {
			out[i].UndoReport = json.RawMessage(*row.Report)
		}
	}
	return out, total, nil
}

// RetryMergeUndo runs a failed undo again. A second failure is recorded the
// same way and returned.
func (r *AdminRepository) RetryMergeUndo(recordID int64, vndbOf func(int) string) (*patchMerge.UndoReport, error) {
	rec, err := patchMerge.FailedRecord(r.db, recordID)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, ErrNoFailedUndo
	}
	vndb := ""
	if rec.Action == patchMerge.ActionRenumber && vndbOf != nil {
		vndb = vndbOf(rec.NewID)
	}
	var report *patchMerge.UndoReport
	if err := r.db.Transaction(func(tx *gorm.DB) error {
		locked, err := patchMerge.LockFailedRecord(tx, recordID)
		if err != nil {
			return err
		}
		if locked == nil {
			return ErrNoFailedUndo
		}
		report, err = patchMerge.Undo(tx, locked, vndb)
		return err
	}); err != nil {
		if errors.Is(err, ErrNoFailedUndo) || errors.Is(err, patchMerge.ErrAlreadyUndone) {
			return nil, ErrNoFailedUndo
		}
		if merr := patchMerge.MarkFailed(r.db, rec.ID, err); merr != nil {
			return nil, merr
		}
		return nil, err
	}
	return report, nil
}

// purgeMergeSnapshots takes the purged user's rows out of the merge records
// (052), which keep a full copy of every row a merge moved. Their pages follow
// the purge: snapshots of pages it deletes go, so an undo cannot bring a purged
// page back; pages it hands over name the heir.
func purgeMergeSnapshots(tx *gorm.DB, userID int, ownedPagesToo bool, heirUID int) error {
	uid := strconv.Itoa(userID)
	if err := tx.Exec(`
		DELETE FROM patch_merge_record_row
		WHERE table_name IN ('patch_resource', 'patch_comment', 'user_patch_contribute_relation', 'user_patch_favorite_relation')
		  AND value->>'user_id' = ?`, uid).Error; err != nil {
		return err
	}
	if ownedPagesToo {
		return tx.Exec(`DELETE FROM patch_merge_record_row WHERE table_name = 'patch' AND value->>'user_id' = ?`, uid).Error
	}
	return tx.Exec(`
		UPDATE patch_merge_record_row SET value = jsonb_set(value, '{user_id}', to_jsonb(?::int))
		WHERE table_name = 'patch' AND value->>'user_id' = ?`, heirUID, uid).Error
}
