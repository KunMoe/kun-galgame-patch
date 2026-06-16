// cmd/image-audit is a schema-derived guard against the two image-migration
// drift bugs moyu kept hitting (because image refs live in free-text markdown
// across many columns, hand-enumerated in two places). It enumerates EVERY
// text/varchar/json column in the DB from information_schema and fails if:
//
//	(a) any column still contains a legacy CDN URL (image.moyu.moe /
//	    image.kungal.com) → migration incomplete, blocks bucket decommission.
//	(b) any column contains a /image/<hash> content token but is NOT in
//	    cron.ContentTokenColumns → ref-ping doesn't cover it, so those images
//	    get GC'd after the ~60d cold-storage TTL.
//
// Exit non-zero on any finding so it can run as a CI test or a periodic
// inspection cron — turning "remember to keep the migration + ref-ping column
// lists in sync" into an automatic guard. (letmoe doesn't need this because it
// stores structured hash columns; moyu can't avoid free-text user markdown.)
//
//	go run ./cmd/image-audit            # report + exit 1 on findings
//	go run ./cmd/image-audit -fail=false  # report only (always exit 0)
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"kun-galgame-patch-api/internal/infrastructure/cron"
	"kun-galgame-patch-api/internal/infrastructure/database"
	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/logger"

	"github.com/joho/godotenv"
)

// legacyHosts are the old image buckets being decommissioned. A reference to any
// of them in content means that row never got migrated.
var legacyHosts = []string{"image.moyu.moe", "image.kungal.com"}

func main() {
	_ = godotenv.Load()

	failOnFindings := flag.Bool("fail", true, "exit non-zero if any issue is found (set false for report-only)")
	flag.Parse()

	cfg := config.Load()
	logger.Init(cfg.Server.Mode)
	db := database.NewPostgres(cfg.Database, cfg.Server.Mode)

	// The authoritative ref-ping coverage list (shared source of truth).
	covered := map[string]bool{}
	for _, c := range cron.ContentTokenColumns {
		covered[c.Table+"."+c.Col] = true
	}

	// Every scannable column in the app schema.
	type col struct{ TableName, ColumnName string }
	var cols []col
	if err := db.Raw(`
		SELECT table_name, column_name FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND data_type IN ('text','character varying','character','json','jsonb')
		ORDER BY table_name, column_name`).Scan(&cols).Error; err != nil {
		slog.Error("枚举列失败", "error", err)
		os.Exit(2)
	}

	scanLike := func(table, column, pattern string) int64 {
		var n int64
		// table/column come from information_schema (trusted) → double-quote them.
		q := fmt.Sprintf(`SELECT count(*) FROM %q WHERE %q::text LIKE ?`, table, column)
		_ = db.Raw(q, pattern).Row().Scan(&n)
		return n
	}
	scanRegex := func(table, column, re string) int64 {
		var n int64
		q := fmt.Sprintf(`SELECT count(*) FROM %q WHERE %q::text ~ ?`, table, column)
		_ = db.Raw(q, re).Row().Scan(&n)
		return n
	}

	var legacyHits, gapHits int
	for _, c := range cols {
		key := c.TableName + "." + c.ColumnName

		// (a) legacy CDN URLs still present → migration incomplete.
		for _, host := range legacyHosts {
			if n := scanLike(c.TableName, c.ColumnName, "%"+host+"%"); n > 0 {
				slog.Error("legacy CDN URL 仍存在(迁移未完成)", "column", key, "host", host, "rows", n)
				legacyHits++
			}
		}

		// (b) /image/<hash> token in a column ref-ping doesn't cover → GC risk.
		if n := scanRegex(c.TableName, c.ColumnName, `/image/[0-9a-f]{64}`); n > 0 && !covered[key] {
			slog.Error("内容图 token 所在列未被 ref-ping 覆盖(将被 GC)", "column", key, "rows", n,
				"fix", "把该列加入 cron.ContentTokenColumns")
			gapHits++
		}
	}

	// Sanity: a ref-ping entry pointing at a non-existent column is itself a bug.
	present := map[string]bool{}
	for _, c := range cols {
		present[c.TableName+"."+c.ColumnName] = true
	}
	for k := range covered {
		if !present[k] {
			slog.Warn("cron.ContentTokenColumns 列不存在(打错表/列名?)", "column", k)
		}
	}

	if legacyHits+gapHits > 0 {
		slog.Error("image-audit 不通过", "legacy_url_columns", legacyHits, "uncovered_token_columns", gapHits)
		if *failOnFindings {
			os.Exit(1)
		}
		return
	}
	slog.Info("image-audit 通过", "scanned_columns", len(cols),
		"legacy_url_columns", 0, "uncovered_token_columns", 0)
}
