package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"kun-galgame-patch-api/pkg/catalogv2"

	"gorm.io/gorm"
)

type patchRow struct {
	ID            int
	VndbID        string
	ResourceCount int
	Published     bool
	IsStub        bool
}

// requireSchema refuses to run against a database migration 037 has not
// reached. Without ON UPDATE CASCADE the renumber aborts partway through on
// "update or delete on table patch violates foreign key constraint", and
// without the ledger every old URL is lost with no way to rebuild the mapping
// afterwards -- catalog names the survivor of its own merges, but nothing
// records what this site's page id used to be.
func requireSchema(db *gorm.DB) error {
	var ledger int64
	if err := db.Raw(
		`SELECT count(*) FROM information_schema.tables WHERE table_name = 'patch_redirect'`,
	).Scan(&ledger).Error; err != nil {
		return err
	}
	if ledger == 0 {
		return fmt.Errorf("patch_redirect 不存在，先跑 migration 037")
	}
	var stale []string
	if err := db.Raw(`
		SELECT rc.constraint_name
		FROM information_schema.referential_constraints rc
		JOIN information_schema.table_constraints tc ON tc.constraint_name = rc.constraint_name
		JOIN information_schema.constraint_column_usage ccu ON ccu.constraint_name = rc.constraint_name
		WHERE tc.constraint_type = 'FOREIGN KEY' AND ccu.table_name = 'patch'
		  AND rc.update_rule <> 'CASCADE'
	`).Scan(&stale).Error; err != nil {
		return err
	}
	if len(stale) > 0 {
		return fmt.Errorf("这些外键仍是 ON UPDATE NO ACTION，先跑 migration 037: %v", stale)
	}
	return nil
}

func loadPatches(db *gorm.DB) ([]patchRow, error) {
	var rows []patchRow
	err := db.Table("patch").
		Select("id, vndb_id, resource_count, published, is_stub").
		Order("id ASC").Scan(&rows).Error
	return rows, err
}

// workOfGID is the rule the site used to route a page before this change, kept
// here verbatim instead of calling the client.
//
// It has to be a frozen copy. The same commit that runs this command turns the
// client's resolver into the identity, so a command that asked the client would
// answer "every page is already where it belongs" and renumber nothing.
//
// The rule: the `curated` anchor names the work, and failing that the gid is
// taken to be a catalog id of its own -- but only when that work does not claim
// some other page, because 10,289-odd gids are also the catalog id of an
// unrelated work. `galgame_wiki` is not consulted: catalog has no such source
// key, so every lookup by it was a wasted round trip.
func workOfGID(ctx context.Context, v2 *catalogv2.Client, gid int) (int64, bool, error) {
	w, err := v2.WorkByRef(ctx, "curated", strconv.Itoa(gid), true)
	switch {
	case err == nil && w != nil:
		if id, ok := w.IntID(); ok {
			return id, true, nil
		}
	case err != nil && !errors.Is(err, catalogv2.ErrNotFound):
		return 0, false, err
	}

	page, err := v2.ListWorks(ctx, catalogv2.WorksQuery{
		IDs: []int64{int64(gid)}, NSFW: true, Limit: 1,
	})
	if err != nil {
		if errors.Is(err, catalogv2.ErrNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	for i := range page.Items {
		id, ok := page.Items[i].IntID()
		if !ok || id != int64(gid) {
			continue
		}
		if claimedGID(page.Items[i].Claim) == gid {
			return id, true, nil
		}
		if page.Items[i].Claim == nil {
			return id, true, nil
		}
	}
	return 0, false, nil
}

func claimedGID(c *catalogv2.Claim) int {
	if c == nil || (c.Site != catalogv2.SiteKungal && c.Site != "galgame_wiki") {
		return 0
	}
	id, ok := catalogv2.ParseID(c.SiteWorkID)
	if !ok {
		return 0
	}
	return int(id)
}

// resolveOne retries a blown request instead of losing the whole run to it.
// The local catalog timed out once at 6,532 of 10,928 rows and the command
// exited, throwing away eight minutes of resolve. A transient failure must
// also never be read as "catalog cannot name this page": that answer parks a
// live page at LocalOnlyIDBase, where nothing would ever look for it again.
// workOfGID already returns a clean not-found as (0, false, nil), so anything
// arriving here as an error is a transport problem and worth another attempt.
const resolveAttempts = 5

func resolveOne(ctx context.Context, v2 *catalogv2.Client, gid int) (int64, bool, error) {
	var err error
	for attempt := range resolveAttempts {
		var (
			workID int64
			found  bool
		)
		workID, found, err = workOfGID(ctx, v2, gid)
		if err == nil {
			return workID, found, nil
		}
		slog.Warn("解析重试", "gid", gid, "attempt", attempt+1, "error", err)
		select {
		case <-ctx.Done():
			return 0, false, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * time.Second):
		}
	}
	return 0, false, err
}

func resolveTargets(
	ctx context.Context,
	v2 *catalogv2.Client,
	rows []patchRow,
	concurrency int,
) (map[int]int64, error) {
	if concurrency < 1 {
		concurrency = 1
	}
	out := make(map[int]int64, len(rows))
	var mu sync.Mutex
	var firstErr error
	var done int64

	jobs := make(chan int)
	var wg sync.WaitGroup
	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for gid := range jobs {
				workID, found, err := resolveOne(ctx, v2, gid)
				mu.Lock()
				switch {
				case err != nil && firstErr == nil:
					firstErr = fmt.Errorf("resolve gid %d: %w", gid, err)
				case err == nil && found && workID > 0:
					out[gid] = workID
				}
				mu.Unlock()
				if n := atomic.AddInt64(&done, 1); n%2000 == 0 {
					slog.Info("解析中", "done", n, "total", len(rows))
				}
			}
		}()
	}
	for _, r := range rows {
		mu.Lock()
		stop := firstErr != nil
		mu.Unlock()
		if stop {
			break
		}
		jobs <- r.ID
	}
	close(jobs)
	wg.Wait()
	return out, firstErr
}
