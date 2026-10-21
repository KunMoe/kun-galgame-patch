package merge

import (
	"fmt"

	"gorm.io/gorm"
)

// Action is what the catalog merge cron did with a merged work's page.
type Action string

const (
	// The retired page took the survivor's number; nothing sat there yet.
	ActionRenumber Action = "renumber"
	// Both numbers were pages; the retired one was folded in and deleted.
	ActionFold Action = "fold"
	// Only the survivor was a page; only redirect rows moved.
	ActionLedger Action = "ledger"
)

// Every table that hangs rows off a page by galgame_id.
var childTables = []string{
	"patch_resource",
	"patch_comment",
	"patch_link",
	"user_patch_contribute_relation",
	"user_patch_favorite_relation",
}

const (
	changeBefore    = "before"
	changeAfter     = "after"
	changeMoved     = "moved"
	changeDropped   = "dropped"
	changeRepointed = "repointed"
)

// Begin opens the record of a merge of oldID into newID and snapshots every row
// it is about to touch. Call it inside the merge's transaction, before the
// merge; Finish closes it once the merge is applied.
func Begin(tx *gorm.DB, action Action, oldID, newID int) (int64, error) {
	var recordID int64
	if err := tx.Raw(
		`INSERT INTO patch_merge_record (old_id, new_id, action) VALUES (?, ?, ?) RETURNING id`,
		oldID, newID, action,
	).Scan(&recordID).Error; err != nil {
		return 0, fmt.Errorf("open merge record: %w", err)
	}
	if err := snapshot(tx, recordID, "patch", changeBefore, "id IN ?", []int{oldID, newID}); err != nil {
		return 0, err
	}
	for _, table := range childTables {
		if err := snapshot(tx, recordID, table, changeMoved, "galgame_id = ?", oldID); err != nil {
			return 0, err
		}
	}
	if err := tx.Exec(`
		INSERT INTO patch_merge_record_row (record_id, table_name, row_id, change, value)
		SELECT ?, 'patch_redirect', r.old_id, ?, to_jsonb(r) FROM patch_redirect r WHERE r.new_id = ?`,
		recordID, changeRepointed, oldID,
	).Error; err != nil {
		return 0, fmt.Errorf("snapshot redirects: %w", err)
	}
	return recordID, nil
}

// Finish records what the merge left behind: the survivor page as it now
// stands, and the child rows a fold dropped because the survivor already had
// their twin.
func Finish(tx *gorm.DB, recordID int64, newID int) error {
	if err := snapshot(tx, recordID, "patch", changeAfter, "id = ?", newID); err != nil {
		return err
	}
	for _, table := range childTables {
		if err := tx.Exec(fmt.Sprintf(`
			INSERT INTO patch_merge_record_row (record_id, table_name, row_id, change, value)
			SELECT m.record_id, m.table_name, m.row_id, '%s', m.value
			FROM patch_merge_record_row m
			WHERE m.record_id = ? AND m.table_name = '%s' AND m.change = '%s'
			  AND NOT EXISTS (SELECT 1 FROM %s t WHERE t.id = m.row_id)`,
			changeDropped, table, changeMoved, table), recordID).Error; err != nil {
			return fmt.Errorf("record rows dropped from %s: %w", table, err)
		}
	}
	return nil
}

func snapshot(tx *gorm.DB, recordID int64, table, change, where string, args ...any) error {
	q := fmt.Sprintf(`
		INSERT INTO patch_merge_record_row (record_id, table_name, row_id, change, value)
		SELECT ?, '%s', t.id, '%s', to_jsonb(t) FROM %s t WHERE %s`, table, change, table, where)
	if err := tx.Exec(q, append([]any{recordID}, args...)...).Error; err != nil {
		return fmt.Errorf("snapshot %s: %w", table, err)
	}
	return nil
}
