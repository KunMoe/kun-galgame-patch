// catalog-merge-sync drains catalog's work-merge feed once, on demand. The cron
// in internal/infrastructure/cron runs the same code every ten minutes; this
// exists because the first drain is not a steady-state tick -- it replays the
// whole merge history, which was 4,473 work redirects when this shipped -- and
// that is worth running under supervision instead of finding it in a log.
//
//	go run ./cmd/catalog-merge-sync           # drain and report, writes nothing
//	go run ./cmd/catalog-merge-sync -apply    # same, and writes
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/infrastructure/cron"
	"kun-galgame-patch-api/internal/infrastructure/database"
	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/logger"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	apply := flag.Bool("apply", false, "真正写库（默认只出报告）")
	flag.Parse()

	cfg := config.Load()
	logger.Init(cfg.Server.Mode)

	db := database.NewPostgres(cfg.Database, cfg.Server.Mode)
	v2 := galgameClient.NewWithKey(cfg.NextMoeAPI.BaseURL, cfg.NextMoeAPI.APIKey).V2()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	report, caughtUp, err := cron.RunCatalogMergeSync(ctx, db, v2, *apply)
	if err != nil {
		slog.Error("合并同步失败", "error", err)
		os.Exit(1)
	}

	fmt.Printf("\n合并同步\n")
	fmt.Printf("  读到的合并      %6d\n", report.Scanned)
	fmt.Printf("  折叠重复页      %6d\n", report.Folded)
	fmt.Printf("  页面改号        %6d\n", report.Renumbered)
	fmt.Printf("  只补 301        %6d\n", report.Ledger)
	fmt.Printf("  与本站无关      %6d\n", report.Skipped)
	fmt.Printf("  已追平          %6v\n", caughtUp)

	if !*apply {
		fmt.Println("\n未加 -apply，未写库（游标也没动）。")
	}
}
