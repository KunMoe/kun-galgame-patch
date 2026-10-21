package merge

import (
	"encoding/json"
	"fmt"
	"strconv"

	"gorm.io/gorm"
)

// UndoReport is what an undo did, kept on the record for the admin merge page.
// DeletedSince rows were removed by their owner after the merge and stay gone;
// Conflicts and Notes are what a person has to look at.
type UndoReport struct {
	RecordID       int64              `json:"record_id"`
	Action         Action             `json:"action"`
	MovedBack      map[string]int     `json:"moved_back,omitempty"`
	Reinserted     map[string]int     `json:"reinserted,omitempty"`
	KeptOnSurvivor map[string]int     `json:"kept_on_survivor,omitempty"`
	DeletedSince   map[string][]int64 `json:"deleted_since,omitempty"`
	Conflicts      map[string][]int64 `json:"conflicts,omitempty"`
	Notes          []string           `json:"notes,omitempty"`
}

func (r *UndoReport) Incomplete() bool { return len(r.Conflicts) > 0 || len(r.Notes) > 0 }

func (r *UndoReport) count(m *map[string]int, table string, n int) {
	if n == 0 {
		return
	}
	if *m == nil {
		*m = map[string]int{}
	}
	(*m)[table] += n
}

func (r *UndoReport) list(m *map[string][]int64, table string, id int64) {
	if *m == nil {
		*m = map[string][]int64{}
	}
	(*m)[table] = append((*m)[table], id)
}

type recordRow struct {
	TableName string
	RowID     int64
	Change    string
	Value     json.RawMessage
}

// The column besides galgame_id that a child table keeps unique per page.
var uniquePeer = map[string]string{
	"patch_link":                     "name",
	"user_patch_contribute_relation": "user_id",
	"user_patch_favorite_relation":   "user_id",
}

type undo struct {
	tx     *gorm.DB
	oldID  int
	newID  int
	action Action
	rows   []recordRow
	report *UndoReport
}

// Undo puts back what the open merge record for oldID moved. It answers false
// when there is none: the merge predates the records (052) or was undone
// already. vndbOf names a work's VNDB id for a page the survivor needs again,
// and may answer "".
func Undo(tx *gorm.DB, oldID int, vndbOf func(int) string) (*UndoReport, bool, error) {
	var rec struct {
		ID     int64
		NewID  int
		Action Action
	}
	if err := tx.Raw(`
		SELECT id, new_id, action FROM patch_merge_record
		WHERE old_id = ? AND undone_at IS NULL ORDER BY id DESC LIMIT 1`, oldID,
	).Scan(&rec).Error; err != nil {
		return nil, false, fmt.Errorf("find merge record: %w", err)
	}
	if rec.ID == 0 {
		return nil, false, nil
	}
	u := &undo{tx: tx, oldID: oldID, newID: rec.NewID, action: rec.Action,
		report: &UndoReport{RecordID: rec.ID, Action: rec.Action}}
	if err := tx.Raw(`
		SELECT table_name, row_id, change, value FROM patch_merge_record_row
		WHERE record_id = ? ORDER BY id`, rec.ID,
	).Scan(&u.rows).Error; err != nil {
		return nil, false, fmt.Errorf("load merge record rows: %w", err)
	}

	if rec.Action != ActionLedger {
		restored, err := u.restorePages(vndbOf)
		if err != nil {
			return nil, false, err
		}
		if restored {
			if err := u.restoreChildren(); err != nil {
				return nil, false, err
			}
			if err := u.settlePages(); err != nil {
				return nil, false, err
			}
		}
	}
	if err := u.restoreRedirects(); err != nil {
		return nil, false, err
	}
	for _, id := range []int{oldID, rec.NewID} {
		if err := tx.Exec(`SELECT anchor_presentation_enqueue(1::smallint, ?)`, strconv.Itoa(id)).Error; err != nil {
			return nil, false, fmt.Errorf("enqueue presentation %d: %w", id, err)
		}
	}
	body, _ := json.Marshal(u.report)
	if err := tx.Exec(
		`UPDATE patch_merge_record SET undone_at = now(), undo_report = ? WHERE id = ?`, string(body), rec.ID,
	).Error; err != nil {
		return nil, false, fmt.Errorf("close merge record: %w", err)
	}
	return u.report, true, nil
}

func (u *undo) find(table, change string, rowID int64) json.RawMessage {
	for i := range u.rows {
		r := &u.rows[i]
		if r.TableName == table && r.Change == change && r.RowID == rowID {
			return r.Value
		}
	}
	return nil
}

func (u *undo) of(table, change string) []recordRow {
	var out []recordRow
	for _, r := range u.rows {
		if r.TableName == table && r.Change == change {
			out = append(out, r)
		}
	}
	return out
}

func (u *undo) exists(table string, id int64) (bool, error) {
	var ok bool
	err := u.tx.Raw(fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM %s WHERE id = ?)`, table), id).Scan(&ok).Error
	return ok, err
}

func (u *undo) note(format string, args ...any) {
	u.report.Notes = append(u.report.Notes, fmt.Sprintf(format, args...))
}
