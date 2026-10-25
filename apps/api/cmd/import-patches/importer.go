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
	statusOK             status = "ok"
	statusSkipped        status = "skipped"
	statusDryRun         status = "dry-run"
	statusUnrecognized   status = "unrecognized"
	statusCatalogMissing status = "catalog-missing"
	statusFailed         status = "failed"
)

type fileResult struct {
	file   string
	status status
	msg    string
}

type Importer struct {
	db      *gorm.DB
	repo    *patchRepo.PatchRepository
	galgame *galgameClient.Client
	art     *artifactclient.Client
	userID  int
	dryRun  bool
	touched map[int]struct{}
}

func (imp *Importer) processFile(ctx context.Context, path string) fileResult {
	name := filepath.Base(path)

	p := parsePatchFileName(path)
	if p == nil {
		return fileResult{name, statusUnrecognized, "unrecognized filename"}
	}

	exists, galgameID, err := imp.galgame.CheckGalgameByVndbID(ctx, p.VndbID)
	if err != nil {
		return fileResult{name, statusFailed, "catalog check: " + err.Error()}
	}
	if !exists {
		return fileResult{name, statusCatalogMissing, "vndb " + p.VndbID + " not in catalog"}
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

	fi, err := os.Stat(path)
	if err != nil {
		return fileResult{name, statusFailed, err.Error()}
	}
	if fi.Size() == 0 {
		return fileResult{name, statusSkipped, "empty placeholder file"}
	}

	if imp.dryRun {
		return fileResult{name, statusDryRun, fmt.Sprintf("would import -> galgame %d", galgameID)}
	}

	if err := imp.ensurePatch(ctx, galgameID, p.VndbID); err != nil {
		return fileResult{name, statusFailed, "ensure patch: " + err.Error()}
	}
	slog.Info("uploading", "file", name, "galgame", galgameID, "size", formatSize(fi.Size()))
	uuid, size, err := imp.art.UploadFile(ctx, path, sanitized, fi.Size(), imp.userID)
	if err != nil {
		return fileResult{name, statusFailed, "upload: " + err.Error()}
	}

	if err := imp.createResource(galgameID, p, sanitized, uuid, size); err != nil {
		return fileResult{name, statusFailed, "db: " + err.Error()}
	}
	return fileResult{name, statusOK, fmt.Sprintf("galgame %d, %s, %s", galgameID, formatSize(size), uuid)}
}

func (imp *Importer) ensurePatch(ctx context.Context, galgameID int, vndbID string) error {
	if existing, err := imp.repo.GetPatchDetail(galgameID); err == nil && existing != nil && existing.ID != 0 {
		_ = imp.repo.EnsureContributor(imp.userID, galgameID)
		return nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("get patch: %w", err)
	}

	var releaseDate *time.Time
	if env, gErr := imp.galgame.GetGalgame(ctx, galgameID, ""); gErr == nil && env != nil && env.Galgame.ReleaseDate != nil {
		releaseDate = utils.ParseGalgameReleaseDate(*env.Galgame.ReleaseDate)
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
		if ex, e := imp.repo.GetPatchDetail(galgameID); e == nil && ex != nil && ex.ID != 0 {
			return nil
		}
		return fmt.Errorf("create patch: %w", err)
	}
	return nil
}

func (imp *Importer) resourceExists(galgameID int, sanitized string) (bool, error) {
	var count int64
	err := imp.db.Model(&model.PatchResource{}).
		Where("galgame_id = ? AND user_id = ? AND (name = ? OR strpos(note, ?) > 0)",
			galgameID, imp.userID, sanitized, sanitized).
		Count(&count).Error
	return count > 0, err
}

func (imp *Importer) createResource(galgameID int, p *parsedPatch, sanitized, uuid string, size int64) error {
	res := &model.PatchResource{
		GalgameID:             galgameID,
		Storage:               "s3",
		ArtifactUUID:          uuid,
		Name:                  buildResourceName(p.GroupName, p.GameName, sanitized),
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

func (imp *Importer) processDelete(ctx context.Context, rawLine string) fileResult {
	name := filepath.Base(strings.TrimSpace(rawLine))
	if name == "" || strings.HasPrefix(name, "#") {
		return fileResult{name, statusSkipped, "blank/comment"}
	}

	p := parsePatchFileName(name)
	if p == nil {
		return fileResult{name, statusUnrecognized, "unrecognized filename"}
	}
	exists, galgameID, err := imp.galgame.CheckGalgameByVndbID(ctx, p.VndbID)
	if err != nil {
		return fileResult{name, statusFailed, "catalog check: " + err.Error()}
	}
	if !exists {
		return fileResult{name, statusCatalogMissing, "vndb " + p.VndbID + " not in catalog"}
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
		end := min(i+80, len(ids))
		states, err := imp.galgame.ClaimStates(ctx, ids[i:end])
		if err != nil {
			slog.Warn("draft check: catalog claim lookup failed (skipping this chunk)", "err", err)
			for _, id := range ids[i:end] {
				published[id] = struct{}{}
			}
			continue
		}
		for id, state := range states {
			if state == "live" {
				published[id] = struct{}{}
			}
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
