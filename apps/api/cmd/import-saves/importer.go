package main

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	authModel "kun-galgame-patch-api/internal/auth/model"
	galgameClient "kun-galgame-patch-api/internal/galgame/client"
	"kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/catalogv2"
	"kun-galgame-patch-api/pkg/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var importedAt = time.Date(2024, 10, 29, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*3600))

type status string

const (
	statusOK          status = "ok"
	statusDryRun      status = "dry-run"
	statusSkipped     status = "skipped"
	statusWorkDeleted status = "work-deleted"
	statusWorkMissing status = "work-missing"
	statusFailed      status = "failed"
)

type result struct {
	assetID int64
	status  status
	msg     string
}

type catalog interface {
	GalgameBatch(ctx context.Context, ids []int, contentLimit string) ([]galgameClient.GalgameBrief, error)
	GetGalgame(ctx context.Context, gid int, contentLimit string) (*galgameClient.GalgameDetailEnvelope, error)
}

type fileStore interface {
	UploadFile(ctx context.Context, path, uploadName string, size int64, uploaderSub int) (string, int64, error)
	Delete(ctx context.Context, uuid string) error
}

type importer struct {
	db      *gorm.DB
	catalog catalog
	files   fileStore
	root    string
	userID  int
	dryRun  bool
}

func (imp *importer) importWork(ctx context.Context, workID int, recs []saveRecord) []result {
	all := func(s status, msg string) []result {
		out := make([]result, len(recs))
		for i, r := range recs {
			out[i] = result{r.AssetID, s, msg}
		}
		return out
	}
	st, err := imp.ensurePage(ctx, workID)
	if err != nil {
		return all(statusFailed, "page: "+err.Error())
	}
	if st != statusOK {
		return all(st, "")
	}
	out := make([]result, 0, len(recs))
	for _, r := range recs {
		if ctx.Err() != nil {
			break
		}
		out = append(out, imp.importSave(ctx, r))
	}
	return out
}

func (imp *importer) importSave(ctx context.Context, r saveRecord) result {
	var dup bool
	if err := imp.db.Raw(`SELECT EXISTS (SELECT 1 FROM patch_resource
		WHERE galgame_id = ? AND user_id = ? AND strpos(note, ?) > 0)`,
		r.WorkID, imp.userID, r.OriginalSHA256).Scan(&dup).Error; err != nil {
		return result{r.AssetID, statusFailed, "dedup: " + err.Error()}
	}
	if dup {
		return result{r.AssetID, statusSkipped, "already imported"}
	}
	if imp.dryRun {
		return result{r.AssetID, statusDryRun, ""}
	}
	uuid, size, err := imp.files.UploadFile(ctx, filepath.Join(imp.root, r.Path), r.Filename, r.Bytes, imp.userID)
	if err != nil {
		return result{r.AssetID, statusFailed, "upload: " + err.Error()}
	}
	if err := imp.db.Transaction(func(tx *gorm.DB) error { return imp.insertSave(tx, r, uuid, size) }); err != nil {
		_ = imp.files.Delete(context.WithoutCancel(ctx), uuid)
		return result{r.AssetID, statusFailed, "db: " + err.Error()}
	}
	return result{r.AssetID, statusOK, uuid}
}

func (imp *importer) insertSave(tx *gorm.DB, r saveRecord, uuid string, size int64) error {
	res := &model.PatchResource{
		Storage:      "s3",
		ArtifactUUID: uuid,
		Name:         resourceName(r),
		Size:         fmt.Sprintf("%.3f MB", float64(size)/(1024*1024)),
		Note:         resourceNote(r, imp.userID),
		Type:         model.JSONArray{"save"},
		Language:     model.JSONArray{"ja"},
		Platform:     model.JSONArray{"windows"},
		UserID:       imp.userID,
		GalgameID:    r.WorkID,
		UpdateTime:   importedAt,
		Created:      importedAt,
		Updated:      importedAt,
	}
	if err := tx.Create(res).Error; err != nil {
		return err
	}
	// Recomputed like RecalculatePatchAggregates, but as one statement that
	// leaves patch.updated alone: its Updates bumps it to now.
	if err := tx.Exec(`
UPDATE patch p SET resource_count = p.resource_count + 1,
	type = a.type, language = a.language, platform = a.platform
FROM (SELECT
	COALESCE((SELECT jsonb_agg(DISTINCT v ORDER BY v) FROM patch_resource r, jsonb_array_elements_text(r.type) v WHERE r.galgame_id = ?), '[]') AS type,
	COALESCE((SELECT jsonb_agg(DISTINCT v ORDER BY v) FROM patch_resource r, jsonb_array_elements_text(r.language) v WHERE r.galgame_id = ?), '[]') AS language,
	COALESCE((SELECT jsonb_agg(DISTINCT v ORDER BY v) FROM patch_resource r, jsonb_array_elements_text(r.platform) v WHERE r.galgame_id = ?), '[]') AS platform
) a WHERE p.id = ?`, r.WorkID, r.WorkID, r.WorkID, r.WorkID).Error; err != nil {
		return err
	}
	if err := imp.ensureContributor(tx, r.WorkID); err != nil {
		return err
	}
	return markBackfill(tx, "key = ?", fmt.Sprintf("patch_resource:%d", res.ID))
}

// markBackfill keeps the community push of what this job wrote from
// notifying anyone: a backfill row never notifies.
func markBackfill(tx *gorm.DB, where string, args ...any) error {
	return tx.Exec(`UPDATE activity_push_queue SET backfill = true WHERE `+where, args...).Error
}

func (imp *importer) ensureContributor(tx *gorm.DB, workID int) error {
	rel := model.UserPatchContributeRelation{UserID: imp.userID, GalgameID: workID, Created: importedAt, Updated: importedAt}
	res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rel)
	if res.Error != nil || res.RowsAffected == 0 {
		return res.Error
	}
	return tx.Exec(`UPDATE patch SET contribute_count = contribute_count + 1 WHERE id = ?`, workID).Error
}

type pageState struct {
	ID               int
	IsStub           bool
	CatalogDeletedAt *time.Time
}

// ensurePage creates the work's page when it has none and adopts a stub, both
// dated importedAt and published unless a claim event hid the work; a page
// that already holds resources is left as it is.
func (imp *importer) ensurePage(ctx context.Context, workID int) (status, error) {
	var page pageState
	err := imp.db.Raw(`SELECT id, is_stub, catalog_deleted_at FROM patch WHERE id = ?`, workID).Scan(&page).Error
	if err != nil {
		return "", err
	}
	if page.ID != 0 {
		if page.CatalogDeletedAt != nil {
			return statusWorkDeleted, nil
		}
		if !page.IsStub || imp.dryRun {
			return statusOK, nil
		}
		return statusOK, imp.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(`UPDATE patch SET user_id = ?, is_stub = false, published = `+publishedUnlessHidden+`,
				created = ?, updated = ?, resource_update_time = ? WHERE id = ? AND is_stub`,
				imp.userID, workID, catalogv2.ClaimStateHidden, importedAt, importedAt, importedAt, workID).Error; err != nil {
				return err
			}
			if err := imp.ensureContributor(tx, workID); err != nil {
				return err
			}
			return markBackfill(tx, "key IN (SELECT 'patch_resource:' || id FROM patch_resource WHERE galgame_id = ?)", workID)
		})
	}

	briefs, err := imp.catalog.GalgameBatch(ctx, []int{workID}, "")
	if err != nil {
		return "", err
	}
	var brief *galgameClient.GalgameBrief
	for i := range briefs {
		if briefs[i].ID == workID {
			brief = &briefs[i]
		}
	}
	if brief == nil {
		return statusWorkMissing, nil
	}
	if imp.dryRun {
		return statusOK, nil
	}
	vndb := brief.VndbID
	if vndb == "" {
		vndb = fmt.Sprintf("wiki-%d", workID)
	}
	var releaseDate *time.Time
	if env, err := imp.catalog.GetGalgame(ctx, workID, ""); err == nil && env != nil && env.Galgame.ReleaseDate != nil {
		releaseDate = utils.ParseGalgameReleaseDate(*env.Galgame.ReleaseDate)
	}
	err = imp.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&authModel.User{ID: imp.userID}).Error; err != nil {
			return err
		}
		var published bool
		if err := tx.Raw(`SELECT `+publishedUnlessHidden, workID, catalogv2.ClaimStateHidden).Scan(&published).Error; err != nil {
			return err
		}
		p := &model.Patch{
			ID: workID, VndbID: vndb, UserID: imp.userID, ReleaseDate: releaseDate, Published: published,
			ResourceUpdateTime: importedAt, Created: importedAt, Updated: importedAt,
		}
		if err := tx.Create(p).Error; err != nil {
			return err
		}
		return imp.ensureContributor(tx, workID)
	})
	if err != nil {
		var again pageState
		if e := imp.db.Raw(`SELECT id FROM patch WHERE id = ?`, workID).Scan(&again).Error; e == nil && again.ID != 0 {
			return statusOK, nil
		}
		return "", err
	}
	return statusOK, nil
}

// The same rule as PatchRepository.MarkIndexed: a work whose latest claim
// event hid it stays unpublished.
const publishedUnlessHidden = `COALESCE((SELECT to_state FROM claim_event_processed
	WHERE work_id = ? ORDER BY event_id DESC LIMIT 1), '') <> ?`
