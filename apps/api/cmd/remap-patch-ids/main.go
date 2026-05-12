// cmd/remap-patch-ids realigns moyu's local patch.id with the Wiki galgame.id
// for every patch whose vndb_id resolves on Wiki.
//
// Why
// ----
// 1 patch corresponds to exactly 1 galgame, identified by vndb_id. The local
// patch.id has historically been an auto-increment unrelated to galgame_id —
// this duplicates state. After this migration:
//
//   - patch.id = galgame.id (looked up via Wiki /galgame/check?vndb_id=...)
//   - patch.galgame_id column is dropped (redundant with id)
//   - child FK columns are renamed: patch_resource.patch_id → galgame_id, etc.
//
// What
// ----
// 1. Read every patch row.
// 2. For each, call Wiki /galgame/check?vndb_id=... to get the target id.
//    Rows with vndb_id="pending-N" or whose vndb_id Wiki doesn't recognize
//    are orphans — they're left untouched and the script aborts if any of
//    their current ids collide with a target id (use --allow-orphan-collision
//    only if you understand what that does).
// 3. In a single transaction, two-pass remap (offset → final) for patch.id
//    and the 5 child FK columns. Triggers are temporarily disabled so FK
//    constraints don't reject mid-pass states.
// 4. Drop patch.galgame_id, rename child patch_id columns to galgame_id,
//    reset patch_id_seq.
//
// Usage
// -----
//
//	go run ./cmd/remap-patch-ids                    # actually run
//	go run ./cmd/remap-patch-ids -dry-run           # plan only, no write
//	go run ./cmd/remap-patch-ids -concurrency=8     # parallel Wiki lookups
//	go run ./cmd/remap-patch-ids -orphans-out=...   # write orphan list to file
//
// The DB DSN is read from KUN_DATABASE_URL (same as the API server).
//
// Orphans
// -------
// Patches whose vndb_id is empty / `pending-N` / not found on Wiki are not
// touched (their patch.id stays at the legacy auto-increment value). The full
// list is written to a TSV file (default `orphans-<timestamp>.txt`) so the
// admin can manually rebind via /admin/orphans afterwards. Game names are not
// available locally (per D12 they live on Wiki); the file therefore lists the
// patch id, current vndb_id, publisher, counts and creation time as the most
// actionable identifying signals.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/infrastructure/database"
	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/logger"
	"kun-galgame-patch-api/pkg/userclient"

	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

// Tables that hold a `patch_id` FK and need both:
//   1. Two-pass remap of values (so FKs follow patch.id)
//   2. Post-remap rename of the column to `galgame_id`
var childTables = []string{
	"patch_resource",
	"patch_comment",
	"patch_link",
	"user_patch_favorite_relation",
	"user_patch_contribute_relation",
}

// offset moves IDs into a non-overlapping range during pass 1. Galgame IDs
// from Wiki are <= ~6 digits; 10^9 is safely beyond that.
const offset int64 = 1_000_000_000

// patchRow carries everything we read from the patch table — the lookup keys
// plus enough context for the orphan report (publisher, counts, created).
//
// publisher_name used to come from a JOIN to "user".name, but after migration
// 005 (user-table slim) name lives on OAuth, not locally. The orphan report
// now batch-resolves names via pkg/userclient at write time.
type patchRow struct {
	ID              int
	VndbID          string
	UserID          int
	ResourceCount   int
	CommentCount    int
	FavoriteCount   int
	ContributeCount int
	View            int
	Download        int
	Created         time.Time
}

type mapping struct {
	OldID  int
	NewID  int
	VndbID string
}

type skip struct {
	Row    patchRow
	Reason string
}

func main() {
	_ = godotenv.Load()

	dryRun := flag.Bool("dry-run", false, "只打印计划，不写库")
	concurrency := flag.Int("concurrency", 4, "并发度（同时 in-flight 的 Wiki 请求数）")
	allowOrphanCollision := flag.Bool("allow-orphan-collision", false,
		"允许 orphan 的旧 id 与某个 new_id 撞值时仍然继续（默认 abort）")
	defaultOrphansFile := fmt.Sprintf("orphans-%s.txt", time.Now().Format("20060102-150405"))
	orphansOut := flag.String("orphans-out", defaultOrphansFile,
		"orphan 报告输出路径（TSV）；传空字符串则不写")
	flag.Parse()

	cfg := config.Load()
	logger.Init(cfg.Server.Mode)

	db := database.NewPostgres(cfg.Database, cfg.Server.Mode)
	wiki := galgameClient.New(cfg.GalgameWiki.BaseURL)
	users := userclient.New(userclient.Config{
		BaseURL:      cfg.OAuth.ServerURL,
		ClientID:     cfg.OAuth.ClientID,
		ClientSecret: cfg.OAuth.ClientSecret,
	})

	ctx := context.Background()

	// ── Step 0: idempotency check ─────────────────────────
	// If a previous successful run already renamed child.patch_id → galgame_id
	// (and dropped patch.galgame_id), this script has nothing to do. Bail out
	// cleanly instead of letting pass 1 crash with "column patch_id does not exist".
	state, err := schemaState(db)
	if err != nil {
		slog.Error("schema state check failed", "error", err)
		os.Exit(1)
	}
	switch state {
	case schemaDone:
		fmt.Println("✅ patch.id == galgame.id alignment already complete (child.galgame_id present, no patch_id). Nothing to do.")
		return
	case schemaMixed:
		slog.Error("schema is in a mixed state (some child tables have patch_id, others galgame_id); refuse to proceed",
			"hint", "restore from a backup taken before the previous run, or fix manually")
		os.Exit(1)
	case schemaUnknown:
		slog.Error("could not determine schema state (expected child tables not found)")
		os.Exit(1)
	}
	// state == schemaPending → proceed.

	// ── Step 1: read every patch row ──
	// publisher_name is no longer JOINed in — after migration 005 it lives on
	// OAuth. The orphan report enriches it via userclient at write time.
	var rows []patchRow
	if err := db.Table("patch AS p").
		Select(`p.id, p.vndb_id, p.user_id,
			p.resource_count, p.comment_count, p.favorite_count, p.contribute_count,
			p.view, p.download, p.created`).
		Order("p.id ASC").Scan(&rows).Error; err != nil {
		slog.Error("读取 patch 列表失败", "error", err)
		os.Exit(1)
	}
	if len(rows) == 0 {
		fmt.Println("patch 表为空，无需迁移。")
		return
	}
	slog.Info("读取 patch 完成", "total", len(rows))

	// ── Step 2: parallel Wiki lookup, partition into mappings vs skipped ────
	mappings, skipped := resolveMappings(ctx, wiki, rows, *concurrency)

	// ── Step 3: validate ──────────────────────────────────────
	// 3a. Two patches mapping to the same galgame_id (would only happen if the
	//     local patch table somehow has duplicate vndb_ids — defended against
	//     via a unique index but worth a tripwire).
	byNew := map[int]int{}
	for _, m := range mappings {
		if prev, ok := byNew[m.NewID]; ok {
			slog.Error("致命：两个 patch 映射到同一个 galgame_id",
				"new_id", m.NewID, "old_id_a", prev, "old_id_b", m.OldID)
			os.Exit(1)
		}
		byNew[m.NewID] = m.OldID
	}

	// 3b. Skipped rows (orphans) keep their original patch.id. If any of those
	//     original ids equals a new_id we're about to assign, the migration
	//     would collide. Detect and abort by default.
	skippedIDs := map[int]struct{}{}
	for _, s := range skipped {
		skippedIDs[s.Row.ID] = struct{}{}
	}
	collisions := []int{}
	for _, m := range mappings {
		if _, ok := skippedIDs[m.NewID]; ok {
			collisions = append(collisions, m.NewID)
		}
	}
	if len(collisions) > 0 {
		slog.Error("致命：要写入的 new_id 与某个 orphan 的当前 id 撞了",
			"collision_ids", collisions,
			"hint", "先到 /admin/orphans 处理这些 orphan，或加 --allow-orphan-collision（不推荐）")
		if !*allowOrphanCollision {
			os.Exit(1)
		}
	}

	fmt.Printf("\n=== 迁移计划 ===\n")
	fmt.Printf("将 remap %d 行（patch.id ← Wiki galgame.id）\n", len(mappings))
	fmt.Printf("跳过 %d 行（orphan / pending / Wiki 不存在）\n", len(skipped))
	fmt.Printf("dry_run = %v\n\n", *dryRun)

	if len(skipped) > 0 {
		fmt.Println("跳过的行（保留原 id）：")
		for i, s := range skipped {
			if i >= 20 {
				fmt.Printf("  ... 还有 %d 条（详见 %s）\n", len(skipped)-20, *orphansOut)
				break
			}
			fmt.Printf("  patch.id=%-8d vndb=%-15s  reason=%s\n", s.Row.ID, s.Row.VndbID, s.Reason)
		}
		fmt.Println()

		// Persist the full list to a TSV file for offline triage.
		if *orphansOut != "" {
			if err := writeOrphansFile(ctx, *orphansOut, skipped, users); err != nil {
				slog.Error("写 orphan 报告失败", "path", *orphansOut, "error", err)
			} else {
				fmt.Printf("📝 orphan 报告已写入 %s（%d 条）\n\n", *orphansOut, len(skipped))
			}
		}
	}

	if *dryRun {
		fmt.Println("[dry-run] 不会写库。")
		return
	}

	// ── Step 4: actual remap in a single transaction ─────────
	if err := runRemap(db, mappings); err != nil {
		slog.Error("迁移失败", "error", err)
		os.Exit(1)
	}

	fmt.Println("\n✅ 完成。请记得重启 apps/api 让新 schema 生效。")
}

// resolveMappings hits Wiki concurrently and returns the partition.
func resolveMappings(
	ctx context.Context,
	wiki *galgameClient.Client,
	rows []patchRow,
	concurrency int,
) ([]mapping, []skip) {
	type result struct {
		row    patchRow
		newID  int
		reason string
	}

	jobs := make(chan patchRow)
	results := make(chan result)
	var wg sync.WaitGroup
	var done atomic.Int64
	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := range jobs {
				if r.VndbID == "" || strings.HasPrefix(r.VndbID, "pending-") {
					results <- result{row: r, reason: "pending_or_empty_vndb"}
					continue
				}
				exists, gid, err := wiki.CheckGalgameByVndbID(ctx, r.VndbID)
				if err != nil {
					results <- result{row: r, reason: "wiki_error: " + err.Error()}
					continue
				}
				if !exists || gid <= 0 {
					results <- result{row: r, reason: "wiki_not_found"}
					continue
				}
				results <- result{row: r, newID: gid}
			}
		}()
	}

	go func() {
		for _, r := range rows {
			jobs <- r
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	mappings := make([]mapping, 0, len(rows))
	skipped := make([]skip, 0)
	for r := range results {
		if r.newID > 0 {
			mappings = append(mappings, mapping{
				OldID: r.row.ID, NewID: r.newID, VndbID: r.row.VndbID,
			})
		} else {
			skipped = append(skipped, skip{
				Row: r.row, Reason: r.reason,
			})
		}
		n := done.Add(1)
		if n%500 == 0 {
			slog.Info("Wiki 查询进度", "done", n, "total", len(rows),
				"elapsed", time.Since(start).Round(time.Second))
		}
	}
	slog.Info("Wiki 查询完成",
		"total", len(rows), "mapped", len(mappings), "skipped", len(skipped),
		"elapsed", time.Since(start).Round(time.Second))
	return mappings, skipped
}

// runRemap performs the actual two-pass remap + schema fixups in one tx.
func runRemap(db *gorm.DB, mappings []mapping) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// Disable triggers so FK constraints don't reject mid-pass values.
		// Includes patch itself and every child table.
		allTables := append([]string{"patch"}, childTables...)
		for _, t := range allTables {
			if err := tx.Exec(fmt.Sprintf(`ALTER TABLE %q DISABLE TRIGGER ALL`, t)).Error; err != nil {
				return fmt.Errorf("disable triggers on %s: %w", t, err)
			}
		}

		// Build a temp mapping table _id_map(old_id, new_id) for batch UPDATEs.
		// Temp tables auto-drop at tx end.
		if err := tx.Exec(`CREATE TEMP TABLE _id_map (old_id INT PRIMARY KEY, new_id INT NOT NULL UNIQUE) ON COMMIT DROP`).Error; err != nil {
			return fmt.Errorf("create temp map: %w", err)
		}
		// Bulk-insert in chunks to keep query size bounded.
		const chunk = 500
		for i := 0; i < len(mappings); i += chunk {
			end := i + chunk
			if end > len(mappings) {
				end = len(mappings)
			}
			placeholders := make([]string, 0, end-i)
			args := make([]any, 0, (end-i)*2)
			for _, m := range mappings[i:end] {
				placeholders = append(placeholders, "(?, ?)")
				args = append(args, m.OldID, m.NewID)
			}
			sql := `INSERT INTO _id_map (old_id, new_id) VALUES ` + strings.Join(placeholders, ",")
			if err := tx.Exec(sql, args...).Error; err != nil {
				return fmt.Errorf("insert map chunk: %w", err)
			}
		}

		// ── Pass 1: shift to offset range ─────────────────────
		// Update child FK columns first (they currently reference old patch.id)
		for _, t := range childTables {
			sql := fmt.Sprintf(`UPDATE %q SET patch_id = patch_id + ?
				FROM _id_map WHERE %q.patch_id = _id_map.old_id`, t, t)
			if err := tx.Exec(sql, offset).Error; err != nil {
				return fmt.Errorf("pass1 %s: %w", t, err)
			}
		}
		// Then move patch.id itself
		if err := tx.Exec(`UPDATE patch SET id = id + ?
			FROM _id_map WHERE patch.id = _id_map.old_id`, offset).Error; err != nil {
			return fmt.Errorf("pass1 patch.id: %w", err)
		}

		// ── Pass 2: from offset → final new_id ────────────────
		for _, t := range childTables {
			sql := fmt.Sprintf(`UPDATE %q SET patch_id = _id_map.new_id
				FROM _id_map WHERE %q.patch_id = _id_map.old_id + ?`, t, t)
			if err := tx.Exec(sql, offset).Error; err != nil {
				return fmt.Errorf("pass2 %s: %w", t, err)
			}
		}
		if err := tx.Exec(`UPDATE patch SET id = _id_map.new_id
			FROM _id_map WHERE patch.id = _id_map.old_id + ?`, offset).Error; err != nil {
			return fmt.Errorf("pass2 patch.id: %w", err)
		}

		// ── Schema cleanup ────────────────────────────────────
		// 1. patch.galgame_id is now redundant (== patch.id), drop it.
		if err := tx.Exec(`ALTER TABLE patch DROP COLUMN galgame_id`).Error; err != nil {
			return fmt.Errorf("drop patch.galgame_id: %w", err)
		}
		// 2. Rename FK columns: child.patch_id → galgame_id (the "patch" id is
		//    now the galgame id by definition, so the column name should reflect that).
		for _, t := range childTables {
			if err := tx.Exec(fmt.Sprintf(
				`ALTER TABLE %q RENAME COLUMN patch_id TO galgame_id`, t,
			)).Error; err != nil {
				return fmt.Errorf("rename %s.patch_id: %w", t, err)
			}
		}

		// ── Reset sequence ────────────────────────────────────
		if err := tx.Exec(`SELECT setval(pg_get_serial_sequence('patch', 'id'),
			(SELECT COALESCE(MAX(id), 1) FROM patch))`).Error; err != nil {
			return fmt.Errorf("reset patch_id_seq: %w", err)
		}

		// Re-enable triggers
		for _, t := range allTables {
			if err := tx.Exec(fmt.Sprintf(`ALTER TABLE %q ENABLE TRIGGER ALL`, t)).Error; err != nil {
				return fmt.Errorf("enable triggers on %s: %w", t, err)
			}
		}

		return nil
	})
}

// schemaPhase classifies the current state of the patch / child-table schema
// so the script can decide whether to run, exit clean, or refuse.
type schemaPhase int

const (
	// schemaPending: child tables still have patch_id; safe to run.
	schemaPending schemaPhase = iota
	// schemaDone: every child table already has galgame_id and no patch_id;
	// a previous run completed. Nothing to do.
	schemaDone
	// schemaMixed: some child tables migrated, others not. Bail.
	schemaMixed
	// schemaUnknown: at least one expected child table has neither column.
	schemaUnknown
)

// schemaState inspects every child table and returns the aggregate phase.
func schemaState(db *gorm.DB) (schemaPhase, error) {
	pendingCount, doneCount := 0, 0
	for _, t := range childTables {
		var hasPatchID, hasGalgameID bool
		if err := db.Raw(`SELECT EXISTS(
				SELECT 1 FROM information_schema.columns
				WHERE table_name = ? AND column_name = 'patch_id')`, t).
			Scan(&hasPatchID).Error; err != nil {
			return schemaUnknown, err
		}
		if err := db.Raw(`SELECT EXISTS(
				SELECT 1 FROM information_schema.columns
				WHERE table_name = ? AND column_name = 'galgame_id')`, t).
			Scan(&hasGalgameID).Error; err != nil {
			return schemaUnknown, err
		}
		switch {
		case hasPatchID && !hasGalgameID:
			pendingCount++
		case !hasPatchID && hasGalgameID:
			doneCount++
		case hasPatchID && hasGalgameID:
			return schemaMixed, fmt.Errorf("table %q has BOTH patch_id and galgame_id columns", t)
		default:
			return schemaUnknown, fmt.Errorf("table %q has neither patch_id nor galgame_id column", t)
		}
	}
	if pendingCount == len(childTables) {
		return schemaPending, nil
	}
	if doneCount == len(childTables) {
		return schemaDone, nil
	}
	return schemaMixed, fmt.Errorf("split state: %d pending, %d done", pendingCount, doneCount)
}

// writeOrphansFile dumps every skipped patch to a TSV file. Game names live
// on Wiki and these rows by definition have no Wiki match, so the most useful
// identifying signals are: the publisher's name + the row counts + creation
// time. The admin can use the "open" column (a direct moyu URL) to inspect
// the patch in the browser.
//
// Publisher names are resolved in one batch call to OAuth /users/batch (via
// userclient). If OAuth is unreachable we still write the file with empty
// names — the publisher_uid alone is enough to look up manually.
func writeOrphansFile(ctx context.Context, path string, skipped []skip, users *userclient.Client) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	names := resolveOrphanPublisherNames(ctx, skipped, users)

	header := strings.Join([]string{
		"id", "vndb_id", "reason",
		"resources", "comments", "favorites", "contributors", "view", "download",
		"publisher_uid", "publisher_name", "created", "open",
	}, "\t")
	if _, err := fmt.Fprintln(f, header); err != nil {
		return err
	}

	for _, s := range skipped {
		row := []string{
			fmt.Sprint(s.Row.ID),
			s.Row.VndbID,
			s.Reason,
			fmt.Sprint(s.Row.ResourceCount),
			fmt.Sprint(s.Row.CommentCount),
			fmt.Sprint(s.Row.FavoriteCount),
			fmt.Sprint(s.Row.ContributeCount),
			fmt.Sprint(s.Row.View),
			fmt.Sprint(s.Row.Download),
			fmt.Sprint(s.Row.UserID),
			names[uint(s.Row.UserID)],
			s.Row.Created.Format(time.RFC3339),
			fmt.Sprintf("/patch/%d/introduction", s.Row.ID),
		}
		if _, err := fmt.Fprintln(f, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return nil
}

// resolveOrphanPublisherNames batch-fetches user names from OAuth /users/batch
// for every distinct publisher across the skipped rows. On error it returns
// an empty map: the orphan report still has publisher_uid, which is enough.
func resolveOrphanPublisherNames(ctx context.Context, skipped []skip, users *userclient.Client) map[uint]string {
	idSet := make(map[uint]struct{}, len(skipped))
	for _, s := range skipped {
		if s.Row.UserID > 0 {
			idSet[uint(s.Row.UserID)] = struct{}{}
		}
	}
	if len(idSet) == 0 {
		return map[uint]string{}
	}
	ids := make([]uint, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	briefs, err := users.Users(ctx, ids)
	if err != nil {
		slog.Warn("OAuth /users/batch failed; orphan report will lack publisher_name", "error", err)
		return map[uint]string{}
	}
	out := make(map[uint]string, len(briefs))
	for id, b := range briefs {
		if b != nil {
			out[id] = b.Name
		}
	}
	return out
}
