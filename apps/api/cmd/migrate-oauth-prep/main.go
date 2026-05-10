// cmd/migrate-oauth-prep applies the one-shot data-shape conversion that the
// kungalgame_patch database needs *before* the numbered migrations under
// migrations/ can run.
//
// What this is:
//
//   - Fill NULL → '{}' for 16 text[] columns (patch / patch_resource / etc.)
//     so the subsequent jsonb cast does not produce NULL on required fields.
//   - Convert those 16 columns to jsonb (DROP DEFAULT → ALTER TYPE → SET DEFAULT).
//   - Add denormalized `*_count` fields on user / patch / patch_comment /
//     patch_resource and backfill them from the relation tables.
//   - Create the oauth_account table (later dropped by migration 005 once
//     the OAuth-side id alignment is done; harmless in the interim).
//
// Why a separate cmd (not just another file under migrations/):
//
//   - This runs *before* numbered migrations 001-005 in the upgrade pipeline,
//     not as part of normal feature evolution. Slotting it under migrations/
//     would put it inline with 001-... and confuse the up/down semantics.
//   - It runs exactly once per database. Idempotency is handled by checking
//     the existing _migrations tracker for the marker `oauth_prep_20260409`.
//
// Replaces:
//
//	psql -h <host> -U <user> -d <db> -f apps/next-web/prisma/migrations/20260409_oauth_integration/migration.sql
//
// Usage:
//
//	go run ./cmd/migrate-oauth-prep             # apply with confirmation prompt
//	go run ./cmd/migrate-oauth-prep -yes        # skip confirmation (CI)
//	go run ./cmd/migrate-oauth-prep -dry-run    # print SQL only
//	go run ./cmd/migrate-oauth-prep -force      # re-run even if marker is present
package main

import (
	"bufio"
	"database/sql"
	_ "embed"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"kun-galgame-patch-api/internal/infrastructure/database"
	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/logger"

	"github.com/joho/godotenv"
)

//go:embed migration.sql
var migrationSQL string

// markerName is the row written into _migrations after a successful apply.
// Re-running with the marker present requires -force.
const markerName = "oauth_prep_20260409"

func main() {
	_ = godotenv.Load()

	dryRun := flag.Bool("dry-run", false, "Print the SQL without executing")
	autoYes := flag.Bool("yes", false, "Skip confirmation prompt (CI)")
	force := flag.Bool("force", false, "Re-run even if the marker says it already ran")
	flag.Parse()

	cfg := config.Load()
	logger.Init(cfg.Server.Mode)

	if *dryRun {
		fmt.Print(migrationSQL)
		return
	}

	db := database.NewPostgres(cfg.Database, cfg.Server.Mode)
	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("get db conn failed", "error", err)
		os.Exit(1)
	}

	if err := ensureTracker(sqlDB); err != nil {
		slog.Error("ensure tracker failed", "error", err)
		os.Exit(1)
	}

	if alreadyRan(sqlDB) {
		if !*force {
			fmt.Printf("oauth-prep already applied (marker: %s). Use -force to re-run.\n", markerName)
			return
		}
		fmt.Println("⚠️  -force: re-running despite existing marker")
	}

	printPlan(cfg)

	if !*autoYes && !confirm() {
		fmt.Println("Cancelled")
		return
	}

	// The embedded SQL already wraps in BEGIN/COMMIT, so a single Exec is
	// enough; PostgreSQL will roll back on any failure inside.
	if _, err := sqlDB.Exec(migrationSQL); err != nil {
		slog.Error("oauth-prep SQL failed", "error", err)
		os.Exit(1)
	}

	if _, err := sqlDB.Exec(
		`INSERT INTO _migrations (name) VALUES ($1) ON CONFLICT (name) DO NOTHING`,
		markerName,
	); err != nil {
		slog.Error("write marker failed", "error", err)
		os.Exit(1)
	}

	fmt.Println("✅ oauth-prep applied")
}

func ensureTracker(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS _migrations (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL UNIQUE,
			applied_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func alreadyRan(db *sql.DB) bool {
	var n int
	_ = db.QueryRow("SELECT COUNT(*) FROM _migrations WHERE name = $1", markerName).Scan(&n)
	return n > 0
}

func printPlan(cfg *config.Config) {
	fmt.Println("──────────────────────────────────────────")
	fmt.Printf("Database : %s\n", redactURL(cfg.Database.URL))
	fmt.Println("Action   : OAuth integration prep (one-shot)")
	fmt.Printf("Marker   : %s\n", markerName)
	fmt.Printf("SQL size : %d bytes (~%d lines)\n", len(migrationSQL), strings.Count(migrationSQL, "\n"))
	fmt.Println("──────────────────────────────────────────")
	fmt.Println("This will, atomically:")
	fmt.Println("  1. NULL → '{}' on 16 text[] columns")
	fmt.Println("  2. Convert those 16 columns to jsonb")
	fmt.Println("  3. Add denormalized *_count fields and backfill them")
	fmt.Println("  4. Create the oauth_account table")
	fmt.Println("──────────────────────────────────────────")
}

func confirm() bool {
	fmt.Print("Apply? (y/N) ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return answer == "y" || answer == "yes"
}

// redactURL replaces the password in postgres://user:pass@host/db with ***.
func redactURL(u string) string {
	at := strings.Index(u, "@")
	if at < 0 {
		return u
	}
	colon := strings.LastIndex(u[:at], ":")
	scheme := strings.Index(u, "://")
	if colon < 0 || scheme < 0 || colon <= scheme+2 {
		return u
	}
	return u[:colon+1] + "***" + u[at:]
}
