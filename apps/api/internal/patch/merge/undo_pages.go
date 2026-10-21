package merge

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	patchRepo "kun-galgame-patch-api/internal/patch/repository"
)

// restorePages gives the retired work its page back. It answers false when it
// could not, and the child rows then stay where they are.
func (u *undo) restorePages() (bool, error) {
	oldID := int64(u.oldID)
	newExists, err := u.exists("patch", int64(u.newID))
	if err != nil {
		return false, err
	}
	if ok, err := u.exists("patch", oldID); err != nil {
		return false, err
	} else if ok {
		if u.action == ActionRenumber && newExists {
			u.note("撤销前原 id %d 上已新建了页面：资源搬到这个新页面，合并时改号的原页行仍在 %d 上，需人工核对", u.oldID, u.newID)
		} else {
			u.note("撤销前原 id %d 上已新建了页面：资源搬到这个新页面，合并前的原页行没有恢复", u.oldID)
		}
		return true, nil
	}
	if u.action == ActionRenumber && newExists {
		return true, u.takeBackRenumberedPage()
	}
	before := u.find("patch", changeBefore, oldID)
	if before == nil {
		u.note("没有被合并页 %d 的快照，无法恢复该页", u.oldID)
		return false, nil
	}
	switch {
	case u.action == ActionFold && newExists:
		if err := u.restoreSurvivor(before); err != nil {
			return false, err
		}
	case u.action == ActionFold:
		u.note("存续页 %d 已不在（又被合并或删除），它当时加上的计数留在现在的页面上", u.newID)
	}
	return u.reinsertPage(before)
}

// The page at newID is the retired page under the survivor's number. It goes
// back to its own number with everything on it, and whatever was published on
// it after the merge goes to a page of the survivor's own.
func (u *undo) takeBackRenumberedPage() error {
	post := map[string][]int64{}
	for _, table := range childTables {
		recorded := make([]int64, 0)
		for _, r := range u.of(table, changeMoved) {
			recorded = append(recorded, r.RowID)
		}
		q := u.tx.Table(table).Where("galgame_id = ?", u.newID)
		if len(recorded) > 0 {
			q = q.Where("id NOT IN ?", recorded)
		}
		var ids []int64
		if err := q.Pluck("id", &ids).Error; err != nil {
			return fmt.Errorf("find rows added to %s after the merge: %w", table, err)
		}
		if len(ids) > 0 {
			post[table] = ids
		}
	}

	if err := u.tx.Exec(`UPDATE patch SET id = ? WHERE id = ?`, u.oldID, u.newID).Error; err != nil {
		return fmt.Errorf("renumber page %d back to %d: %w", u.newID, u.oldID, err)
	}
	if len(post) == 0 {
		return nil
	}
	if err := u.createSurvivorPage(post); err != nil {
		return err
	}
	for table, ids := range post {
		if err := u.tx.Exec(fmt.Sprintf(`UPDATE %s SET galgame_id = ? WHERE id IN ?`, table), u.newID, ids).Error; err != nil {
			return fmt.Errorf("keep %s published after the merge on %d: %w", table, u.newID, err)
		}
		u.report.count(&u.report.KeptOnSurvivor, table, len(ids))
	}
	return u.tx.Exec(`
		INSERT INTO user_patch_contribute_relation (user_id, galgame_id, updated)
		SELECT DISTINCT user_id, ?::int, now() FROM patch_resource WHERE galgame_id = ?
		ON CONFLICT DO NOTHING`, u.newID, u.newID).Error
}

func (u *undo) createSurvivorPage(post map[string][]int64) error {
	var userID int
	if ids := post["patch_resource"]; len(ids) > 0 {
		u.tx.Raw(`SELECT user_id FROM patch_resource WHERE id IN ? ORDER BY created, id LIMIT 1`, ids).Scan(&userID)
	}
	if ids := post["user_patch_contribute_relation"]; userID == 0 && len(ids) > 0 {
		u.tx.Raw(`SELECT user_id FROM user_patch_contribute_relation WHERE id IN ? ORDER BY id LIMIT 1`, ids).Scan(&userID)
	}
	if userID == 0 {
		u.tx.Raw(`SELECT user_id FROM patch WHERE id = ?`, u.oldID).Scan(&userID)
	}
	vndb := u.survivorVndb
	if taken, err := u.taken("vndb_id", vndb); err != nil {
		return err
	} else if vndb == "" || taken {
		vndb = fmt.Sprintf("wiki-%d", u.newID)
	}
	if err := u.tx.Exec(`
		INSERT INTO patch (id, vndb_id, user_id, is_stub, updated) VALUES (?, ?, ?, true, now())`,
		u.newID, vndb, userID,
	).Error; err != nil {
		return fmt.Errorf("create a page for survivor %d: %w", u.newID, err)
	}
	return nil
}

// restoreSurvivor takes back what the fold added to the survivor: the retired
// page's counters, and each column the fold set that nothing has changed since.
func (u *undo) restoreSurvivor(retired json.RawMessage) error {
	before := u.find("patch", changeBefore, int64(u.newID))
	after := u.find("patch", changeAfter, int64(u.newID))
	if before == nil || after == nil {
		u.note("没有存续页 %d 的合并前后快照，其计数与字段保持现状", u.newID)
		return nil
	}
	return u.tx.Exec(`
		WITH b AS (SELECT * FROM jsonb_populate_record(NULL::patch, ?::jsonb)),
		     a AS (SELECT * FROM jsonb_populate_record(NULL::patch, ?::jsonb)),
		     l AS (SELECT * FROM jsonb_populate_record(NULL::patch, ?::jsonb))
		UPDATE patch p SET
			view                 = GREATEST(p.view - l.view, 0),
			download             = GREATEST(p.download - l.download, 0),
			favorite_count       = GREATEST(p.favorite_count - l.favorite_count, 0),
			published            = CASE WHEN p.published = a.published THEN b.published ELSE p.published END,
			status               = CASE WHEN p.status = a.status THEN b.status ELSE p.status END,
			is_stub              = CASE WHEN p.is_stub = a.is_stub THEN b.is_stub ELSE p.is_stub END,
			creator_id           = CASE WHEN p.creator_id IS NOT DISTINCT FROM a.creator_id THEN b.creator_id ELSE p.creator_id END,
			release_date         = CASE WHEN p.release_date IS NOT DISTINCT FROM a.release_date THEN b.release_date ELSE p.release_date END,
			created              = CASE WHEN p.created = a.created THEN b.created ELSE p.created END,
			resource_update_time = CASE WHEN p.resource_update_time = a.resource_update_time THEN b.resource_update_time ELSE p.resource_update_time END,
			vndb_id              = CASE WHEN p.vndb_id = a.vndb_id THEN b.vndb_id ELSE p.vndb_id END,
			bangumi_id           = CASE WHEN p.bangumi_id IS NOT DISTINCT FROM a.bangumi_id THEN b.bangumi_id ELSE p.bangumi_id END
		FROM b, a, l WHERE p.id = ?`, string(before), string(after), string(retired), u.newID).Error
}

// reinsertPage puts the retired page row back as it was. Its VNDB and Bangumi
// ids are unique, and its publisher may have been purged since: a taken id is
// left off, a gone publisher hands the page to the publisher of its resources.
func (u *undo) reinsertPage(before json.RawMessage) (bool, error) {
	var row map[string]any
	if err := json.Unmarshal(before, &row); err != nil {
		return false, fmt.Errorf("read snapshot of page %d: %w", u.oldID, err)
	}
	if v, _ := row["vndb_id"].(string); v != "" {
		if taken, err := u.taken("vndb_id", v); err != nil {
			return false, err
		} else if taken {
			row["vndb_id"] = fmt.Sprintf("wiki-%d", u.oldID)
			u.note("页面 %d 的 vndb_id %s 已被其他页面占用，恢复时改为占位值", u.oldID, v)
		}
	}
	if v, ok := row["bangumi_id"].(float64); ok {
		if taken, err := u.taken("bangumi_id", int(v)); err != nil {
			return false, err
		} else if taken {
			row["bangumi_id"] = nil
			u.note("页面 %d 的 bangumi_id %d 已被其他页面占用，恢复时留空", u.oldID, int(v))
		}
	}
	if uid, _ := row["user_id"].(float64); !u.userExists(int(uid)) {
		heir := u.heir()
		if heir == 0 {
			u.note("页面 %d 的发布者已不在，也找不到可接手的用户，无法恢复该页", u.oldID)
			return false, nil
		}
		row["user_id"] = heir
		u.note("页面 %d 的发布者已被清除，恢复后交给用户 %d", u.oldID, heir)
	}
	body, _ := json.Marshal(row)
	inserted, err := u.insertSnapshot("patch", body)
	if err != nil {
		return false, fmt.Errorf("restore page %d: %w", u.oldID, err)
	}
	return inserted, nil
}

// heir is the publisher of the retired page's earliest resource that still
// exists, else the survivor's publisher: the order the user purge hands a page
// over in.
func (u *undo) heir() int {
	var heir int
	u.tx.Raw(`
		SELECT (r.value->>'user_id')::int FROM patch_merge_record_row r
		WHERE r.record_id = ? AND r.table_name = 'patch_resource' AND r.change = ?
		  AND EXISTS (SELECT 1 FROM "user" WHERE id = (r.value->>'user_id')::int)
		ORDER BY (r.value->>'created')::timestamptz, r.row_id LIMIT 1`,
		u.report.RecordID, changeMoved).Scan(&heir)
	if heir == 0 {
		u.tx.Raw(`SELECT user_id FROM patch WHERE id = ?`, u.newID).Scan(&heir)
	}
	return heir
}

func (u *undo) userExists(id int) bool {
	if id <= 0 {
		return false
	}
	var ok bool
	u.tx.Raw(`SELECT EXISTS (SELECT 1 FROM "user" WHERE id = ?)`, id).Scan(&ok)
	return ok
}

func (u *undo) taken(column string, value any) (bool, error) {
	if value == "" || value == nil {
		return false, nil
	}
	var ok bool
	err := u.tx.Raw(fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM patch WHERE %s = ?)`, column), value).Scan(&ok).Error
	return ok, err
}

// insertSnapshot inserts a row from its snapshot. Only the columns both the
// snapshot and the table have are written, so a column added since takes its
// default instead of the NULL jsonb_populate_record would give it.
func (u *undo) insertSnapshot(table string, value []byte) (bool, error) {
	cols, ok := u.columns[table]
	if !ok {
		if err := u.tx.Raw(`
			SELECT column_name FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ?`, table,
		).Scan(&cols).Error; err != nil {
			return false, err
		}
		u.columns[table] = cols
	}
	var row map[string]json.RawMessage
	if err := json.Unmarshal(value, &row); err != nil {
		return false, err
	}
	names := make([]string, 0, len(row))
	for _, c := range cols {
		if _, ok := row[c]; ok {
			names = append(names, `"`+c+`"`)
		}
	}
	slices.Sort(names)
	list := strings.Join(names, ", ")
	res := u.tx.Exec(fmt.Sprintf(
		`INSERT INTO %[1]s (%[2]s) SELECT %[2]s FROM jsonb_populate_record(NULL::%[1]s, ?::jsonb) ON CONFLICT DO NOTHING`,
		table, list), string(value))
	return res.RowsAffected > 0, res.Error
}

// restoreChildren moves every recorded child row back by primary key and
// re-inserts the ones the fold dropped as duplicates.
func (u *undo) restoreChildren() error {
	for _, table := range childTables {
		for _, r := range u.of(table, changeMoved) {
			if err := u.restoreChild(table, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (u *undo) restoreChild(table string, r recordRow) error {
	var at []int
	if err := u.tx.Raw(fmt.Sprintf(`SELECT galgame_id FROM %s WHERE id = ?`, table), r.RowID).Scan(&at).Error; err != nil {
		return err
	}
	if len(at) == 0 {
		return u.reinsertDropped(table, r)
	}
	switch cur := at[0]; {
	case cur == u.oldID:
		u.report.count(&u.report.MovedBack, table, 1)
		return nil
	case !u.allowed[cur]:
		u.report.list(&u.report.MovedElsewhere, table, r.RowID)
		return nil
	default:
		u.touched[cur] = true
	}

	q := fmt.Sprintf(`UPDATE %s t SET galgame_id = ? WHERE t.id = ?`, table)
	args := []any{u.oldID, r.RowID}
	if peer, ok := uniquePeer[table]; ok {
		q += fmt.Sprintf(` AND NOT EXISTS (
			SELECT 1 FROM %[1]s x WHERE x.galgame_id = ? AND x.%[2]s = t.%[2]s AND x.id <> t.id)`, table, peer)
		args = append(args, u.oldID)
	}
	res := u.tx.Exec(q, args...)
	if res.Error != nil {
		return fmt.Errorf("move %s %d back to %d: %w", table, r.RowID, u.oldID, res.Error)
	}
	if res.RowsAffected == 0 {
		u.report.list(&u.report.Conflicts, table, r.RowID)
		return nil
	}
	u.report.count(&u.report.MovedBack, table, 1)
	return nil
}

// A row the fold dropped as a duplicate comes back from its snapshot. One that
// is simply gone was deleted after the merge, or its owner purged, and stays so.
func (u *undo) reinsertDropped(table string, r recordRow) error {
	if u.find(table, changeDropped, r.RowID) == nil {
		u.report.list(&u.report.DeletedSince, table, r.RowID)
		return nil
	}
	var owner struct {
		UserID *int `json:"user_id"`
	}
	if json.Unmarshal(r.Value, &owner) == nil && owner.UserID != nil && !u.userExists(*owner.UserID) {
		u.report.list(&u.report.DeletedSince, table, r.RowID)
		return nil
	}
	inserted, err := u.insertSnapshot(table, r.Value)
	if err != nil {
		return fmt.Errorf("re-insert %s %d: %w", table, r.RowID, err)
	}
	if !inserted {
		u.report.list(&u.report.Conflicts, table, r.RowID)
		return nil
	}
	u.report.count(&u.report.Reinserted, table, 1)
	return nil
}

// settlePages recounts every page the undo touched. The restored page keeps
// the published flag of its snapshot; the survivor, or the page created for
// it, is published only through MarkIndexed, which a hidden claim still
// refuses: getting resources back is not an unban.
func (u *undo) settlePages() error {
	pages := map[int]bool{u.oldID: true, u.newID: true}
	for id := range u.touched {
		pages[id] = true
	}
	repo := patchRepo.New(u.tx)
	for id := range pages {
		ok, err := u.exists("patch", int64(id))
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if err := Recount(u.tx, id); err != nil {
			return fmt.Errorf("recount %d: %w", id, err)
		}
		if id != u.newID {
			continue
		}
		var resources int
		u.tx.Raw(`SELECT resource_count FROM patch WHERE id = ?`, id).Scan(&resources)
		if resources > 0 {
			if err := repo.MarkIndexed(id); err != nil {
				return fmt.Errorf("publish %d: %w", id, err)
			}
		}
	}
	return nil
}

// restoreRedirects points the rows the merge flattened onto the survivor back
// at the retired id, which names a live work again.
func (u *undo) restoreRedirects() error {
	for _, r := range u.of("patch_redirect", changeRepointed) {
		var v struct {
			Scope string `json:"scope"`
			OldID int    `json:"old_id"`
		}
		if err := json.Unmarshal(r.Value, &v); err != nil {
			return fmt.Errorf("read redirect snapshot: %w", err)
		}
		res := u.tx.Exec(`UPDATE patch_redirect SET new_id = ? WHERE scope = ? AND old_id = ?`, u.oldID, v.Scope, v.OldID)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			u.report.count(&u.report.MovedBack, "patch_redirect", 1)
		}
	}
	return nil
}
