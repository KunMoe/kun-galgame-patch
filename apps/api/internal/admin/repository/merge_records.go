package repository

import (
	"encoding/json"
	"time"
)

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
		where = "(undo_report->'conflicts' IS NOT NULL OR undo_report->'notes' IS NOT NULL)"
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
