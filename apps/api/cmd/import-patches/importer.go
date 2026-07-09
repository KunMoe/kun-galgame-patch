package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/patch/model"
	patchRepo "kun-galgame-patch-api/internal/patch/repository"
	"kun-galgame-patch-api/pkg/artifactclient"
	"kun-galgame-patch-api/pkg/utils"

	"gorm.io/gorm"
)

type status string

const (
	statusOK           status = "ok"
	statusSkipped      status = "skipped"
	statusDryRun       status = "dry-run"
	statusUnrecognized status = "unrecognized"
	statusWikiMissing  status = "wiki-missing"
	statusFailed       status = "failed"
)

type fileResult struct {
	file   string
	status status
	msg    string
}

// Importer holds the minimal wiring for the archive import: DB + repo (the
// controlled subset of PatchService.CreateResource writes, MINUS moemoepoint and
// notifications per the internal-account requirement), the Wiki client (vndb_id
// -> galgame_id), and the artifact client (large-file upload).
type Importer struct {
	db     *gorm.DB
	repo   *patchRepo.PatchRepository
	wiki   *galgameClient.Client
	art    *artifactclient.Client
	userID int
	dryRun bool
	// touched collects every galgame_id we resolved (published or draft) so the
	// run can flag the ones still at wiki status=2 afterwards. CheckGalgameByVndbID
	// returns exists=true even for unclaimed VNDB drafts, so a naive import creates
	// patches on galgames the public wiki batch/detail won't return → invisible on
	// moyu. We can't claim them here (needs a user/admin JWT the S2S importer
	// lacks), so we DETECT + report them with the exact remediation instead.
	touched map[int]struct{}
}

func (imp *Importer) processFile(ctx context.Context, path string) fileResult {
	name := filepath.Base(path)

	p := parsePatchFileName(path)
	if p == nil {
		return fileResult{name, statusUnrecognized, "unrecognized filename"}
	}

	exists, galgameID, err := imp.wiki.CheckGalgameByVndbID(ctx, p.VndbID)
	if err != nil {
		return fileResult{name, statusFailed, "wiki check: " + err.Error()}
	}
	if !exists {
		// Premise: vndb_id is expected to exist on the Wiki; a miss is the rare
		// case — record it for manual review, never crash the batch.
		return fileResult{name, statusWikiMissing, "vndb " + p.VndbID + " not on wiki"}
	}
	if imp.touched != nil {
		imp.touched[galgameID] = struct{}{}
	}

	sanitized := sanitizeFileName(p.FileName)
	dup, err := imp.resourceExists(galgameID, sanitized)
	if err != nil {
		return fileResult{name, statusFailed, "dedup: " + err.Error()}
	}
	if dup {
		return fileResult{name, statusSkipped, "already imported"}
	}

	// dry-run stops here — everything above is read-only.
	if imp.dryRun {
		return fileResult{name, statusDryRun, fmt.Sprintf("would import -> galgame %d", galgameID)}
	}

	fi, err := os.Stat(path)
	if err != nil {
		return fileResult{name, statusFailed, err.Error()}
	}
	// Skip empty placeholder files: the archive's create_empty_files.py
	// materializes 0-byte stand-ins for files shipped in earlier increments.
	if fi.Size() == 0 {
		return fileResult{name, statusSkipped, "empty placeholder file"}
	}

	// Writes begin here — create the patch carrier only for a real import.
	if err := imp.ensurePatch(ctx, galgameID, p.VndbID); err != nil {
		return fileResult{name, statusFailed, "ensure patch: " + err.Error()}
	}
	slog.Info("uploading", "file", name, "galgame", galgameID, "size", formatSize(fi.Size()))
	uuid, size, err := uploadFileToArtifact(ctx, imp.art, path, sanitized, fi.Size(), imp.userID)
	if err != nil {
		return fileResult{name, statusFailed, "upload: " + err.Error()}
	}

	if err := imp.createResource(galgameID, p, sanitized, uuid, size); err != nil {
		return fileResult{name, statusFailed, "db: " + err.Error()}
	}
	return fileResult{name, statusOK, fmt.Sprintf("galgame %d, %s, %s", galgameID, formatSize(size), uuid)}
}

// ensurePatch idempotently registers the local patch carrier (patch.id =
// galgame_id). Mirrors createPatchRow MINUS the +3 moemoepoint. If the row
// already exists (real publish or an interaction stub) it is left as-is; only
// the missing case inserts, registering the archive account as a contributor.
func (imp *Importer) ensurePatch(ctx context.Context, galgameID int, vndbID string) error {
	if existing, err := imp.repo.GetPatchDetail(galgameID); err == nil && existing != nil && existing.ID != 0 {
		_ = imp.repo.EnsureContributor(imp.userID, galgameID)
		return nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("get patch: %w", err)
	}

	// Mirror the Wiki release_date locally (best-effort; a wiki blip leaves NULL).
	var releaseDate *time.Time
	if env, gErr := imp.wiki.GetGalgame(ctx, galgameID, ""); gErr == nil && env != nil && env.Galgame.ReleaseDate != nil {
		releaseDate = utils.ParseWikiReleaseDate(*env.Galgame.ReleaseDate)
	}

	err := imp.db.Transaction(func(tx *gorm.DB) error {
		p := &model.Patch{ID: galgameID, VndbID: vndbID, UserID: imp.userID, ReleaseDate: releaseDate}
		if e := tx.Create(p).Error; e != nil {
			return e
		}
		if e := tx.Create(&model.UserPatchContributeRelation{UserID: imp.userID, GalgameID: galgameID}).Error; e != nil {
			return e
		}
		return tx.Model(&model.Patch{}).Where("id = ?", galgameID).
			UpdateColumn("contribute_count", gorm.Expr("contribute_count + 1")).Error
	})
	if err != nil {
		// A concurrent import of another file for the same galgame may have won
		// the pkey race — treat as success if the row is present now.
		if ex, e := imp.repo.GetPatchDetail(galgameID); e == nil && ex != nil && ex.ID != 0 {
			return nil
		}
		return fmt.Errorf("create patch: %w", err)
	}
	return nil
}

// resourceExists is the idempotency guard, scoped to the archive account.
// Artifact-backed rows blank Content/S3Key, and — critically — the 1530 legacy
// archive rows carry name=” with the sanitized filename embedded in `note`
// (the old sync-patch tool's note template ends with {文件名}). So we match on
// EITHER the new name column OR the sanitized filename appearing literally in
// note (strpos, not LIKE, so '_' in the name isn't a wildcard).
func (imp *Importer) resourceExists(galgameID int, sanitized string) (bool, error) {
	var count int64
	err := imp.db.Model(&model.PatchResource{}).
		Where("galgame_id = ? AND user_id = ? AND (name = ? OR strpos(note, ?) > 0)",
			galgameID, imp.userID, sanitized, sanitized).
		Count(&count).Error
	return count > 0, err
}

// createResource performs the controlled minimal write set: insert the
// artifact-backed resource, bump resource_count, recompute the patch's
// type/language/platform aggregates, stamp resource_update_time, and ensure the
// archive account is a contributor. Deliberately NOT done (vs
// PatchService.CreateResource): moemoepoint award + favorited-user notifications.
func (imp *Importer) createResource(galgameID int, p *parsedPatch, sanitized, uuid string, size int64) error {
	res := &model.PatchResource{
		GalgameID:             galgameID,
		Storage:               "s3", // artifact-backed; Content/S3Key stay empty
		ArtifactUUID:          uuid,
		Name:                  sanitized,
		LocalizationGroupName: p.GroupName,
		Size:                  formatSize(size),
		Note:                  renderNote(p, sanitized, imp.userID),
		Type:                  model.JSONArray{"manual"},
		Language:              model.JSONArray(p.Languages),
		Platform:              model.JSONArray{p.Platform},
		UserID:                imp.userID,
	}
	if err := imp.repo.CreateResource(res); err != nil {
		return err
	}

	// Post-insert aggregates: failures here leave a valid resource row but stale
	// counters, so warn rather than fail the whole import.
	if err := imp.repo.UpdateCount(galgameID, "resource_count", 1); err != nil {
		slog.Warn("resource_count bump failed", "galgame", galgameID, "err", err)
	}
	if err := imp.repo.RecalculatePatchAggregates(galgameID); err != nil {
		slog.Warn("recalc aggregates failed", "galgame", galgameID, "err", err)
	}
	if err := imp.db.Model(&model.Patch{}).Where("id = ?", galgameID).
		Update("resource_update_time", time.Now()).Error; err != nil {
		slog.Warn("resource_update_time bump failed", "galgame", galgameID, "err", err)
	}
	if err := imp.repo.EnsureContributor(imp.userID, galgameID); err != nil {
		slog.Warn("ensure contributor failed", "galgame", galgameID, "err", err)
	}
	return nil
}

// processDelete mirrors one archive "delete old file" entry (delete_list.txt):
// the increment supersedes an older patch file, so the matching moyu resource
// should go too. Resolve the superseded filename the same way as import
// (parse → wiki → galgame_id), find the ARCHIVE-owned row (guarded to
// user_id == imp.userID so a user-uploaded resource is never touched), soft-
// delete the artifact blob, hard-delete the row (FK cascades take likes /
// favorites / history / revisions), and fix the aggregates.
func (imp *Importer) processDelete(ctx context.Context, rawLine string) fileResult {
	name := filepath.Base(strings.TrimSpace(rawLine))
	if name == "" || strings.HasPrefix(name, "#") {
		return fileResult{name, statusSkipped, "blank/comment"}
	}

	p := parsePatchFileName(name)
	if p == nil {
		return fileResult{name, statusUnrecognized, "unrecognized filename"}
	}
	exists, galgameID, err := imp.wiki.CheckGalgameByVndbID(ctx, p.VndbID)
	if err != nil {
		return fileResult{name, statusFailed, "wiki check: " + err.Error()}
	}
	if !exists {
		return fileResult{name, statusWikiMissing, "vndb " + p.VndbID + " not on wiki"}
	}

	sanitized := sanitizeFileName(name)
	var res model.PatchResource
	err = imp.db.Where("galgame_id = ? AND user_id = ? AND (name = ? OR strpos(note, ?) > 0)",
		galgameID, imp.userID, sanitized, sanitized).First(&res).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fileResult{name, statusSkipped, "not present (already gone)"}
	}
	if err != nil {
		return fileResult{name, statusFailed, "lookup: " + err.Error()}
	}

	if imp.dryRun {
		return fileResult{name, statusDryRun, fmt.Sprintf("would delete resource %d (galgame %d)", res.ID, galgameID)}
	}

	if res.ArtifactUUID != "" {
		if e := imp.art.Delete(ctx, res.ArtifactUUID); e != nil {
			slog.Warn("artifact delete failed (continuing with DB delete)", "uuid", res.ArtifactUUID, "err", e)
		}
	}
	if e := imp.db.Delete(&model.PatchResource{}, res.ID).Error; e != nil {
		return fileResult{name, statusFailed, "delete row: " + e.Error()}
	}
	if e := imp.repo.UpdateCount(galgameID, "resource_count", -1); e != nil {
		slog.Warn("resource_count decrement failed", "galgame", galgameID, "err", e)
	}
	if e := imp.repo.RecalculatePatchAggregates(galgameID); e != nil {
		slog.Warn("recalc aggregates failed", "galgame", galgameID, "err", e)
	}
	return fileResult{name, statusOK, fmt.Sprintf("deleted resource %d (galgame %d)", res.ID, galgameID)}
}

// unpublishedDrafts returns the touched galgame_ids that the public wiki batch
// does NOT return — i.e. still at status=2 (unclaimed VNDB draft). Their imported
// resources are invisible on moyu (homepage/list/detail read only published
// galgames) until they're published. Claiming needs a user/admin JWT the S2S
// importer has no way to obtain, so we surface them for the operator to publish.
func (imp *Importer) unpublishedDrafts(ctx context.Context) []int {
	if len(imp.touched) == 0 {
		return nil
	}
	ids := make([]int, 0, len(imp.touched))
	for id := range imp.touched {
		ids = append(ids, id)
	}
	published := make(map[int]struct{}, len(ids))
	for i := 0; i < len(ids); i += 80 {
		end := i + 80
		if end > len(ids) {
			end = len(ids)
		}
		briefs, err := imp.wiki.GalgameBatch(ctx, ids[i:end], "all")
		if err != nil {
			slog.Warn("draft check: wiki batch failed (skipping this chunk)", "err", err)
			continue
		}
		for j := range briefs {
			published[briefs[j].ID] = struct{}{}
		}
	}
	var drafts []int
	for _, id := range ids {
		if _, ok := published[id]; !ok {
			drafts = append(drafts, id)
		}
	}
	sort.Ints(drafts)
	return drafts
}
