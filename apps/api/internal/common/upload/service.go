// Package upload encapsulates the patch-resource upload flow. The bytes live in
// the centralized artifact service (kun-galgame-infra); this package only drives
// the small init/complete/abort JSON calls (the browser PUTs straight to B2 via
// the presigned URLs artifact returns) and keeps the moyu-side business rules
// the artifact service does NOT know about: per-USER daily quota (artifact only
// has a per-SITE quota), allowed extensions, and complete-idempotency.
//
// One server-driven flow: Init → (single PUT | multipart parts, as artifact
// decides) → Complete. After Complete the frontend calls
// POST /api/patch/:id/resource with the returned artifact_uuid to persist the
// record. daily_upload_size is decremented at Complete based on the verified
// size artifact reports (HeadObject is done server-side by artifact).
package upload

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	authModel "kun-galgame-patch-api/internal/auth/model"
	"kun-galgame-patch-api/internal/constants"
	"kun-galgame-patch-api/pkg/artifactclient"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Service combines the artifact client, DB and Redis. Redis is an idempotency
// guard for Complete (MOYU-PR7 / M5): without it, completing the same upload
// twice would double-deduct daily_upload_size. We can't dedupe via
// patch_resource (created in a separate later call); a 24h SETNX keyed by the
// artifact uuid is the cheapest correct fix.
type Service struct {
	art *artifactclient.Client
	db  *gorm.DB
	rdb *redis.Client
}

// New constructs a Service. rdb may be nil in tests (idempotency then degrades
// to best-effort).
func New(art *artifactclient.Client, db *gorm.DB, rdb *redis.Client) *Service {
	return &Service{art: art, db: db, rdb: rdb}
}

func ptr[T any](v T) *T { return &v }

// completeOnceTTL covers the daily-quota reset window.
const completeOnceTTL = 24 * time.Hour

// markCompleteOnce returns true if THIS call is the first complete for the given
// artifact uuid, false if a prior call already deducted quota. Nil rdb (tests)
// always returns true. Uses SetArgs NX (SetNX is deprecated since 2.6.12).
func (s *Service) markCompleteOnce(ctx context.Context, uuid string) (bool, error) {
	if s.rdb == nil {
		return true, nil
	}
	key := "upload:complete:" + uuid
	res, err := s.rdb.SetArgs(ctx, key, "1", redis.SetArgs{TTL: completeOnceTTL, Mode: "NX"}).Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}
	return res == "OK", nil
}

// unmarkComplete releases the idempotency marker when a complete attempt set it
// but failed before the quota deduction committed (so a retry can re-run it).
func (s *Service) unmarkComplete(uuid string) {
	if s.rdb == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s.rdb.Del(ctx, "upload:complete:"+uuid)
}

const oneGiB int64 = 1024 * 1024 * 1024

// validatePreUpload pre-checks at init time: extension, per-role single-file cap,
// and (unless the tier is unlimited) the per-user daily quota based on the
// declared size. tier is resolved by the handler from the OAuth roles claim.
func (s *Service) validatePreUpload(userID int, fileName string, declaredSize int64, tier constants.UploadTier) error {
	if declaredSize <= 0 || declaredSize > tier.MaxFileSize {
		return fmt.Errorf("文件大小超过 %d GB 上限", tier.MaxFileSize/oneGiB)
	}
	ext := strings.ToLower(filepath.Ext(fileName))
	if !slices.Contains(constants.AllowedResourceExtensions, ext) {
		return fmt.Errorf("不支持的文件类型: %s", ext)
	}
	if tier.DailyLimit == constants.UnlimitedDailyUpload {
		return nil
	}
	var user authModel.User
	if err := s.db.Select("daily_upload_size").First(&user, userID).Error; err != nil {
		return fmt.Errorf("获取用户信息失败")
	}
	if user.DailyUploadSize+declaredSize > tier.DailyLimit {
		return fmt.Errorf("超过今日上传限额 (%d GB)", tier.DailyLimit/oneGiB)
	}
	return nil
}

// dailyImageLimit is moyu's per-USER daily image-upload cap. image_service
// enforces only a per-SITE quota, so moyu keeps this per-user fair-use limit
// itself (aligned with kungal's image service). Reset nightly by the cron that
// zeroes daily_image_count on the user table.
const dailyImageLimit = 50

// CheckDailyImageQuota returns an error if the user has already hit the per-user
// daily image cap. Checked BEFORE hitting image_service so the user gets a
// friendly message without spending a per-site quota unit.
func (s *Service) CheckDailyImageQuota(userID int) error {
	var user authModel.User
	if err := s.db.Select("daily_image_count").First(&user, userID).Error; err != nil {
		return fmt.Errorf("获取用户信息失败")
	}
	if user.DailyImageCount >= dailyImageLimit {
		return fmt.Errorf("今日图片上传次数已达上限")
	}
	return nil
}

// IncrementDailyImageCount bumps the per-user counter after a SUCCESSFUL upload.
// Best-effort: a counter miss must not fail an upload that already landed in
// image_service.
func (s *Service) IncrementDailyImageCount(userID int) {
	s.db.Model(&authModel.User{}).Where("id = ?", userID).
		Update("daily_image_count", gorm.Expr("daily_image_count + 1"))
}

// Init validates, then asks the artifact service to start an upload. Artifact
// returns the presigned single-PUT URL or the multipart parts (server-driven).
func (s *Service) Init(ctx context.Context, userID int, tier constants.UploadTier, req InitRequest) (*InitResponse, error) {
	if err := s.validatePreUpload(userID, req.FileName, req.FileSize, tier); err != nil {
		return nil, err
	}

	in := artifactclient.InitUploadRequest{
		Name:        req.FileName,
		FileSize:    req.FileSize,
		Public:      ptr(true), // patch downloads are public (served via the CDN domain)
		UploaderSub: ptr(strconv.Itoa(userID)),
	}
	if req.MimeType != "" {
		in.MimeType = ptr(req.MimeType)
	}

	res, err := s.art.InitUpload(ctx, in)
	if err != nil {
		return nil, mapArtifactErr(err)
	}

	resp := &InitResponse{
		ArtifactUUID: res.Uuid,
		Multipart:    res.Multipart,
		ExpiresAt:    res.ExpiresAt,
	}
	if res.Multipart {
		if res.PartSize != nil {
			resp.PartSize = *res.PartSize
		}
		if res.PartUrls != nil {
			for _, p := range *res.PartUrls {
				resp.Parts = append(resp.Parts, PartURL{PartNumber: int(p.PartNumber), URL: p.Url})
			}
		}
	} else if res.UploadUrl != nil {
		resp.UploadURL = *res.UploadUrl
	}
	return resp, nil
}

// Complete finalizes the upload via artifact (which HeadObject-verifies the size
// server-side), then deducts the per-user daily quota once (idempotent).
func (s *Service) Complete(ctx context.Context, userID int, tier constants.UploadTier, req CompleteRequest) (*CompleteResponse, error) {
	var cr artifactclient.CompleteUploadRequest
	if len(req.Parts) > 0 {
		parts := make([]artifactclient.CompletedPart, 0, len(req.Parts))
		for _, p := range req.Parts {
			parts = append(parts, artifactclient.CompletedPart{PartNumber: int32(p.PartNumber), Etag: p.ETag})
		}
		cr.Parts = &parts
	}

	art, err := s.art.CompleteUpload(ctx, req.ArtifactUUID, cr)
	if err != nil {
		return nil, mapArtifactErr(err)
	}

	// artifact verified actual == declared server-side; art.FileSize is the
	// verified size. Deduct the per-user daily quota exactly once.
	size := art.FileSize
	if err := s.deductQuotaOnce(ctx, userID, req.ArtifactUUID, size, tier); err != nil {
		return nil, err
	}
	return &CompleteResponse{ArtifactUUID: req.ArtifactUUID, Size: size}, nil
}

// deductQuotaOnce increments daily_upload_size once per artifact uuid (Redis
// SETNX, 24h TTL). If the user is over quota at this point the artifact is
// soft-deleted (it's already uploaded) and an error is returned.
func (s *Service) deductQuotaOnce(ctx context.Context, userID int, uuid string, size int64, tier constants.UploadTier) error {
	first, err := s.markCompleteOnce(ctx, uuid)
	if err != nil {
		return fmt.Errorf("complete 幂等校验失败: %w", err)
	}
	if !first {
		return nil // already deducted on a prior complete
	}
	deducted := false
	defer func() {
		if !deducted {
			s.unmarkComplete(uuid)
		}
	}()

	// Unlimited tiers (admin) skip the ceiling check but still accrue usage, so
	// the quota display reflects what they uploaded today.
	if tier.DailyLimit != constants.UnlimitedDailyUpload {
		var user authModel.User
		if err := s.db.Select("daily_upload_size").First(&user, userID).Error; err != nil {
			return fmt.Errorf("获取用户信息失败")
		}
		if user.DailyUploadSize+size > tier.DailyLimit {
			_ = s.art.Delete(context.Background(), uuid)
			return fmt.Errorf("超过今日上传限额，已删除")
		}
	}
	if err := s.db.Model(&authModel.User{}).
		Where("id = ?", userID).
		UpdateColumn("daily_upload_size", gorm.Expr("daily_upload_size + ?", size)).Error; err != nil {
		return fmt.Errorf("扣减限额失败: %w", err)
	}
	deducted = true
	return nil
}

// Resume continues an interrupted upload: it asks the artifact service which
// parts are already stored (skip them) and returns fresh presigned URLs for only
// the missing parts, so a paused / dropped / page-refreshed upload finishes
// without re-sending bytes already in B2. No quota is touched here — the per-user
// daily budget is pre-checked at Init and deducted once at Complete; calling
// resume also refreshes the artifact's activity timestamp so the orphan GC won't
// reap it mid-resume.
func (s *Service) Resume(ctx context.Context, req ResumeRequest) (*ResumeResponse, error) {
	out, err := s.art.Resume(ctx, req.ArtifactUUID)
	if err != nil {
		return nil, mapArtifactErr(err)
	}

	resp := &ResumeResponse{
		ArtifactUUID: out.Uuid,
		Multipart:    out.Multipart,
		ExpiresAt:    out.ExpiresAt,
	}
	if out.Multipart {
		if out.PartSize != nil {
			resp.PartSize = *out.PartSize
		}
		if out.PartUrls != nil {
			for _, p := range *out.PartUrls {
				resp.Parts = append(resp.Parts, PartURL{PartNumber: int(p.PartNumber), URL: p.Url})
			}
		}
		if out.UploadedParts != nil {
			for _, p := range *out.UploadedParts {
				resp.UploadedParts = append(resp.UploadedParts, ResumePart{
					PartNumber: int(p.PartNumber),
					ETag:       p.Etag,
					Size:       p.Size,
				})
			}
		}
	} else if out.UploadUrl != nil {
		resp.UploadURL = *out.UploadUrl
	}
	return resp, nil
}

// Abort soft-deletes an in-progress upload on client request (the artifact GC
// also reclaims orphaned status=uploading rows after its TTL).
func (s *Service) Abort(ctx context.Context, req AbortRequest) error {
	return s.art.Delete(ctx, req.ArtifactUUID)
}

// mapArtifactErr translates artifact client sentinels into user-facing messages.
func mapArtifactErr(err error) error {
	switch {
	case errors.Is(err, artifactclient.ErrTooBig):
		return fmt.Errorf("文件大小超过上限")
	case errors.Is(err, artifactclient.ErrMIMEDenied):
		return fmt.Errorf("不支持的文件类型")
	case errors.Is(err, artifactclient.ErrSizeMismatch):
		return fmt.Errorf("上传文件大小与声明不符，请重新上传")
	case errors.Is(err, artifactclient.ErrQuotaExceeded):
		return fmt.Errorf("服务器今日制品配额已满，请稍后再试")
	case errors.Is(err, artifactclient.ErrUploadDisabled):
		return fmt.Errorf("上传功能暂未开放")
	case errors.Is(err, artifactclient.ErrUnauthorized):
		return fmt.Errorf("制品服务鉴权失败")
	case errors.Is(err, artifactclient.ErrNotConfigured):
		return fmt.Errorf("制品服务未配置")
	default:
		return err
	}
}
