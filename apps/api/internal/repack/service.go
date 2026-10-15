// Package repack keeps the derived copy of a hosted resource's file that
// letmoe's forge builds for merged downloads. Its lifecycle lives in migration
// 047's triggers; this package attaches, withdraws and purges.
package repack

import (
	"context"
	"errors"
	"fmt"

	"kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/artifactclient"

	"gorm.io/gorm"
)

var (
	ErrResourceNotFound = errors.New("resource not found, not live, or not a file hosted on moyu")
	ErrSourceChanged    = errors.New("the resource's file is no longer source_artifact_uuid; read /file again and repack the current one")
	ErrArtifactUnusable = errors.New("artifact cannot be attached as a repack")
)

type uploadOwners interface {
	RequireOwner(ctx context.Context, uuid string, userID int) error
}

type Service struct {
	db     *gorm.DB
	art    *artifactclient.Client
	owners uploadOwners
}

func New(db *gorm.DB, art *artifactclient.Client, owners uploadOwners) *Service {
	return &Service{db: db, art: art, owners: owners}
}

type AttachRequest struct {
	ArtifactUUID       string `json:"artifact_uuid" validate:"required,uuid"`
	SourceArtifactUUID string `json:"source_artifact_uuid" validate:"required,uuid"`
	Blake3             string `json:"blake3" validate:"required,len=64,hexadecimal,lowercase"`
	Bytes              int64  `json:"bytes" validate:"required,min=1"`
	FileCount          int    `json:"file_count" validate:"required,min=1"`
	SourceBlake3       string `json:"source_blake3" validate:"required,len=64,hexadecimal,lowercase"`
}

func unusable(reason string) error { return fmt.Errorf("%w: %s", ErrArtifactUnusable, reason) }

func (s *Service) Attach(ctx context.Context, resourceID, botUserID int, req AttachRequest) (*model.PatchResourceRepack, error) {
	if req.ArtifactUUID == req.SourceArtifactUUID {
		return nil, unusable("it is the resource's own file")
	}
	if err := s.owners.RequireOwner(ctx, req.ArtifactUUID, botUserID); err != nil {
		return nil, err
	}
	meta, err := s.art.Get(ctx, req.ArtifactUUID)
	if errors.Is(err, artifactclient.ErrNotFound) {
		return nil, unusable("no such artifact")
	}
	if err != nil {
		return nil, err
	}
	switch {
	case meta.Status != artifactclient.StatusReady:
		return nil, unusable("upload not completed")
	case !meta.Public:
		return nil, unusable("not public, so no composite can pin it")
	case meta.FileSize != req.Bytes:
		return nil, unusable(fmt.Sprintf("bytes is %d, the stored file is %d", req.Bytes, meta.FileSize))
	}

	var out model.PatchResourceRepack
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// FOR SHARE holds off a concurrent file replace until this commits;
		// that replace then drops the repack through 047's trigger.
		var res struct {
			Status       int
			ArtifactUUID string
		}
		found := tx.Raw(`SELECT status, artifact_uuid FROM patch_resource WHERE id = ? FOR SHARE`, resourceID).Scan(&res)
		if found.Error != nil {
			return found.Error
		}
		if found.RowsAffected == 0 || res.Status != 0 || res.ArtifactUUID == "" {
			return ErrResourceNotFound
		}
		if res.ArtifactUUID != req.SourceArtifactUUID {
			return ErrSourceChanged
		}
		var normalized bool
		if err := tx.Raw(`SELECT EXISTS (SELECT 1 FROM patch_resource_normalize WHERE artifact_uuid = ?)`,
			res.ArtifactUUID).Scan(&normalized).Error; err != nil {
			return err
		}
		if normalized {
			return unusable("the resource's file is already a forge zip; use it as the layer")
		}

		var holder struct{ ResourceID int }
		if err := tx.Raw(`SELECT resource_id FROM patch_resource_repack WHERE artifact_uuid = ?`, req.ArtifactUUID).
			Scan(&holder).Error; err != nil {
			return err
		}
		if holder.ResourceID == resourceID {
			return tx.Where("resource_id = ?", resourceID).First(&out).Error
		}
		if holder.ResourceID != 0 {
			return unusable(fmt.Sprintf("already the repack of resource %d", holder.ResourceID))
		}
		var taken bool
		if err := tx.Raw(`SELECT EXISTS (SELECT 1 FROM patch_resource WHERE artifact_uuid = ?)
			OR EXISTS (SELECT 1 FROM patch_resource_repack_purge WHERE artifact_uuid = ?)`,
			req.ArtifactUUID, req.ArtifactUUID).Scan(&taken).Error; err != nil {
			return err
		}
		if taken {
			return unusable("it is a resource's file, or a repack being deleted")
		}

		return tx.Raw(`INSERT INTO patch_resource_repack
				(resource_id, artifact_uuid, source_artifact_uuid, blake3, bytes, file_count, source_blake3)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (resource_id) DO UPDATE SET
				artifact_uuid = EXCLUDED.artifact_uuid,
				source_artifact_uuid = EXCLUDED.source_artifact_uuid,
				blake3 = EXCLUDED.blake3,
				bytes = EXCLUDED.bytes,
				file_count = EXCLUDED.file_count,
				source_blake3 = EXCLUDED.source_blake3,
				created_at = now()
			RETURNING *`,
			resourceID, req.ArtifactUUID, req.SourceArtifactUUID, req.Blake3, req.Bytes, req.FileCount, req.SourceBlake3,
		).Scan(&out).Error
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Withdraw drops the repack; 047's trigger queues its artifact for deletion.
func (s *Service) Withdraw(ctx context.Context, resourceID int) (bool, error) {
	res := s.db.WithContext(ctx).Exec(`DELETE FROM patch_resource_repack WHERE resource_id = ?`, resourceID)
	return res.RowsAffected > 0, res.Error
}
