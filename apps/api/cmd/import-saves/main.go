// import-saves uploads the crawled save-data archives as save resources on
// their works, owned by one archive account.
//
// The manifest is written by infra's `import-walkthroughs --stage saves` (one
// JSON object per line, already matched to a catalog work and deduplicated by
// work and sha256); --root is the directory its paths are relative to.
//
// Every row it writes is dated importedAt, the day moyu.moe was registered, so
// tens of thousands of archived saves do not flood the new-patch and
// new-resource lists, and it neither awards moemoepoints nor notifies anyone.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"sync"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/infrastructure/database"
	"kun-galgame-patch-api/pkg/artifactclient"
	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/logger"
)

func main() {
	manifest := flag.String("manifest", "", "JSONL manifest from import-walkthroughs --stage saves (REQUIRED)")
	root := flag.String("root", ".", "directory the manifest paths are relative to")
	userID := flag.Int("user-id", 0, "archive account that owns the imported saves (REQUIRED)")
	dryRun := flag.Bool("dry-run", false, "check pages and duplicates only: no upload, no write")
	limit := flag.Int("limit", 0, "process at most N manifest records (0 = all)")
	workers := flag.Int("workers", 4, "works processed concurrently")
	flag.Parse()

	cfg := config.Load()
	logger.Init(cfg.Server.Mode)

	if *manifest == "" || *userID <= 0 {
		slog.Error("--manifest and --user-id are required")
		os.Exit(2)
	}
	if cfg.NextMoeAPI.APIKey == "" {
		slog.Error("KUN_NEXTMOE_API_KEY is required: the catalog galgame read face is gated by the internal-tier key")
		os.Exit(1)
	}
	recs, err := readManifest(*manifest, *limit)
	if err != nil {
		slog.Error("read manifest", "error", err)
		os.Exit(1)
	}

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

	imp := &importer{
		db:      database.NewPostgres(cfg.Database, cfg.Server.Mode),
		catalog: galgameClient.NewWithKey(cfg.NextMoeAPI.BaseURL, cfg.NextMoeAPI.APIKey),
		files:   art,
		root:    *root,
		userID:  *userID,
		dryRun:  *dryRun,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	byWork := map[int][]saveRecord{}
	for _, r := range recs {
		byWork[r.WorkID] = append(byWork[r.WorkID], r)
	}
	works := make([]int, 0, len(byWork))
	for w := range byWork {
		works = append(works, w)
	}
	sort.Ints(works)

	var mu sync.Mutex
	counts := map[status]int{}
	done := 0
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range max(*workers, 1) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for w := range jobs {
				for _, res := range imp.importWork(ctx, w, byWork[w]) {
					if res.status == statusFailed {
						slog.Error("failed", "asset", res.assetID, "work", w, "detail", res.msg)
					}
					mu.Lock()
					counts[res.status]++
					if done++; done%250 == 0 {
						slog.Info("progress", "done", done, "of", len(recs))
					}
					mu.Unlock()
				}
			}
		}()
	}
	for _, w := range works {
		if ctx.Err() != nil {
			break
		}
		jobs <- w
	}
	close(jobs)
	wg.Wait()

	slog.Info("summary", "works", len(works), "ok", counts[statusOK], "dry-run", counts[statusDryRun],
		"skipped", counts[statusSkipped], "work-deleted", counts[statusWorkDeleted],
		"work-missing", counts[statusWorkMissing], "failed", counts[statusFailed])
	if counts[statusFailed] > 0 {
		os.Exit(1)
	}
}

func readManifest(path string, limit int) ([]saveRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []saveRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		var r saveRecord
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			return nil, err
		}
		out = append(out, r)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, sc.Err()
}
