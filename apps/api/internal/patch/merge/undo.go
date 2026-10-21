package merge

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"gorm.io/gorm"
)

// UndoReport is what an undo did, kept on the record for the admin merge page.
// DeletedSince rows were removed by their owner, or their owner purged, after
// the merge and stay gone; Conflicts, MovedElsewhere and Notes are what a
// person has to look at.
type UndoReport struct {
	RecordID       int64              `json:"record_id"`
	Action         Action             `json:"action"`
	MovedBack      map[string]int     `json:"moved_back,omitempty"`
	Reinserted     map[string]int     `json:"reinserted,omitempty"`
	KeptOnSurvivor map[string]int     `json:"kept_on_survivor,omitempty"`
	DeletedSince   map[string][]int64 `json:"deleted_since,omitempty"`
	Conflicts      map[string][]int64 `json:"conflicts,omitempty"`
	MovedElsewhere map[string][]int64 `json:"moved_elsewhere,omitempty"`
	Notes          []string           `json:"notes,omitempty"`
}

func (r *UndoReport) Incomplete() bool {
	return len(r.Conflicts) > 0 || len(r.MovedElsewhere) > 0 || len(r.Notes) > 0
}

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
	tx           *gorm.DB
	oldID        int
	newID        int
	action       Action
	survivorVndb string
	rows         []recordRow
	report       *UndoReport
	// Pages a recorded row may be taken back from: the survivor and every page
	// it was merged into since. A row anywhere else was moved by something
	// else, an earlier undo included, and stays.
	allowed     map[int]bool
	touched     map[int]bool
	createdStub bool
	columns     map[string][]string
}

// Record is one applied merge an undo can put back.
type Record struct {
	ID     int64
	OldID  int
	NewID  int
	Action Action
}

// OpenRecords are the merges of oldID not undone yet, newest first. None: the
// merge predates the records (052) or was undone already.
func OpenRecords(db *gorm.DB, oldID int) ([]Record, error) {
	var recs []Record
	if err := db.Raw(`
		SELECT id, old_id, new_id, action FROM patch_merge_record
		WHERE old_id = ? AND undone_at IS NULL ORDER BY id DESC`, oldID,
	).Scan(&recs).Error; err != nil {
		return nil, fmt.Errorf("find merge records: %w", err)
	}
	return recs, nil
}

// FailedRecord is an open record whose last undo failed, by id: the only kind
// the admin page may run again. A merge catalog has not undone is never one.
func FailedRecord(db *gorm.DB, id int64) (*Record, error) {
	return findRecord(db, `id = ? AND undo_report->'error' IS NOT NULL`, id, "")
}

// LockFailedRecord is FailedRecord under a row lock held to the end of tx. A
// second retry waits on it and then finds the record undone.
func LockFailedRecord(tx *gorm.DB, id int64) (*Record, error) {
	return findRecord(tx, `id = ? AND undo_report->'error' IS NOT NULL`, id, " FOR UPDATE")
}

var ErrAlreadyUndone = errors.New("merge record already undone")

func findRecord(db *gorm.DB, where string, arg any, lock string) (*Record, error) {
	var rec Record
	if err := db.Raw(`
		SELECT id, old_id, new_id, action FROM patch_merge_record
		WHERE `+where+` AND undone_at IS NULL ORDER BY id DESC LIMIT 1`+lock, arg,
	).Scan(&rec).Error; err != nil {
		return nil, fmt.Errorf("find merge record: %w", err)
	}
	if rec.ID == 0 {
		return nil, nil
	}
	return &rec, nil
}

// Undo puts back what rec moved, inside tx. survivorVndb is the survivor's
// VNDB id for a page it may need again, resolved before the transaction so no
// catalog call holds the locks; "" falls back to a placeholder.
func Undo(tx *gorm.DB, rec *Record, survivorVndb string) (*UndoReport, error) {
	var open []bool
	if err := tx.Raw(
		`SELECT undone_at IS NULL FROM patch_merge_record WHERE id = ? FOR UPDATE`, rec.ID,
	).Scan(&open).Error; err != nil {
		return nil, fmt.Errorf("lock merge record: %w", err)
	}
	if len(open) == 0 || !open[0] {
		return nil, ErrAlreadyUndone
	}
	u := &undo{tx: tx, oldID: rec.OldID, newID: rec.NewID, action: rec.Action,
		survivorVndb: survivorVndb, touched: map[int]bool{}, columns: map[string][]string{},
		report: &UndoReport{RecordID: rec.ID, Action: rec.Action}}
	if err := tx.Raw(`
		SELECT table_name, row_id, change, value FROM patch_merge_record_row
		WHERE record_id = ? ORDER BY id`, rec.ID,
	).Scan(&u.rows).Error; err != nil {
		return nil, fmt.Errorf("load merge record rows: %w", err)
	}
	if err := u.loadAllowed(); err != nil {
		return nil, err
	}

	if rec.Action != ActionLedger {
		restored, err := u.restorePages()
		if err != nil {
			return nil, err
		}
		if restored {
			if err := u.restoreChildren(); err != nil {
				return nil, err
			}
			if err := u.settlePages(); err != nil {
				return nil, err
			}
		}
	}
	if err := u.restoreRedirects(); err != nil {
		return nil, err
	}
	for _, id := range []int{rec.OldID, rec.NewID} {
		if err := tx.Exec(`SELECT anchor_presentation_enqueue(1::smallint, ?)`, strconv.Itoa(id)).Error; err != nil {
			return nil, fmt.Errorf("enqueue presentation %d: %w", id, err)
		}
	}
	body, _ := json.Marshal(u.report)
	if err := tx.Exec(
		`UPDATE patch_merge_record SET undone_at = now(), undo_report = ? WHERE id = ?`, string(body), rec.ID,
	).Error; err != nil {
		return nil, fmt.Errorf("close merge record: %w", err)
	}
	return u.report, nil
}

// MarkFailed keeps rec open with the reason its undo failed, for the admin
// page to show and retry.
func MarkFailed(db *gorm.DB, recordID int64, cause error) error {
	return db.Exec(`
		UPDATE patch_merge_record
		SET undo_report = jsonb_build_object('error', ?::text, 'failed_at', now())
		WHERE id = ?`, cause.Error(), recordID).Error
}

// A merge whose undo failed has lost its redirect but not its effect, so the
// chain follows the failed record as well as the redirect.
func (u *undo) loadAllowed() error {
	u.allowed = map[int]bool{u.newID: true}
	queue := []int{u.newID}
	for len(queue) > 0 {
		at := queue[0]
		queue = queue[1:]
		var next []int
		if err := u.tx.Raw(`
			SELECT new_id FROM patch_redirect WHERE scope = 'merge' AND old_id = ?
			UNION
			SELECT new_id FROM patch_merge_record
			WHERE old_id = ? AND undone_at IS NULL AND undo_report->'error' IS NOT NULL`, at, at,
		).Scan(&next).Error; err != nil {
			return fmt.Errorf("follow merges of %d: %w", at, err)
		}
		for _, n := range next {
			if !u.allowed[n] {
				u.allowed[n] = true
				queue = append(queue, n)
			}
		}
	}
	return nil
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
