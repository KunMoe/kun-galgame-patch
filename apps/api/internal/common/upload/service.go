// Package upload encapsulates the patch resource upload flow (D10, 2026-04-21).
//
// Two independent paths:
//   - Small files (<= 200 MB): PresignPutObject in one shot
//   - Large files (> 200 MB, <= 1 GB): multipart init / part presign / complete / abort
//
// After a successful upload, the frontend receives the s3_key and calls
// POST /api/patch/:id/resource to persist the record. The daily quota
// (daily_upload_size) is decremented at the complete stage based on the actual
// size returned by HeadObject.
package upload

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"kun-galgame-patch-api/internal/constants"
	"kun-galgame-patch-api/internal/infrastructure/storage"
	authModel "kun-galgame-patch-api/internal/auth/model"

	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Service combines the S3 client and DB + Redis. Redis is used purely as an
// idempotency guard for the Complete endpoints (MOYU-PR7 / M5): without it,
// calling /upload/small/complete twice on the same s3_key double-deducts
// daily_upload_size. We can't dedupe via patch_resource (which is created in
// a separate later call) and don't want to add an upload_session table just
// for this; a 24h SETNX is the cheapest correct fix.
type Service struct {
	s3  *storage.S3Client
	db  *gorm.DB
	rdb *redis.Client
}

// New constructs a Service. rdb may be nil in tests; the complete-idempotency
// guard then degrades to "best effort, no DB-level dedupe" — acceptable for
// pure unit tests that don't care.
func New(s3 *storage.S3Client, db *gorm.DB, rdb *redis.Client) *Service {
	return &Service{s3: s3, db: db, rdb: rdb}
}

// completeOnceTTL is how long we remember "this s3_key already had its
// quota deducted". 24h covers the daily-quota reset window.
const completeOnceTTL = 24 * time.Hour

// markCompleteOnce returns true if THIS call is the first complete for the
// given s3_key, false if a prior call already deducted quota. Nil rdb (tests)
// always returns true (degraded "best effort"). Uses SetArgs with NX mode
// (the modern Redis API; SetNX is deprecated since 2.6.12) — matches the
// pattern already used in middleware/auth.go refreshOAuthToken.
func (s *Service) markCompleteOnce(ctx context.Context, s3Key string) (bool, error) {
	if s.rdb == nil {
		return true, nil
	}
	key := "upload:complete:" + s3Key
	res, err := s.rdb.SetArgs(ctx, key, "1", redis.SetArgs{
		TTL:  completeOnceTTL,
		Mode: "NX",
	}).Result()
	if err != nil {
		// Redis returns redis.Nil when the NX condition fails (key already
		// exists). That's the "not first" signal, not an error.
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}
	return res == "OK", nil
}

// unmarkComplete releases the idempotency marker for s3Key, used when a
// complete attempt set the marker but then failed before the quota deduction
// committed (so a retry can re-run the deduction). Best-effort, and uses a
// fresh context because the request context may already be done by the time
// this runs in a deferred cleanup.
func (s *Service) unmarkComplete(s3Key string) {
	if s.rdb == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s.rdb.Del(ctx, "upload:complete:"+s3Key)
}

// ─── s3_key generation ───────────────────────────────

var (
	s3KeyAlphabet   = []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")
	fileNameInvalid = regexp.MustCompile(`[^\p{L}\p{N}_\-]`) // matches apps/next-web/utils/sanitizeFileName.ts
)

// sanitizeFileName mirrors the original TS sanitizeFileName: keep letters,
// digits, underscore and hyphen; strip all other characters; preserve the
// extension; truncate the basename to 100 characters.
func sanitizeFileName(name string) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	base = fileNameInvalid.ReplaceAllString(base, "")
	if len([]rune(base)) > 100 {
		base = string([]rune(base)[:100])
	}
	return base + ext
}

// randomSegment returns a length-char [A-Za-z0-9] random string, replacing the legacy BLAKE3 hash segment.
func randomSegment(length int) (string, error) {
	b := make([]byte, length)
	max := big.NewInt(int64(len(s3KeyAlphabet)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = s3KeyAlphabet[n.Int64()]
	}
	return string(b), nil
}

// buildPatchResourceKey builds the full S3 object key "patch/{patchId}/{random64}/{fileName}".
func buildPatchResourceKey(patchID int, fileName string) (string, error) {
	seg, err := randomSegment(constants.S3KeyRandomLength)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("patch/%d/%s/%s", patchID, seg, sanitizeFileName(fileName)), nil
}

// ─── Validation (business rules beyond auth) ─────────

// validatePreUpload pre-checks at init time: extension, size cap, daily quota (based on declared size).
//
// privileged grants the higher daily quota (admins / moderators); resolved by
// the handler from OAuth roles claim. The local user table no longer stores
// role since the OAuth migration.
func (s *Service) validatePreUpload(userID int, fileName string, declaredSize int64, privileged bool) error {
	if declaredSize <= 0 || declaredSize > constants.MaxLargeFileSize {
		return fmt.Errorf("文件大小超过 1GB 上限")
	}

	ext := strings.ToLower(filepath.Ext(fileName))
	if !slices.Contains(constants.AllowedResourceExtensions, ext) {
		return fmt.Errorf("不支持的文件类型: %s", ext)
	}

	var user authModel.User
	if err := s.db.Select("daily_upload_size").First(&user, userID).Error; err != nil {
		return fmt.Errorf("获取用户信息失败")
	}

	limit := s.dailyLimit(privileged)
	if int64(user.DailyUploadSize)+declaredSize > limit {
		return fmt.Errorf("超过今日上传限额 (%d MB)", limit/1024/1024)
	}
	return nil
}

func (s *Service) dailyLimit(privileged bool) int64 {
	if privileged {
		return constants.CreatorDailyUploadLimit
	}
	return constants.UserDailyUploadLimit
}

// verifyAndFinalize is shared by small and multipart completion:
//  1. HeadObject confirms the object really exists
//  2. Compare actual size with declared size (mismatch -> delete + error)
//  3. Idempotency guard: only deduct quota once per s3_key (Redis SETNX,
//     24h TTL — MOYU-PR7 / M5; prevents double-charge on retry/replay)
//  4. Increment daily_upload_size (atomic UPDATE)
func (s *Service) verifyAndFinalize(ctx context.Context, userID int, s3Key string, declared int64, privileged bool) (int64, error) {
	info, err := s.s3.StatObject(ctx, s3Key)
	if err != nil {
		return 0, fmt.Errorf("HeadObject 失败: %w", err)
	}
	actual := info.Size

	if actual != declared {
		_ = s.s3.DeleteObject(ctx, s3Key)
		return 0, fmt.Errorf("文件大小不一致（声明 %d，实际 %d），已删除", declared, actual)
	}
	if actual > constants.MaxLargeFileSize {
		_ = s.s3.DeleteObject(ctx, s3Key)
		return 0, fmt.Errorf("文件大小超过 1GB 上限，已删除")
	}

	// SETNX-style idempotency: if this s3_key was already complete'd within
	// the TTL window, return the size without re-deducting quota. The S3
	// object stays put (caller's retry sees the same successful "size"
	// response).
	first, err := s.markCompleteOnce(ctx, s3Key)
	if err != nil {
		return 0, fmt.Errorf("complete 幂等校验失败: %w", err)
	}
	if !first {
		return actual, nil
	}
	// The idempotency marker is now held. It must persist ONLY if the quota
	// deduction below actually commits — otherwise a retry hits the "!first"
	// fast-path above and returns success WITHOUT ever deducting, permanently
	// under-counting daily_upload_size. Release the marker on every failure
	// path so a retry can re-run the deduction (MOYU-PR7 / M5 follow-up).
	deducted := false
	defer func() {
		if !deducted {
			s.unmarkComplete(s3Key)
		}
	}()

	var user authModel.User
	if err := s.db.Select("daily_upload_size").First(&user, userID).Error; err != nil {
		return 0, fmt.Errorf("获取用户信息失败")
	}
	if int64(user.DailyUploadSize)+actual > s.dailyLimit(privileged) {
		_ = s.s3.DeleteObject(ctx, s3Key)
		return 0, fmt.Errorf("超过今日上传限额，已删除")
	}

	if err := s.db.Model(&authModel.User{}).
		Where("id = ?", userID).
		UpdateColumn("daily_upload_size", gorm.Expr("daily_upload_size + ?", actual)).Error; err != nil {
		return 0, fmt.Errorf("扣减限额失败: %w", err)
	}
	deducted = true
	return actual, nil
}

// ─── Public actions ──────────────────────────────────

// InitSmall initializes a small-file upload: generate s3_key and a presigned PUT URL.
func (s *Service) InitSmall(ctx context.Context, userID int, privileged bool, req SmallInitRequest) (*SmallInitResponse, error) {
	if req.FileSize > constants.MaxSmallFileSize {
		return nil, fmt.Errorf("小文件上限 200MB，请走 multipart")
	}
	if err := s.validatePreUpload(userID, req.FileName, req.FileSize, privileged); err != nil {
		return nil, err
	}

	key, err := buildPatchResourceKey(req.GalgameID, req.FileName)
	if err != nil {
		return nil, err
	}
	u, err := s.s3.PresignPutObject(ctx, key, constants.PresignPutObjectTTL)
	if err != nil {
		return nil, err
	}
	return &SmallInitResponse{S3Key: key, UploadURL: u}, nil
}

// CompleteSmall completes a small-file upload: HeadObject + quota deduction.
func (s *Service) CompleteSmall(ctx context.Context, userID int, privileged bool, req SmallCompleteRequest) (*CompleteResponse, error) {
	size, err := s.verifyAndFinalize(ctx, userID, req.S3Key, req.DeclaredSize, privileged)
	if err != nil {
		return nil, err
	}
	return &CompleteResponse{S3Key: req.S3Key, Size: size}, nil
}

// InitMultipart initializes a large-file upload: CreateMultipartUpload + presign a URL for every part.
func (s *Service) InitMultipart(ctx context.Context, userID int, privileged bool, req MultipartInitRequest) (*MultipartInitResponse, error) {
	if req.FileSize <= constants.MaxSmallFileSize {
		return nil, fmt.Errorf("≤ 200MB 请走 /upload/small")
	}
	if err := s.validatePreUpload(userID, req.FileName, req.FileSize, privileged); err != nil {
		return nil, err
	}
	// part_count must match the fixed 10 MiB chunking the client uses (FE
	// computes ceil(file_size / MULTIPART_PART_SIZE) with the same constant).
	// Reject mismatches so a client can't decouple part_count from the real
	// size and force the server to presign thousands of bogus part URLs.
	wantParts := int((req.FileSize + constants.MultipartPartSize - 1) / constants.MultipartPartSize)
	if req.PartCount != wantParts {
		return nil, fmt.Errorf("分片数不正确：应为 %d", wantParts)
	}

	key, err := buildPatchResourceKey(req.GalgameID, req.FileName)
	if err != nil {
		return nil, err
	}

	uploadID, err := s.s3.InitMultipart(ctx, key)
	if err != nil {
		return nil, err
	}

	urls := make([]string, 0, req.PartCount)
	for i := 1; i <= req.PartCount; i++ {
		u, err := s.s3.PresignUploadPart(ctx, key, uploadID, i, constants.PresignUploadPartTTL)
		if err != nil {
			// Failed mid-signing -> abort the upload so the client can retry
			_ = s.s3.AbortMultipart(ctx, key, uploadID)
			return nil, fmt.Errorf("签 part %d 失败: %w", i, err)
		}
		urls = append(urls, u)
	}

	return &MultipartInitResponse{S3Key: key, UploadID: uploadID, PartURLs: urls}, nil
}

// CompleteMultipart completes a large-file upload: CompleteMultipartUpload + HeadObject + quota deduction.
func (s *Service) CompleteMultipart(ctx context.Context, userID int, privileged bool, req MultipartCompleteRequest) (*CompleteResponse, error) {
	parts := make([]storage.CompletedPart, 0, len(req.Parts))
	for _, p := range req.Parts {
		parts = append(parts, storage.CompletedPart{PartNumber: p.PartNumber, ETag: p.ETag})
	}

	if err := s.s3.CompleteMultipart(ctx, req.S3Key, req.UploadID, parts); err != nil {
		return nil, err
	}

	size, err := s.verifyAndFinalize(ctx, userID, req.S3Key, req.DeclaredSize, privileged)
	if err != nil {
		return nil, err
	}
	return &CompleteResponse{S3Key: req.S3Key, Size: size}, nil
}

// AbortMultipart cancels a multipart upload on client request.
func (s *Service) AbortMultipart(ctx context.Context, req MultipartAbortRequest) error {
	return s.s3.AbortMultipart(ctx, req.S3Key, req.UploadID)
}

// ─── minio error code helpers ────────────────────────

// IsMinioNotFound is used by other layers to detect "object not found".
func IsMinioNotFound(err error) bool {
	if err == nil {
		return false
	}
	return minio.ToErrorResponse(err).Code == "NoSuchKey"
}
