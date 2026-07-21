// cmd/backfill-release-date mirrors Wiki's galgame.release_date into the local
// patch.release_date column (added in migration 010). This is the one-time
// half of the A-lite sync model: new patches get release_date stamped at
// creation (PatchService.CreatePatch); this command fills in existing rows.
//
// It reads every patch id (or only the NULL ones) and calls Wiki
// GET /galgame/:gid per id (concurrently) to fetch release_date, then UPDATEs
// the local row.
//
// Why per-id GetGalgame and NOT /galgame/batch: the lightweight batch endpoint
// does NOT return release_date — it only carries id / name / banner /
// content_limit / status / user_id / resource_update_time / original_language
// / age_limit. Only the single-detail endpoint (and the list/search endpoints)
// include release_date. GetGalgame is the simplest "only fetch the ids I need"
// option (vs paginating the whole 60k-row wiki to build a map).
//
// content_limit="" (no NSFW filter) so SFW + NSFW games are both backfilled —
// the column is a neutral metadata mirror, not a gated surface.
//
// Usage:
//
//	go run ./cmd/backfill-release-date                  # fill only NULL rows
//	go run ./cmd/backfill-release-date -dry-run         # plan only, no write
//	go run ./cmd/backfill-release-date -only-null=false # refresh ALL rows
//	go run ./cmd/backfill-release-date -concurrency=16  # parallel GetGalgame
//
// Requires the Wiki API server to be ONLINE (HTTP), unlike the offline db
// migration cmds — run it after services are up.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/infrastructure/database"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/logger"
	"kun-galgame-patch-api/pkg/utils"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	dryRun := flag.Bool("dry-run", false, "只打印计划，不写库")
	onlyNull := flag.Bool("only-null", true, "只补 release_date IS NULL 的行（false = 全量刷新，覆盖已有值）")
	concurrency := flag.Int("concurrency", 8, "并发 GetGalgame 请求数")
	flag.Parse()

	cfg := config.Load()
	logger.Init(cfg.Server.Mode)

	db := database.NewPostgres(cfg.Database, cfg.Server.Mode)
	wiki := galgameClient.New(cfg.NextMoeAPI.BaseURL)
	ctx := context.Background()

	var ids []int
	q := db.Model(&patchModel.Patch{})
	if *onlyNull {
		q = q.Where("release_date IS NULL")
	}
	if err := q.Order("id").Pluck("id", &ids).Error; err != nil {
		slog.Error("拉取 patch id 失败", "error", err)
		os.Exit(1)
	}
	slog.Info("backfill release_date 开始",
		"candidates", len(ids), "dry_run", *dryRun, "only_null", *onlyNull, "concurrency", *concurrency)

	var scanned, updated, noDate, failed atomic.Int64
	sem := make(chan struct{}, *concurrency)
	var wg sync.WaitGroup

	for _, id := range ids {
		wg.Add(1)
		sem <- struct{}{}
		go func(id int) {
			defer wg.Done()
			defer func() { <-sem }()

			n := scanned.Add(1)
			if n%500 == 0 {
				slog.Info("进度", "scanned", n, "updated", updated.Load(), "no_date", noDate.Load(), "failed", failed.Load())
			}

			// GetGalgame returns release_date; batch does not. content_limit=""
			// so NSFW games are included.
			env, err := wiki.GetGalgame(ctx, id, "")
			if err != nil || env == nil {
				failed.Add(1)
				return
			}
			if env.Galgame.ReleaseDate == nil {
				noDate.Add(1)
				return
			}
			d := utils.ParseWikiReleaseDate(*env.Galgame.ReleaseDate)
			if d == nil {
				noDate.Add(1)
				return
			}
			if *dryRun {
				updated.Add(1)
				return
			}
			if err := db.Model(&patchModel.Patch{}).Where("id = ?", id).
				Update("release_date", d).Error; err != nil {
				slog.Warn("更新 release_date 失败", "id", id, "error", err)
				failed.Add(1)
				return
			}
			updated.Add(1)
		}(id)
	}
	wg.Wait()

	slog.Info("backfill release_date 完成",
		"scanned", scanned.Load(), "updated", updated.Load(),
		"no_date", noDate.Load(), "failed", failed.Load())
}
