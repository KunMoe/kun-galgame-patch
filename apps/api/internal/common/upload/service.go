package upload

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	authModel "kun-galgame-patch-api/internal/auth/model"
	"kun-galgame-patch-api/internal/constants"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/artifactclient"
	apperrors "kun-galgame-patch-api/pkg/errors"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Service struct {
	art *artifactclient.Client
	db  *gorm.DB
	rdb *redis.Client
}

func New(art *artifactclient.Client, db *gorm.DB, rdb *redis.Client) *Service {
	return &Service{art: art, db: db, rdb: rdb}
}

func ptr[T any](v T) *T { return &v }

const completeOnceTTL = 24 * time.Hour

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

func (s *Service) unmarkComplete(uuid string) {
	if s.rdb == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s.rdb.Del(ctx, "upload:complete:"+uuid)
}

// The artifact service checks the uploader only when the call carries a user
// token. moyu calls it with the site-wide key, so until this record existed any
// session could delete a published file by passing its artifact_uuid (which
// every resource list served) to /upload/abort.
const uploadOwnerTTL = 7 * 24 * time.Hour

var (
	errNotUploadOwner = errors.New("上传会话不存在或已过期，请重新上传")
	errArtifactInUse  = errors.New("该文件已用于资源，不能放弃上传")
)

func uploadOwnerKey(uuid string) string { return "upload:owner:" + uuid }

func (s *Service) requireOwner(ctx context.Context, uuid string, userID int) error {
	owner, err := s.rdb.Get(ctx, uploadOwnerKey(uuid)).Int()
	if errors.Is(err, redis.Nil) {
		return errNotUploadOwner
	}
	if err != nil {
		return fmt.Errorf("read upload owner: %w", err)
	}
	if owner != userID {
		return errNotUploadOwner
	}
	return nil
}

const oneGiB int64 = 1024 * 1024 * 1024

func (s *Service) validatePreUpload(userID int, fileName string, declaredSize int64, tier constants.UploadTier) error {
	if declaredSize <= 0 || declaredSize > tier.MaxFileSize {
		return apperrors.ErrBadRequest(fmt.Sprintf("文件大小超过 %d GB 上限", tier.MaxFileSize/oneGiB))
	}
	ext := strings.ToLower(filepath.Ext(fileName))
	if !slices.Contains(constants.AllowedResourceExtensions, ext) {
		return apperrors.ErrBadRequest("不支持的文件类型: " + ext)
	}
	if tier.DailyLimit == constants.UnlimitedDailyUpload {
		return nil
	}
	var user authModel.User
	if err := s.db.Select("daily_upload_size").First(&user, userID).Error; err != nil {
		return fmt.Errorf("read daily upload size: %w", err)
	}
	if user.DailyUploadSize+declaredSize > tier.DailyLimit {
		return apperrors.ErrBadRequest(fmt.Sprintf("超过今日上传限额 (%d GB)", tier.DailyLimit/oneGiB))
	}
	return nil
}

const dailyImageLimit = 50

var errDailyImageLimit = errors.New("今日图片上传次数已达上限")

// ReserveDailyImage spends one of the reader's daily image uploads before the
// upload runs: checking first and counting afterwards let concurrent uploads
// all pass the check.
func (s *Service) ReserveDailyImage(userID int) error {
	res := s.db.Model(&authModel.User{}).
		Where("id = ? AND daily_image_count < ?", userID, dailyImageLimit).
		UpdateColumn("daily_image_count", gorm.Expr("daily_image_count + 1"))
	if res.Error != nil {
		return fmt.Errorf("reserve daily image: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return errDailyImageLimit
	}
	return nil
}

func (s *Service) ReleaseDailyImage(userID int) {
	if err := s.db.Model(&authModel.User{}).
		Where("id = ? AND daily_image_count > 0", userID).
		UpdateColumn("daily_image_count", gorm.Expr("daily_image_count - 1")).Error; err != nil {
		slog.Warn("release daily image reservation failed", "user_id", userID, "error", err)
	}
}

func (s *Service) Init(ctx context.Context, userID int, tier constants.UploadTier, req InitRequest) (*InitResponse, error) {
	if err := s.validatePreUpload(userID, req.FileName, req.FileSize, tier); err != nil {
		return nil, err
	}

	in := artifactclient.InitUploadRequest{
		Name:        req.FileName,
		FileSize:    req.FileSize,
		Public:      ptr(true),
		UploaderSub: ptr(strconv.Itoa(userID)),
	}
	if req.MimeType != "" {
		in.MimeType = ptr(req.MimeType)
	}

	res, err := s.art.InitUpload(ctx, in)
	if err != nil {
		return nil, err
	}
	if err := s.rdb.Set(ctx, uploadOwnerKey(res.Uuid), userID, uploadOwnerTTL).Err(); err != nil {
		if derr := s.art.Delete(context.WithoutCancel(ctx), res.Uuid); derr != nil {
			slog.Warn("upload init: withdrawing the unowned upload failed", "artifact_uuid", res.Uuid, "error", derr)
		}
		return nil, fmt.Errorf("record upload owner: %w", err)
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

func (s *Service) Complete(ctx context.Context, userID int, tier constants.UploadTier, req CompleteRequest) (*CompleteResponse, error) {
	if err := s.requireOwner(ctx, req.ArtifactUUID, userID); err != nil {
		return nil, err
	}
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
		return nil, err
	}

	size := art.FileSize
	if err := s.deductQuotaOnce(ctx, userID, req.ArtifactUUID, size, tier); err != nil {
		return nil, err
	}
	return &CompleteResponse{ArtifactUUID: req.ArtifactUUID, Size: size}, nil
}

var errOverDailyUpload = apperrors.ErrBadRequest("超过今日上传限额，文件已删除")

// deductQuotaOnce runs after the artifact service has stored the file, so a
// local failure is logged and the upload still succeeds: an error here sent the
// reader back to upload a file that was already stored.
func (s *Service) deductQuotaOnce(ctx context.Context, userID int, uuid string, size int64, tier constants.UploadTier) error {
	first, err := s.markCompleteOnce(ctx, uuid)
	if err != nil {
		slog.Error("upload complete: quota not deducted", "artifact_uuid", uuid, "user_id", userID, "error", err)
		return nil
	}
	if !first {
		return nil
	}

	limited := tier.DailyLimit != constants.UnlimitedDailyUpload
	q := s.db.WithContext(ctx).Model(&authModel.User{}).Where("id = ?", userID)
	if limited {
		q = q.Where("daily_upload_size + ? <= ?", size, tier.DailyLimit)
	}
	res := q.UpdateColumn("daily_upload_size", gorm.Expr("daily_upload_size + ?", size))
	if res.Error != nil {
		s.unmarkComplete(uuid)
		slog.Error("upload complete: quota not deducted", "artifact_uuid", uuid, "user_id", userID, "error", res.Error)
		return nil
	}
	if limited && res.RowsAffected == 0 {
		s.unmarkComplete(uuid)
		if err := s.art.Delete(context.WithoutCancel(ctx), uuid); err != nil {
			slog.Warn("upload complete: deleting the over-quota upload failed", "artifact_uuid", uuid, "error", err)
		}
		return errOverDailyUpload
	}
	return nil
}

func (s *Service) Resume(ctx context.Context, userID int, req ResumeRequest) (*ResumeResponse, error) {
	if err := s.requireOwner(ctx, req.ArtifactUUID, userID); err != nil {
		return nil, err
	}
	out, err := s.art.Resume(ctx, req.ArtifactUUID)
	if err != nil {
		return nil, err
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

func (s *Service) Abort(ctx context.Context, userID int, req AbortRequest) error {
	if err := s.requireOwner(ctx, req.ArtifactUUID, userID); err != nil {
		return err
	}
	var inUse int64
	if err := s.db.Model(&patchModel.PatchResource{}).
		Where("artifact_uuid = ?", req.ArtifactUUID).
		Count(&inUse).Error; err != nil {
		return fmt.Errorf("check artifact usage: %w", err)
	}
	if inUse > 0 {
		return errArtifactInUse
	}
	if err := s.art.Delete(ctx, req.ArtifactUUID); err != nil && !errors.Is(err, artifactclient.ErrNotFound) {
		return err
	}
	s.rdb.Del(ctx, uploadOwnerKey(req.ArtifactUUID))
	return nil
}
