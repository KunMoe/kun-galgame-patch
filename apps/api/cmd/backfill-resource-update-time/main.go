// cmd/backfill-resource-update-time repairs the historical pollution of
// patch.resource_update_time — the sort key behind GET /api/galgame's default
// "最近更新". Old write paths stamped it wrong (autoCreateTime on lazy-created
// rows = "被点开那一刻"; resource edits didn't bump it), and the code fix only
// governs FUTURE writes, so existing rows stay wrong until this one-time
// backfill resets them to the real content time.
//
// Two disjoint buckets (partitioned by "has ≥1 visible resource"):
//
//   BUCKET 1 — games WITH a status=0 resource. Local truth exists:
//     resource_update_time = MAX over the game's visible resources of
//     GREATEST(created, update_time) — i.e. the latest publish OR edit.
//     (update_time is the canonical edit time; download/like live on `updated`
//     and must NOT count.) Pure SQL, no Wiki. Recomputed from current rows, so
//     a deleted newest resource correctly pulls the time DOWN to the latest
//     remaining one.
//
//   BUCKET 2 — games with ZERO visible resource (the main "浏览即顶" pollution:
//     a galgame merely opened on moyu got a lazy patch row). These have NO
//     local truth — created == resource_update_time == 被点开那一刻, and
//     release_date is NULL. Pull the real time from Wiki, exactly like the
//     fixed ensureLocalPatch does for new lazy rows (GalgameBatch →
//     brief.resource_update_time, RFC3339). Rows Wiki can't resolve are left
//     untouched.
//
// Writes touch ONLY resource_update_time (raw UPDATE / UpdateColumn) — never
// `updated` — so the backfill doesn't make every row look freshly updated.
//
// Usage:
//
//	go run ./cmd/backfill-resource-update-time -dry-run   # counts only, no write
//	go run ./cmd/backfill-resource-update-time            # apply both buckets
//	go run ./cmd/backfill-resource-update-time -batch-size=100 -concurrency=8
//
// Bucket 1 is offline (DB only); BUCKET 2 needs the Wiki API ONLINE — run it
// after services are up. Production data change: run -dry-run first.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/infrastructure/database"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/logger"

	"github.com/joho/godotenv"
)

// bucket1Select counts the rows BUCKET 1 would change (dry-run preview).
const bucket1Select = `
SELECT count(*)
FROM patch p
JOIN (
  SELECT pr.galgame_id AS id, MAX(GREATEST(pr.created, pr.update_time)) AS t
  FROM patch_resource pr
  WHERE pr.status = 0
  GROUP BY pr.galgame_id
) ct ON ct.id = p.id
WHERE p.resource_update_time IS DISTINCT FROM ct.t`

// bucket1Update sets resource_update_time = latest content time for every game
// that has at least one visible resource. IS DISTINCT FROM keeps it a no-op for
// already-correct rows and is NULL-safe.
const bucket1Update = `
UPDATE patch p
SET resource_update_time = ct.t
FROM (
  SELECT pr.galgame_id AS id, MAX(GREATEST(pr.created, pr.update_time)) AS t
  FROM patch_resource pr
  WHERE pr.status = 0
  GROUP BY pr.galgame_id
) ct
WHERE p.id = ct.id
  AND p.resource_update_time IS DISTINCT FROM ct.t`

func main() {
	_ = godotenv.Load()

	dryRun := flag.Bool("dry-run", false, "只统计，不写库")
	batchSize := flag.Int("batch-size", 100, "每次 GalgameBatch 请求的 id 数 (bucket 2)")
	concurrency := flag.Int("concurrency", 8, "并发 batch 请求数 (bucket 2)")
	flag.Parse()

	cfg := config.Load()
	logger.Init(cfg.Server.Mode)
	db := database.NewPostgres(cfg.Database, cfg.Server.Mode)
	wiki := galgameClient.New(cfg.GalgameWiki.BaseURL)
	ctx := context.Background()

	// ── BUCKET 1: games WITH ≥1 visible resource (pure SQL) ──────────────
	if *dryRun {
		var n int64
		if err := db.Raw(bucket1Select).Scan(&n).Error; err != nil {
			slog.Error("bucket1 预览失败", "error", err)
			os.Exit(1)
		}
		slog.Info("[dry-run] bucket1 (有资源) 将更新", "rows", n)
	} else {
		res := db.Exec(bucket1Update)
		if res.Error != nil {
			slog.Error("bucket1 更新失败", "error", res.Error)
			os.Exit(1)
		}
		slog.Info("bucket1 (有资源) 已更新", "rows", res.RowsAffected)
	}

	// ── BUCKET 2: games with ZERO visible resource (Wiki sync) ───────────
	var ids []int
	if err := db.Model(&patchModel.Patch{}).
		Where(`NOT EXISTS (SELECT 1 FROM patch_resource pr WHERE pr.galgame_id = patch.id AND pr.status = 0)`).
		Order("id").Pluck("id", &ids).Error; err != nil {
		slog.Error("拉取 0 资源 patch id 失败", "error", err)
		os.Exit(1)
	}
	slog.Info("bucket2 (无资源) 候选", "count", len(ids), "dry_run", *dryRun)

	chunks := make([][]int, 0, len(ids)/(*batchSize)+1)
	for i := 0; i < len(ids); i += *batchSize {
		chunks = append(chunks, ids[i:min(i+*batchSize, len(ids))])
	}

	var updated, noWikiTime, missing, failed atomic.Int64
	sem := make(chan struct{}, *concurrency)
	var wg sync.WaitGroup

	for _, chunk := range chunks {
		wg.Add(1)
		sem <- struct{}{}
		go func(chunk []int) {
			defer wg.Done()
			defer func() { <-sem }()

			// content_limit="" → include NSFW; resource_update_time is neutral
			// metadata, same call ensureLocalPatch makes.
			briefs, err := wiki.GalgameBatch(ctx, chunk, "")
			if err != nil {
				failed.Add(int64(len(chunk)))
				slog.Warn("GalgameBatch 失败", "ids", len(chunk), "error", err)
				return
			}
			// Wiki may omit ids it no longer publishes — those rows keep their
			// current (polluted) value; count them so the gap is visible.
			missing.Add(int64(len(chunk) - len(briefs)))

			for i := range briefs {
				t, pErr := time.Parse(time.RFC3339, briefs[i].ResourceUpdateTime)
				if pErr != nil {
					noWikiTime.Add(1)
					continue
				}
				if *dryRun {
					updated.Add(1)
					continue
				}
				// UpdateColumn: write only resource_update_time, do NOT trip
				// `updated` (autoUpdateTime).
				if err := db.Model(&patchModel.Patch{}).Where("id = ?", briefs[i].ID).
					UpdateColumn("resource_update_time", t).Error; err != nil {
					slog.Warn("更新 resource_update_time 失败", "id", briefs[i].ID, "error", err)
					failed.Add(1)
					continue
				}
				updated.Add(1)
			}
		}(chunk)
	}
	wg.Wait()

	slog.Info("bucket2 (无资源) 完成",
		"updated", updated.Load(),
		"no_wiki_time", noWikiTime.Load(),
		"missing_from_wiki", missing.Load(),
		"failed", failed.Load(),
		"dry_run", *dryRun)
}
