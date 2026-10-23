package normalize

import (
	"context"
	"errors"

	"kun-galgame-patch-api/pkg/artifactclient"

	"gorm.io/gorm"
)

type CommentSource struct {
	ResourceID         int    `json:"resource_id"`
	SourceArtifactUUID string `json:"source_artifact_uuid"`
	Filename           string `json:"filename"`
	DownloadURL        string `json:"download_url"`
	Bytes              int64  `json:"bytes"`
}

// CommentSources lists the sources whose comment was never read and that still
// exist: the resource's file, or an original waiting out keepOriginal. next is
// the last uuid scanned, "" once the list is exhausted; a source the artifact
// service no longer holds is skipped, so the page can be shorter than next
// implies.
func (s *Service) CommentSources(ctx context.Context, limit int, after string) ([]CommentSource, string, error) {
	var rows []struct {
		ResourceID         int
		SourceArtifactUUID string
	}
	if err := s.db.WithContext(ctx).Raw(`SELECT n.resource_id, n.source_artifact_uuid
		FROM patch_resource_normalize n
		WHERE n.state IN ('done', 'failed') AND n.source_comment IS NULL AND n.source_artifact_uuid > ?
			AND (EXISTS (SELECT 1 FROM patch_resource r WHERE r.artifact_uuid = n.source_artifact_uuid)
				OR EXISTS (SELECT 1 FROM patch_resource_repack_purge q WHERE q.artifact_uuid = n.source_artifact_uuid))
		ORDER BY n.source_artifact_uuid
		LIMIT ?`, after, limit).Scan(&rows).Error; err != nil {
		return nil, "", err
	}
	out := make([]CommentSource, 0, len(rows))
	for _, row := range rows {
		meta, err := s.art.Get(ctx, row.SourceArtifactUUID)
		if errors.Is(err, artifactclient.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, "", err
		}
		dl, err := s.art.Download(ctx, row.SourceArtifactUUID)
		if errors.Is(err, artifactclient.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, "", err
		}
		out = append(out, CommentSource{
			ResourceID:         row.ResourceID,
			SourceArtifactUUID: row.SourceArtifactUUID,
			Filename:           meta.Name,
			DownloadURL:        dl.Url,
			Bytes:              meta.FileSize,
		})
	}
	next := ""
	if len(rows) == limit {
		next = rows[len(rows)-1].SourceArtifactUUID
	}
	return out, next, nil
}

type CommentRequest struct {
	SourceArtifactUUID string `json:"source_artifact_uuid" validate:"required,uuid"`
	ArchiveComment     string `json:"archive_comment"`
}

// RecordComment stores what the forge read from a job's source, and shows it on
// the resource while the resource's file is still that source or its product.
func (s *Service) RecordComment(ctx context.Context, resourceID int, req CommentRequest) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job struct {
			State        string
			ArtifactUUID string
		}
		res := tx.Raw(`UPDATE patch_resource_normalize SET source_comment = ?, updated_at = now()
			WHERE source_artifact_uuid = ? AND resource_id = ?
			RETURNING state, artifact_uuid`, req.ArchiveComment, req.SourceArtifactUUID, resourceID).Scan(&job)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrNoJob
		}
		file := req.SourceArtifactUUID
		if job.State == "done" {
			file = job.ArtifactUUID
		}
		return tx.Exec(`UPDATE patch_resource SET archive_comment = ? WHERE id = ? AND artifact_uuid = ?`,
			req.ArchiveComment, resourceID, file).Error
	})
}
