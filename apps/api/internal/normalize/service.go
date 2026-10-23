// Package normalize swaps every hosted resource's file for the passwordless
// zip letmoe's forge builds from it (owner's ruling, 2026-10-15). The jobs and
// the two trigger changes the swap depends on are migration 050's.
package normalize

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/artifactclient"
	"kun-galgame-patch-api/pkg/utils"

	"gorm.io/gorm"
)

const (
	leaseTTL    = 6 * time.Hour
	maxAttempts = 5
	MaxClaim    = 20
	// The original is the author's only copy, and a filename-decoding mistake
	// in a forge zip can surface weeks later: 1,243 of 4,178 forge packages
	// shipped 2026-09-29 were unreadable on some locales.
	keepOriginal = 30 * 24 * time.Hour
	// moyu's artifact quota is 3,000 Inits a UTC day, readers' uploads
	// included, and every claim becomes one Init.
	dailyClaimCap   = 2400
	maxCommentBytes = 16384
	MaxCommentList  = 50
)

var (
	ErrResourceNotFound = errors.New("resource not found, or not a file hosted on moyu")
	ErrSourceChanged    = errors.New("the resource's file is no longer source_artifact_uuid; moyu deletes the upload")
	ErrAlreadyDone      = errors.New("source_artifact_uuid is already normalized")
	ErrNoJob            = errors.New("no open job for source_artifact_uuid on this resource")
	ErrArtifactUnusable = errors.New("artifact cannot become the resource's file")
	ErrBadComment       = errors.New("archive_comment must be valid UTF-8 without NUL, at most 16384 bytes")
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

func unusable(reason string) error { return fmt.Errorf("%w: %s", ErrArtifactUnusable, reason) }

type Repack struct {
	ArtifactUUID string `json:"artifact_uuid"`
	Blake3       string `json:"blake3"`
	Bytes        int64  `json:"bytes"`
	FileCount    int    `json:"file_count"`
}

type Job struct {
	ResourceID         int       `json:"resource_id"`
	GalgameID          int       `json:"galgame_id"`
	ArtifactUUID       string    `json:"artifact_uuid"`
	DownloadURL        string    `json:"download_url"`
	Filename           string    `json:"filename"`
	Password           string    `json:"password"`
	PasswordCandidates []string  `json:"password_candidates"`
	Note               string    `json:"note"`
	Bytes              int64     `json:"bytes"`
	Repack             *Repack   `json:"repack"`
	LeaseExpiresAt     time.Time `json:"lease_expires_at"`
}

type claimedRow struct {
	ResourceID      int
	GalgameID       int
	ArtifactUUID    string
	Password        string
	Note            string
	LeaseUntil      time.Time
	RepackUUID      *string
	RepackBlake3    *string
	RepackBytes     *int64
	RepackFileCount *int
}

// Claim leases up to limit pending files, promotions of a resource's repack
// first. ids, when given, narrows the claim to those resources.
func (s *Service) Claim(ctx context.Context, limit int, ids []int) ([]Job, error) {
	var rows []claimedRow
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Claims take turns. A row lock on the resource did not stop forge's
		// parallel claimers on 2026-10-15: the claim writes no resource row, so
		// a claimer that read the queue before another committed found the
		// lock free and leased the same file again (4,902 claims for 2,385
		// resources). Each statement after this lock reads every earlier claim.
		if err := tx.Exec(`SELECT pg_advisory_xact_lock(hashtext('moyu:normalize:claim'))`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`UPDATE patch_resource_normalize
			SET state = 'failed', code = 'abandoned', message = ?, lease_until = NULL, updated_at = now()
			WHERE state = 'claimed' AND lease_until < now() AND attempts >= ?`,
			fmt.Sprintf("the lease ran out %d times", maxAttempts), maxAttempts).Error; err != nil {
			return err
		}
		promotions, err := lease(tx, true, limit, ids)
		if err != nil {
			return err
		}
		rows = promotions
		var today int
		if err := tx.Raw(`SELECT count(*) FROM patch_resource_normalize
			WHERE claimed_at >= date_trunc('day', now(), 'UTC')`).Scan(&today).Error; err != nil {
			return err
		}
		fresh := min(limit-len(rows), dailyClaimCap-today)
		if fresh <= 0 {
			return nil
		}
		uploads, err := lease(tx, false, fresh, ids)
		rows = append(rows, uploads...)
		return err
	})
	if err != nil {
		return nil, err
	}

	candidates, err := s.passwordCandidates(ctx, rows)
	if err != nil {
		s.release(rows)
		return nil, err
	}
	jobs := make([]Job, 0, len(rows))
	for i, row := range rows {
		job, err := s.describe(ctx, row, candidates[row.ResourceID])
		if errors.Is(err, artifactclient.ErrNotFound) {
			if err := s.markFailed(ctx, row.ResourceID, row.ArtifactUUID, "source_missing",
				"the artifact service does not hold this file"); err != nil {
				slog.Warn("normalize: recording a missing source failed", "resource_id", row.ResourceID, "error", err)
			}
			continue
		}
		if err != nil {
			s.release(rows[i:])
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// lease claims pending files whose resource does or does not carry a repack.
// A promotion uploads nothing, so it leaves claimed_at empty and costs none of
// dailyClaimCap.
func lease(tx *gorm.DB, promotions bool, limit int, ids []int) ([]claimedRow, error) {
	filter, args := "", []any{promotions}
	if len(ids) > 0 {
		filter = "AND r.id IN ?"
		args = append(args, ids)
	}
	args = append(args, limit, leaseTTL.Seconds(), promotions)
	var rows []claimedRow
	// NOT IN keeps each job lookup a single hashed scan. As joins, the planner
	// expected one row out of them and probed the job table with a seq scan per
	// resource: 27.8s per claim in production on 2026-10-23, inside the claim
	// lock, which queued every other claimer behind it.
	err := tx.Raw(`WITH due AS (
			SELECT r.id, r.galgame_id, r.artifact_uuid, r.password, r.note
			FROM patch_resource r
			WHERE r.storage = 's3' AND r.status = 0 AND r.artifact_uuid <> ''
				AND `+utils.LiveWorkResource("r")+`
				AND r.artifact_uuid NOT IN (SELECT source_artifact_uuid FROM patch_resource_normalize
					WHERE state <> 'claimed' OR lease_until IS NULL OR lease_until >= now())
				AND r.artifact_uuid NOT IN (SELECT artifact_uuid FROM patch_resource_normalize)
				AND (r.id IN (SELECT resource_id FROM patch_resource_repack)) = ?
				`+filter+`
			ORDER BY r.update_time DESC, r.id DESC
			LIMIT ?
		), leased AS (
			INSERT INTO patch_resource_normalize
				(source_artifact_uuid, resource_id, state, attempts, lease_until, claimed_at)
			SELECT artifact_uuid, id, 'claimed', 1, now() + make_interval(secs => ?),
				CASE WHEN ? THEN NULL ELSE now() END
			FROM due
			ON CONFLICT (source_artifact_uuid) DO UPDATE SET
				state = 'claimed',
				attempts = patch_resource_normalize.attempts + 1,
				lease_until = EXCLUDED.lease_until,
				claimed_at = EXCLUDED.claimed_at,
				updated_at = now()
			WHERE patch_resource_normalize.state = 'claimed' AND patch_resource_normalize.lease_until < now()
			RETURNING resource_id, source_artifact_uuid, lease_until
		)
		SELECT l.resource_id, d.galgame_id, l.source_artifact_uuid AS artifact_uuid, d.password, d.note, l.lease_until,
			rp.artifact_uuid AS repack_uuid, rp.blake3 AS repack_blake3,
			rp.bytes AS repack_bytes, rp.file_count AS repack_file_count
		FROM leased l
		JOIN due d ON d.id = l.resource_id
		LEFT JOIN patch_resource_repack rp ON rp.resource_id = l.resource_id`,
		args...).Scan(&rows).Error
	return rows, err
}

// passwordCandidates offers the passwords of a resource's earlier files: 108
// files failed wrong_password by 2026-10-23 with an empty password field, and
// on resource 2011 the author's replacement was encrypted with the previous
// file's password, which the swap had cleared.
func (s *Service) passwordCandidates(ctx context.Context, rows []claimedRow) (map[int][]string, error) {
	ids := make([]int, len(rows))
	for i, r := range rows {
		ids[i] = r.ResourceID
	}
	var found []struct {
		ResourceID     int
		SourcePassword string
	}
	if len(ids) > 0 {
		if err := s.db.WithContext(ctx).Raw(`SELECT resource_id, source_password FROM patch_resource_normalize
			WHERE resource_id IN ? AND source_password <> ''
			ORDER BY created_at DESC, source_artifact_uuid`, ids).Scan(&found).Error; err != nil {
			return nil, err
		}
	}
	own := make(map[int]string, len(rows))
	for _, r := range rows {
		own[r.ResourceID] = r.Password
	}
	out := make(map[int][]string, len(rows))
	for _, f := range found {
		if f.SourcePassword == own[f.ResourceID] || slices.Contains(out[f.ResourceID], f.SourcePassword) {
			continue
		}
		out[f.ResourceID] = append(out[f.ResourceID], f.SourcePassword)
	}
	return out, nil
}

func (s *Service) describe(ctx context.Context, row claimedRow, candidates []string) (Job, error) {
	meta, err := s.art.Get(ctx, row.ArtifactUUID)
	if err != nil {
		return Job{}, err
	}
	dl, err := s.art.Download(ctx, row.ArtifactUUID)
	if err != nil {
		return Job{}, err
	}
	job := Job{
		ResourceID:         row.ResourceID,
		GalgameID:          row.GalgameID,
		ArtifactUUID:       row.ArtifactUUID,
		DownloadURL:        dl.Url,
		Filename:           meta.Name,
		Password:           row.Password,
		PasswordCandidates: candidates,
		Note:               row.Note,
		Bytes:              meta.FileSize,
		LeaseExpiresAt:     row.LeaseUntil,
	}
	if job.PasswordCandidates == nil {
		job.PasswordCandidates = []string{}
	}
	if row.RepackUUID != nil {
		job.Repack = &Repack{
			ArtifactUUID: *row.RepackUUID,
			Blake3:       *row.RepackBlake3,
			Bytes:        *row.RepackBytes,
			FileCount:    *row.RepackFileCount,
		}
	}
	return job, nil
}

// release hands back leases the forge never received, so an artifact service
// outage does not hold them for leaseTTL or count against the day.
func (s *Service) release(rows []claimedRow) {
	uuids := make([]string, len(rows))
	for i, r := range rows {
		uuids[i] = r.ArtifactUUID
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.db.WithContext(ctx).Exec(`UPDATE patch_resource_normalize
		SET lease_until = now(), attempts = attempts - 1, claimed_at = NULL, updated_at = now()
		WHERE source_artifact_uuid IN ? AND state = 'claimed'`, uuids).Error; err != nil {
		slog.Warn("normalize: releasing unsent leases failed", "count", len(uuids), "error", err)
	}
}

type CompleteRequest struct {
	ArtifactUUID       string `json:"artifact_uuid" validate:"required,uuid"`
	SourceArtifactUUID string `json:"source_artifact_uuid" validate:"required,uuid"`
	Filename           string `json:"filename" validate:"required,max=255"`
	Bytes              int64  `json:"bytes" validate:"required,min=1"`
	Blake3             string `json:"blake3" validate:"required,len=64,hexadecimal,lowercase"`
	FileCount          int    `json:"file_count" validate:"required,min=1"`
	HasArchives        bool   `json:"has_archives"`
	Password           string `json:"password" validate:"max=1007"`
	// Absent means the forge did not read the source; "" means it read no
	// comment. A forge predating 055 sends neither, and reading that as ""
	// would hide its files from the comment backfill.
	ArchiveComment *string `json:"archive_comment"`
}

func CheckComment(c string) error {
	if len(c) > maxCommentBytes || !utf8.ValidString(c) || strings.ContainsRune(c, 0) {
		return ErrBadComment
	}
	return nil
}

// displaySize matches what the publish form writes after an upload.
func displaySize(bytes int64) string {
	return fmt.Sprintf("%.3f MB", float64(bytes)/(1024*1024))
}

func (s *Service) Complete(ctx context.Context, resourceID, botUserID int, req CompleteRequest) (*model.PatchResourceNormalize, error) {
	if req.ArtifactUUID == req.SourceArtifactUUID {
		return nil, unusable("it is the source file itself")
	}
	if !strings.HasSuffix(strings.ToLower(req.Filename), ".zip") || strings.ContainsAny(req.Filename, `/\`) {
		return nil, unusable(fmt.Sprintf("filename %q is not a bare name ending in .zip", req.Filename))
	}
	db := s.db.WithContext(ctx)

	var prior []model.PatchResourceNormalize
	if err := db.Where("source_artifact_uuid = ? AND state = 'done'", req.SourceArtifactUUID).
		Limit(1).Find(&prior).Error; err != nil {
		return nil, err
	}
	if len(prior) > 0 {
		if prior[0].ResourceID != resourceID || prior[0].ArtifactUUID != req.ArtifactUUID {
			return nil, ErrAlreadyDone
		}
		meta, err := s.art.Get(ctx, req.ArtifactUUID)
		if err != nil {
			return nil, err
		}
		if err := s.rename(ctx, meta, req.Filename); err != nil {
			return nil, err
		}
		return &prior[0], nil
	}

	var promoting bool
	if err := db.Raw(`SELECT EXISTS (SELECT 1 FROM patch_resource_repack WHERE resource_id = ? AND artifact_uuid = ?)`,
		resourceID, req.ArtifactUUID).Scan(&promoting).Error; err != nil {
		return nil, err
	}
	if !promoting {
		if err := s.owners.RequireOwner(ctx, req.ArtifactUUID, botUserID); err != nil {
			return nil, err
		}
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
		return nil, unusable("not public, so readers could not download it")
	case meta.FileSize != req.Bytes:
		return nil, unusable(fmt.Sprintf("bytes is %d, the stored file is %d", req.Bytes, meta.FileSize))
	case !strings.HasSuffix(strings.ToLower(meta.Name), ".zip"):
		return nil, unusable(fmt.Sprintf("its name %q does not end in .zip", meta.Name))
	}

	var out model.PatchResourceNormalize
	err = db.Transaction(func(tx *gorm.DB) error {
		var res struct {
			Storage      string
			ArtifactUUID string
			Password     string
			Size         string
		}
		found := tx.Raw(`SELECT storage, artifact_uuid, password, size FROM patch_resource WHERE id = ? FOR UPDATE`,
			resourceID).Scan(&res)
		if found.Error != nil {
			return found.Error
		}
		if found.RowsAffected == 0 || res.Storage != "s3" {
			return ErrResourceNotFound
		}
		if res.ArtifactUUID != req.SourceArtifactUUID {
			return ErrSourceChanged
		}

		var taken bool
		if err := tx.Raw(`SELECT EXISTS (SELECT 1 FROM patch_resource WHERE artifact_uuid = ?)
			OR EXISTS (SELECT 1 FROM patch_resource_repack WHERE artifact_uuid = ? AND resource_id <> ?)
			OR EXISTS (SELECT 1 FROM patch_resource_repack_purge WHERE artifact_uuid = ?)
			OR EXISTS (SELECT 1 FROM patch_resource_normalize WHERE artifact_uuid = ?)`,
			req.ArtifactUUID, req.ArtifactUUID, resourceID, req.ArtifactUUID, req.ArtifactUUID).Scan(&taken).Error; err != nil {
			return err
		}
		if taken {
			return unusable("it is a resource's file, another resource's repack, or being deleted")
		}

		opened := req.Password
		if opened == "" {
			opened = res.Password
		}
		var comment string
		if req.ArchiveComment != nil {
			comment = *req.ArchiveComment
		}
		// The job is written first: 050's change-feed trigger reads it to log
		// the swap below as `normalized`.
		if err := tx.Raw(`INSERT INTO patch_resource_normalize
				(source_artifact_uuid, resource_id, state, artifact_uuid, blake3, bytes, file_count,
				 has_archives, source_password, source_size, source_comment)
			VALUES (?, ?, 'done', ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (source_artifact_uuid) DO UPDATE SET
				state = 'done',
				lease_until = NULL,
				artifact_uuid = EXCLUDED.artifact_uuid,
				blake3 = EXCLUDED.blake3,
				bytes = EXCLUDED.bytes,
				file_count = EXCLUDED.file_count,
				has_archives = EXCLUDED.has_archives,
				source_password = EXCLUDED.source_password,
				source_size = EXCLUDED.source_size,
				source_comment = EXCLUDED.source_comment,
				code = '',
				message = '',
				updated_at = now()
			RETURNING *`,
			req.SourceArtifactUUID, resourceID, req.ArtifactUUID, req.Blake3, req.Bytes, req.FileCount,
			req.HasArchives, opened, res.Size, req.ArchiveComment,
		).Scan(&out).Error; err != nil {
			return err
		}

		// Raw SQL, so GORM's autoUpdateTime leaves `updated` alone: the owner
		// ruled that nothing but the file changes.
		if err := tx.Exec(`UPDATE patch_resource
			SET artifact_uuid = ?, size = ?, blake3 = ?, password = CASE WHEN ? THEN ? ELSE '' END, archive_comment = ?
			WHERE id = ?`,
			req.ArtifactUUID, displaySize(req.Bytes), req.Blake3, req.HasArchives, opened, comment, resourceID).Error; err != nil {
			return err
		}
		return tx.Exec(`INSERT INTO patch_resource_repack_purge (artifact_uuid, resource_id, next_attempt_at)
			VALUES (?, ?, now() + make_interval(secs => ?))
			ON CONFLICT (artifact_uuid) DO NOTHING`, req.SourceArtifactUUID, resourceID, keepOriginal.Seconds()).Error
	})
	if errors.Is(err, ErrSourceChanged) || errors.Is(err, ErrResourceNotFound) {
		s.discard(ctx, resourceID, req.ArtifactUUID)
	}
	if err != nil {
		return nil, err
	}
	if err := s.rename(ctx, meta, req.Filename); err != nil {
		return nil, err
	}
	return &out, nil
}

// rename runs after the swap, so it can only touch a file the swap verified as
// this resource's product. A failure leaves the swap in place; the forge
// replays complete, which lands here again.
func (s *Service) rename(ctx context.Context, meta *artifactclient.ArtifactResponse, name string) error {
	if meta.Name == name {
		return nil
	}
	if _, err := s.art.Rename(ctx, meta.Uuid, name); err != nil {
		return fmt.Errorf("rename %s to %q after the swap: %w", meta.Uuid, name, err)
	}
	return nil
}

// discard queues a product whose resource was deleted or whose source was
// replaced while the forge built it. Nothing else holds it, and the forge has
// no lane to delete its own uploads.
func (s *Service) discard(ctx context.Context, resourceID int, uuid string) {
	if err := s.db.WithContext(context.WithoutCancel(ctx)).Exec(`INSERT INTO patch_resource_repack_purge (artifact_uuid, resource_id)
		SELECT ?, ?
		WHERE NOT EXISTS (SELECT 1 FROM patch_resource WHERE artifact_uuid = ?)
			AND NOT EXISTS (SELECT 1 FROM patch_resource_repack WHERE artifact_uuid = ?)
		ON CONFLICT (artifact_uuid) DO NOTHING`, uuid, resourceID, uuid, uuid).Error; err != nil {
		slog.Warn("normalize: queueing a stale product for deletion failed",
			"resource_id", resourceID, "artifact_uuid", uuid, "error", err)
	}
}

type FailRequest struct {
	SourceArtifactUUID string `json:"source_artifact_uuid" validate:"required,uuid"`
	Code               string `json:"code" validate:"required,max=64"`
	Message            string `json:"message" validate:"max=2000"`
}

// Fail is terminal for the file: it is offered again only once the author
// replaces it or an operator deletes the job.
func (s *Service) Fail(ctx context.Context, resourceID int, req FailRequest) error {
	return s.markFailed(ctx, resourceID, req.SourceArtifactUUID, req.Code, req.Message)
}

func (s *Service) markFailed(ctx context.Context, resourceID int, source, code, message string) error {
	var prior []string
	res := s.db.WithContext(ctx).Raw(`UPDATE patch_resource_normalize n
		SET state = 'failed', code = ?, message = ?, lease_until = NULL, updated_at = now()
		FROM patch_resource_normalize o
		WHERE o.source_artifact_uuid = n.source_artifact_uuid
			AND n.source_artifact_uuid = ? AND n.resource_id = ? AND n.state <> 'done'
		RETURNING o.state`,
		code, message, source, resourceID).Scan(&prior)
	if res.Error != nil {
		return res.Error
	}
	if len(prior) > 0 {
		if code == "wrong_password" && prior[0] == "claimed" {
			s.notifyWrongPassword(ctx, resourceID, source)
		}
		return nil
	}
	var done bool
	if err := s.db.WithContext(ctx).Raw(`SELECT EXISTS (SELECT 1 FROM patch_resource_normalize
		WHERE source_artifact_uuid = ? AND resource_id = ? AND state = 'done')`, source, resourceID).
		Scan(&done).Error; err != nil {
		return err
	}
	if done {
		return ErrAlreadyDone
	}
	return ErrNoJob
}

// notifyWrongPassword tells the author once per file: a failure is terminal,
// and only a claimed job turns into one.
func (s *Service) notifyWrongPassword(ctx context.Context, resourceID int, source string) {
	err := s.db.WithContext(context.WithoutCancel(ctx)).Exec(`INSERT INTO user_message
			(type, content, status, link, sender_id, recipient_id, created, updated)
		SELECT 'system',
			'您发布的补丁资源' || CASE WHEN r.name = '' THEN '' ELSE '「' || r.name || '」' END ||
			CASE WHEN r.password = '' THEN '是加密压缩包，但没有填写解压密码'
				ELSE '填写的解压密码无法解开压缩包' END ||
			'，下载的用户无法解压。请编辑资源，填写正确的解压密码。',
			0, '/resource/' || r.id, NULL, r.user_id, now(), now()
		FROM patch_resource r
		WHERE r.id = ? AND r.artifact_uuid = ?`, resourceID, source).Error
	if err != nil {
		slog.Warn("normalize: notifying the author of a wrong password failed", "resource_id", resourceID, "error", err)
	}
}
