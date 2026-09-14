package cron

import (
	"context"
	"fmt"

	patchMerge "kun-galgame-patch-api/internal/patch/merge"
	"kun-galgame-patch-api/pkg/catalogv2"

	"gorm.io/gorm"
)

const (
	// Its own cursor namespace. The changes feed pages by cur_ strings over
	// catalog_work and the claim feed by an integer event id; this one keysets
	// over (merged_at, entity_type, old_id) and shares a watermark with
	// neither.
	mergeSyncCronName = "catalog_merge_sync"
	// Staggered off the claim sync's */10 and the mirror's 3-59/10 so the three
	// do not queue behind each other on the same catalog origin.
	mergeSyncSchedule = "7-59/10 * * * *"
	mergeSyncPageSize = catalogv2.RedirectsMaxLimit
	// An empty cursor starts at the beginning of catalog's whole merge history
	// -- 4,473 work redirects when this shipped -- and unlike the claim feed
	// there is no seeding at head: a merge from two months ago still names a
	// page this site serves today. 50 pages clears that backlog in one tick.
	mergeSyncMaxPages = 50
)

type redirectAction int

const (
	// Neither number is a page here: the merge is about a work this site never
	// carried.
	redirectSkip redirectAction = iota
	// Only the ledger moves. Nothing sits on the retired id, but the survivor
	// is a page, so /galgame/<old> can 301 instead of answering 404.
	redirectLedger
	// The page follows its work: nothing occupies the survivor's number yet.
	redirectRenumber
	// Both numbers are pages, so the retired one folds into the survivor.
	redirectFold
)

// The survivor having no page is not a reason to record the move. A merge row
// makes /galgame/<old> 301, and 301ing onto a 404 is worse than the 404 the id
// already answers.
func actionFor(oldIsPage, newIsPage bool) redirectAction {
	switch {
	case oldIsPage && newIsPage:
		return redirectFold
	case oldIsPage:
		return redirectRenumber
	case newIsPage:
		return redirectLedger
	default:
		return redirectSkip
	}
}

// MergeSyncReport is what one drain did, split by what each redirect asked
// for. A Folded or Renumbered above zero means a page this site serves changed
// number. Unchanged is the tail the next tick always re-reads: a short page
// carries no cursor to advance past, so the settled state replays its last few
// rows forever and only Applied() tells a real change from that churn.
type MergeSyncReport struct {
	Scanned    int
	Folded     int
	Renumbered int
	Ledger     int
	Unchanged  int
	Skipped    int
}

func (r MergeSyncReport) Applied() int { return r.Folded + r.Renumbered + r.Ledger }

// RunCatalogMergeSync follows catalog's work merges. patch.id IS the catalog
// work id, so a work merged away leaves its page on a number that stopped
// naming anything: the page has to move onto the survivor's number, or fold
// into the page already sitting there, and the retired number has to keep
// answering. Catalog erases the claim in the same transaction as the merge, so
// nothing else on this side can work out where the page went.
//
// apply=false drains and decides without writing anything, cursor included, so
// the report says what a real run would do.
func RunCatalogMergeSync(
	ctx context.Context,
	db *gorm.DB,
	catalog *catalogv2.Client,
	apply bool,
) (report MergeSyncReport, caughtUp bool, err error) {
	if db == nil || catalog == nil || !catalog.Configured() {
		return report, false, fmt.Errorf("catalog merge sync: missing db or catalog client")
	}

	cursor, err := readMergeCursor(db)
	if err != nil {
		return report, false, err
	}

	for page := 0; page < mergeSyncMaxPages; page++ {
		feed, ferr := catalog.Redirects(ctx, cursor, mergeSyncPageSize)
		if ferr != nil {
			return report, false, fmt.Errorf("fetch redirects feed: %w", ferr)
		}
		if aerr := applyRedirects(db, feed.Items, apply, &report); aerr != nil {
			return report, false, aerr
		}
		// A short page carries no next cursor, so the watermark stays at the
		// last full page and the tail is re-read next tick. Replaying it is
		// free: every step below is idempotent once the page has moved.
		if feed.NextCursor == "" {
			caughtUp = true
			break
		}
		cursor = feed.NextCursor
		if !apply {
			continue
		}
		if werr := writeMergeCursor(db, cursor); werr != nil {
			return report, false, werr
		}
	}
	return report, caughtUp, nil
}

func applyRedirects(
	db *gorm.DB,
	items []catalogv2.Redirect,
	apply bool,
	report *MergeSyncReport,
) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]int, 0, len(items)*2)
	for i := range items {
		ids = append(ids, int(items[i].OldID), int(items[i].CurrentID))
	}
	// Most of the backlog is about works this site never carried, so both the
	// page and the ledger are settled for the whole batch in one query each,
	// rather than opening a transaction per redirect to find nothing to do.
	pages, err := existingPages(db, ids)
	if err != nil {
		return err
	}
	ledger, err := mergeLedger(db, ids)
	if err != nil {
		return err
	}

	for i := range items {
		oldID, newID := int(items[i].OldID), int(items[i].CurrentID)
		if oldID <= 0 || newID <= 0 || oldID == newID {
			continue
		}
		report.Scanned++
		action := actionFor(pages[oldID], pages[newID])
		if action == redirectLedger && ledger[oldID] == newID {
			report.Unchanged++
			continue
		}
		switch action {
		case redirectSkip:
			report.Skipped++
			continue
		case redirectFold:
			report.Folded++
		case redirectRenumber:
			report.Renumbered++
		case redirectLedger:
			report.Ledger++
		}
		if apply {
			if terr := db.Transaction(func(tx *gorm.DB) error {
				return applyRedirect(tx, action, oldID, newID)
			}); terr != nil {
				return fmt.Errorf("apply merge %d -> %d: %w", oldID, newID, terr)
			}
		}
		// A later redirect in the same drain can name either end of this one.
		if action != redirectLedger {
			pages[newID] = true
			delete(pages, oldID)
		}
	}
	return nil
}

func applyRedirect(tx *gorm.DB, action redirectAction, oldID, newID int) error {
	switch action {
	case redirectRenumber:
		// The children follow on their own -- migration 037 made every foreign
		// key ON UPDATE CASCADE. No table lock here: the renumber needed one
		// because it held 10,957 rows for minutes against live view counters,
		// and taking ACCESS EXCLUSIVE every ten minutes for one row would stall
		// the site instead.
		if err := tx.Exec(`UPDATE patch SET id = ? WHERE id = ?`, newID, oldID).Error; err != nil {
			return fmt.Errorf("renumber page: %w", err)
		}
	case redirectFold:
		if err := patchMerge.Fold(tx, oldID, newID); err != nil {
			return fmt.Errorf("fold page: %w", err)
		}
		if err := patchMerge.Recount(tx, newID); err != nil {
			return fmt.Errorf("recount: %w", err)
		}
	}

	// Catalog flattens its own chains in place and without touching merged_at,
	// so A -> B never surfaces again after B -> C is merged. A copy that does
	// not flatten at the same moment keeps pointing /galgame/A at a page that
	// stopped existing one merge ago. Both scopes move: a legacy row names the
	// page that took over a pre-037 number, and that page has just moved too.
	if err := tx.Exec(
		`UPDATE patch_redirect SET new_id = ? WHERE new_id = ?`, newID, oldID,
	).Error; err != nil {
		return fmt.Errorf("flatten ledger: %w", err)
	}
	return tx.Exec(`
		INSERT INTO patch_redirect(old_id, new_id, scope) VALUES (?, ?, 'merge')
		ON CONFLICT(scope, old_id) DO UPDATE SET new_id = EXCLUDED.new_id
	`, oldID, newID).Error
}

// mergeLedger is the merge rows already recorded for these ids, so a redirect
// whose row is already right is not rewritten and not counted as work done.
func mergeLedger(db *gorm.DB, ids []int) (map[int]int, error) {
	type row struct{ OldID, NewID int }
	var rows []row
	if err := db.Raw(
		`SELECT old_id, new_id FROM patch_redirect WHERE scope = 'merge' AND old_id IN ?`, ids,
	).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("look up merge ledger: %w", err)
	}
	out := make(map[int]int, len(rows))
	for _, r := range rows {
		out[r.OldID] = r.NewID
	}
	return out, nil
}

func existingPages(db *gorm.DB, ids []int) (map[int]bool, error) {
	var found []int
	if err := db.Raw(`SELECT id FROM patch WHERE id IN ?`, ids).Scan(&found).Error; err != nil {
		return nil, fmt.Errorf("look up pages: %w", err)
	}
	out := make(map[int]bool, len(found))
	for _, id := range found {
		out[id] = true
	}
	return out, nil
}

func readMergeCursor(db *gorm.DB) (string, error) {
	var rows []string
	if err := db.Raw(
		`SELECT COALESCE(last_cursor, '') FROM cron_state WHERE name = ?`, mergeSyncCronName,
	).Scan(&rows).Error; err != nil {
		return "", fmt.Errorf("read merge cursor: %w", err)
	}
	if len(rows) == 0 {
		return "", nil
	}
	return rows[0], nil
}

func writeMergeCursor(db *gorm.DB, cursor string) error {
	return db.Exec(`
		INSERT INTO cron_state(name, last_id, last_cursor, updated_at)
		VALUES (?, 0, ?, NOW())
		ON CONFLICT(name) DO UPDATE
		SET last_cursor = EXCLUDED.last_cursor, updated_at = EXCLUDED.updated_at
	`, mergeSyncCronName, cursor).Error
}
