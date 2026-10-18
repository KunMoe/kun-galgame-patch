package repack

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"kun-galgame-patch-api/pkg/artifactclient"
)

const (
	purgeBatch      = 50
	purgeBackoffCap = time.Hour
)

func (s *Service) Configured() bool { return s.art != nil && s.art.Configured() }

type purgeRow struct {
	ArtifactUUID string
	ResourceID   int
	Attempts     int
}

// Purge deletes the artifacts 047's triggers queued. infra deletes every letmoe
// composite pinning an artifact when moyu deletes it, so a row left here is a
// merged download still serving a patch moyu no longer has.
func (s *Service) Purge(ctx context.Context) (int, error) {
	// Reverting a normalized resource makes its original the file again while
	// the original's delete is still queued.
	if err := s.db.WithContext(ctx).Exec(`DELETE FROM patch_resource_repack_purge q
		WHERE EXISTS (SELECT 1 FROM patch_resource r WHERE r.artifact_uuid = q.artifact_uuid)`).Error; err != nil {
		return 0, err
	}
	var due []purgeRow
	if err := s.db.WithContext(ctx).Raw(`SELECT artifact_uuid, resource_id, attempts
		FROM patch_resource_repack_purge WHERE next_attempt_at <= now()
		ORDER BY next_attempt_at LIMIT ?`, purgeBatch).Scan(&due).Error; err != nil {
		return 0, err
	}
	purged := 0
	var failed error
	for _, row := range due {
		err := s.art.Delete(ctx, row.ArtifactUUID)
		if err == nil || errors.Is(err, artifactclient.ErrNotFound) {
			if dbErr := s.db.WithContext(ctx).Exec(`DELETE FROM patch_resource_repack_purge WHERE artifact_uuid = ?`,
				row.ArtifactUUID).Error; dbErr != nil {
				return purged, dbErr
			}
			purged++
			continue
		}
		if ctx.Err() != nil {
			return purged, ctx.Err()
		}
		failed = err
		wait := min(time.Minute<<min(row.Attempts, 10), purgeBackoffCap)
		slog.Warn("repack artifact delete failed; will retry",
			"artifact_uuid", row.ArtifactUUID, "resource_id", row.ResourceID,
			"attempts", row.Attempts+1, "retry_in", wait, "error", err)
		if dbErr := s.db.WithContext(ctx).Exec(`UPDATE patch_resource_repack_purge
			SET attempts = attempts + 1, last_error = ?, next_attempt_at = now() + make_interval(secs => ?)
			WHERE artifact_uuid = ?`, err.Error(), wait.Seconds(), row.ArtifactUUID).Error; dbErr != nil {
			return purged, dbErr
		}
	}
	return purged, failed
}
