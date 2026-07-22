// cmd/import-patches bulk-imports a directory of standardized Chinese-patch
// archives (`.rar/.zip/.7z`, legacy naming unchanged) into moyu, replacing the
// old standalone sync-patch tool. Per file it: parses the name, maps the VNDB id
// to a Wiki galgame_id (identities are shared: patch.id == galgame_id), ensures
// the local patch carrier exists, uploads the bytes through the centralized
// artifact service (init -> PUT straight to B2 from disk -> complete; NO blake3),
// and inserts an artifact-backed patch_resource.
//
// It is an INTERNAL archive job: all rows are owned by --user-id (default 2310)
// and it deliberately skips the moemoepoint award + favorited-user notifications
// that PatchService.CreateResource does (see importer.go). Idempotent: re-runs
// skip files already imported for that galgame (dedup on (galgame_id, name)).
//
// --delete-list mirrors an increment's "delete old file" list (delete_list.txt):
// each superseded filename's archive-owned resource is removed (artifact blob +
// row + aggregates). Deletes run BEFORE imports, matching the archive's
// delete-then-overlay flow. Both phases honor --dry-run.
//
// Deploy: cross-compile static (CGO_ENABLED=0 GOOS=linux) and run on the same
// docker network as the moyu/infra stack (dokploy-network) so the internal
// service DNS names (artifact / galgame / postgres) resolve, with egress for the
// B2 PUT. Config comes from the moyu-api env (godotenv.Load + config.Load).
//
// Usage:
//
//	import-patches --dir /patches --dry-run              # probe imports, no writes
//	import-patches --dir /patches                        # import
//	import-patches --delete-list del.txt --dir /patches  # delete superseded, then import
//	import-patches --dir /patches --vndb v14,v36         # only these VNDB ids
//	import-patches --dir /patches --limit 3              # first N recognized files (testing)
package main

import (
	"bufio"
	"context"
	"flag"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/infrastructure/database"
	patchRepo "kun-galgame-patch-api/internal/patch/repository"
	"kun-galgame-patch-api/pkg/artifactclient"
	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/logger"

	"github.com/joho/godotenv"
)

var allowedExts = []string{".zip", ".rar", ".7z"}

func main() {
	_ = godotenv.Load()

	dir := flag.String("dir", "./patch", "directory of patch archives to import")
	deleteList := flag.String("delete-list", "", "path to a delete_list.txt of superseded filenames to remove (runs before import)")
	dryRun := flag.Bool("dry-run", false, "probe only: parse + wiki-check + dedup, no upload/write/delete")
	userID := flag.Int("user-id", 2310, "archive account user_id that owns the imported patches/resources")
	vndbFilter := flag.String("vndb", "", "comma-separated VNDB ids to restrict imports to (e.g. v14,v36)")
	limit := flag.Int("limit", 0, "process at most N recognized files (0 = all; for testing)")
	flag.Parse()

	cfg := config.Load()
	logger.Init(cfg.Server.Mode)

	only := map[string]bool{}
	for _, v := range strings.Split(*vndbFilter, ",") {
		if v = strings.TrimSpace(v); v != "" {
			only[v] = true
		}
	}

	db := database.NewPostgres(cfg.Database, cfg.Server.Mode)
	wiki := galgameClient.NewWithKey(cfg.NextMoeAPI.BaseURL, cfg.NextMoeAPI.APIKey)

	// artifact client: default creds to the project's OAuth client when the
	// KUN_ARTIFACT_OAUTH_* vars are unset (mirrors internal/app/app.go).
	artCfg := cfg.Artifact
	if artCfg.ClientID == "" {
		artCfg.ClientID = cfg.OAuth.ClientID
	}
	if artCfg.ClientSecret == "" {
		artCfg.ClientSecret = cfg.OAuth.ClientSecret
	}
	art := artifactclient.New(artifactclient.Config{
		BaseURL:      artCfg.BaseURL,
		ClientID:     artCfg.ClientID,
		ClientSecret: artCfg.ClientSecret,
	})
	if !art.Configured() {
		slog.Error("artifact client not configured (missing base URL or credentials)")
		os.Exit(1)
	}

	imp := &Importer{db: db, repo: patchRepo.New(db), wiki: wiki, art: art, userID: *userID, dryRun: *dryRun, touched: map[int]struct{}{}}

	ctx := context.Background()
	counts := map[status]int{}
	var wikiMissing, failed []fileResult
	tally := func(phase string, r fileResult) {
		counts[r.status]++
		switch r.status {
		case statusWikiMissing:
			wikiMissing = append(wikiMissing, r)
		case statusFailed:
			failed = append(failed, r)
		}
		slog.Info(phase, "status", r.status, "file", r.file, "detail", r.msg)
	}

	// Phase 1: deletions (superseded files), before overlaying the new increment.
	if *deleteList != "" {
		f, err := os.Open(*deleteList)
		if err != nil {
			slog.Error("open delete-list failed", "path", *deleteList, "err", err)
			os.Exit(1)
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1024*1024), 1024*1024) // long CJK filenames
		for sc.Scan() {
			line := sc.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			tally("delete", imp.processDelete(ctx, line))
		}
		_ = f.Close()
		if err := sc.Err(); err != nil {
			slog.Error("read delete-list failed", "err", err)
			os.Exit(1)
		}
	}

	// Phase 2: imports (new/overlaid files).
	entries, err := os.ReadDir(*dir)
	if err != nil {
		if *deleteList != "" {
			slog.Warn("skipping import phase: cannot read --dir", "dir", *dir, "err", err)
		} else {
			slog.Error("read dir failed", "dir", *dir, "err", err)
			os.Exit(1)
		}
	}
	processed := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !slices.Contains(allowedExts, strings.ToLower(filepath.Ext(name))) {
			continue
		}
		path := filepath.Join(*dir, name)

		// --vndb filter needs the parsed id; parse once here for filtering, the
		// importer re-parses (cheap, keeps processFile self-contained).
		if len(only) > 0 {
			p := parsePatchFileName(path)
			if p == nil || !only[p.VndbID] {
				continue
			}
		}
		if *limit > 0 && processed >= *limit {
			break
		}
		processed++
		tally("import", imp.processFile(ctx, path))
	}

	// Flag imported galgames still at wiki status=2 (unclaimed VNDB draft): their
	// resources are invisible on moyu until published. We can't claim from the S2S
	// importer, so report them + the exact remediation.
	if drafts := imp.unpublishedDrafts(ctx); len(drafts) > 0 {
		idList := make([]string, len(drafts))
		for i, id := range drafts {
			idList[i] = strconv.Itoa(id)
		}
		csv := strings.Join(idList, ",")
		slog.Warn("UNPUBLISHED wiki drafts (status=2) — these galgames + their imported resources are INVISIBLE on moyu until published",
			"count", len(drafts), "galgame_ids", csv)
		slog.Warn("remediation 1/2 — on kun_galgame_wiki DB run:",
			"sql", "UPDATE galgame SET status=0 WHERE id IN ("+csv+") AND status=2;")
		slog.Warn("remediation 2/2 — rebuild search so they're findable:",
			"cmd", "reindex-search --index=galgames")
	}

	slog.Info("summary",
		"ok", counts[statusOK], "dry-run", counts[statusDryRun], "skipped", counts[statusSkipped],
		"unrecognized", counts[statusUnrecognized], "wiki-missing", counts[statusWikiMissing],
		"failed", counts[statusFailed])

	if len(wikiMissing) > 0 {
		slog.Warn("VNDB ids not found on Wiki (need manual review):")
		for _, r := range wikiMissing {
			slog.Warn("  wiki-missing", "file", r.file, "detail", r.msg)
		}
	}
	if len(failed) > 0 {
		for _, r := range failed {
			slog.Error("  failed", "file", r.file, "detail", r.msg)
		}
		os.Exit(1)
	}
}
