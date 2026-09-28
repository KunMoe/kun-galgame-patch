// settle-local-pages moves the 27 pages the 037 renumber parked at
// LocalOnlyIDBase onto the catalog works they show, and rewrites the vndb_id
// strings the 2026-09-28 id census found stale or wrong.
//
// Without -apply the whole transaction runs, reports, and rolls back, so the
// report is what a real run would leave behind.
//
//	go run ./cmd/settle-local-pages            # rehearse and roll back
//	go run ./cmd/settle-local-pages -apply     # commit
package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"kun-galgame-patch-api/internal/infrastructure/database"
	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/logger"

	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

var errRehearsal = errors.New("rehearsal: rolled back")

func main() {
	_ = godotenv.Load()

	apply := flag.Bool("apply", false, "提交事务（默认跑完后回滚）")
	flag.Parse()

	cfg := config.Load()
	logger.Init(cfg.Server.Mode)
	db := database.NewPostgres(cfg.Database, cfg.Server.Mode)

	var rep report
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := run(tx, &rep); err != nil {
			return err
		}
		if err := rep.verify(tx); err != nil {
			return err
		}
		if !*apply {
			return errRehearsal
		}
		return nil
	})
	rep.print()

	switch {
	case errors.Is(err, errRehearsal):
		fmt.Println("\n未加 -apply，事务已回滚。")
	case err != nil:
		slog.Error("整改失败，事务已回滚", "error", err)
		os.Exit(1)
	default:
		fmt.Println("\n✅ 已提交：本地段已清空。")
	}
}
