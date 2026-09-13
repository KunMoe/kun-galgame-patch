// align-patch-ids renumbers every patch page to the catalog work it shows, so
// that this site's page id and the catalog work id are the same number.
//
// Run it once, during a maintenance window, between migration 037 (which adds
// the redirect ledger and makes the child foreign keys follow a moving parent)
// and the deploy that ships the identity code with migration 038.
//
//	go run ./cmd/align-patch-ids                 # plan only, writes a TSV
//	go run ./cmd/align-patch-ids -apply          # one transaction, then done
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/infrastructure/database"
	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/logger"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	apply := flag.Bool("apply", false, "真正写库（默认只出计划）")
	concurrency := flag.Int("concurrency", 8, "并发解析 catalog 的协程数")
	out := flag.String("out", "", "计划输出路径（TSV），留空则自动按时间命名")
	flag.Parse()

	cfg := config.Load()
	logger.Init(cfg.Server.Mode)

	db := database.NewPostgres(cfg.Database, cfg.Server.Mode)
	v2 := galgameClient.NewWithKey(cfg.NextMoeAPI.BaseURL, cfg.NextMoeAPI.APIKey).V2()

	if err := requireSchema(db); err != nil {
		slog.Error("schema 未就绪", "error", err)
		os.Exit(1)
	}

	rows, err := loadPatches(db)
	if err != nil {
		slog.Error("读取 patch 失败", "error", err)
		os.Exit(1)
	}
	slog.Info("读取 patch 完成", "rows", len(rows))

	ctx := context.Background()
	resolved, rerr := resolveTargets(ctx, v2, rows, *concurrency)
	if rerr != nil {
		slog.Error("解析 catalog work 失败", "error", rerr)
		os.Exit(1)
	}

	plan := buildPlan(rows, resolved)
	path := *out
	if path == "" {
		path = fmt.Sprintf("align-patch-ids-%s.tsv", time.Now().Format("20060102-150405"))
	}
	if werr := writePlan(path, plan); werr != nil {
		slog.Error("写计划文件失败", "error", werr)
		os.Exit(1)
	}
	report(plan, path)

	if !*apply {
		fmt.Println("\n未加 -apply，未写库。")
		return
	}
	if err := applyPlan(db, plan); err != nil {
		slog.Error("改号失败，事务已回滚", "error", err)
		os.Exit(1)
	}
	fmt.Println("\n✅ 改号完成：patch.id 即 catalog work id。")
}

func report(p *Plan, path string) {
	fmt.Printf("\n计划（%s）\n", path)
	fmt.Printf("  patch 行            %6d\n", p.Total)
	fmt.Printf("  已经恒等            %6d\n", p.Identical)
	fmt.Printf("  需要改号            %6d\n", len(p.Moves))
	fmt.Printf("  需要合并的重复页    %6d 组，丢弃 %d 行子表\n", len(p.Folds), p.FoldLosers)
	fmt.Printf("  catalog 认不出      %6d（挪到 %d+ 的本地专用段）\n", p.Parked, localBase)
	fmt.Printf("  写入 patch_redirect %6d\n", len(p.Redirects))
	for _, f := range p.Folds {
		fmt.Printf("    合并 -> work %d: 保留 %d，并入 %v\n", f.Target, f.Survivor, f.Losers)
	}
}
