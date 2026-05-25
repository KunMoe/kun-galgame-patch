// Package storage wraps S3 / S3-compatible (Backblaze B2 / MinIO / R2 etc.) operations using minio-go v7.
//
// Design notes (D10, 2026-04-21):
//   - Client direct-upload model: the server only signs presigned URLs and never handles file bytes
//   - Small files: PresignedPutObject
//   - Large files: NewMultipartUpload -> presign PresignedUploadPart URL per part -> CompleteMultipartUpload
//   - After upload, use StatObject (= HeadObject) to verify size
//   - Orphan cleanup: ListIncompleteUploads + RemoveIncompleteUpload (run periodically by cron)
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"kun-galgame-patch-api/pkg/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Client wraps minio.Client and provides presigned URL / multipart / head / delete operations.
type S3Client struct {
	client    *minio.Client
	core      *minio.Core
	bucket    string
	publicURL string // public download prefix, e.g. "https://s3.us-east-005.backblazeb2.com/kun-galgame-patch"
}

// NewS3 initializes a client from config. When cfg.Endpoint is empty it returns a disabled placeholder (so dev can start without S3).
func NewS3(cfg config.S3Config) *S3Client {
	if cfg.Endpoint == "" {
		slog.Warn("S3 未配置，storage 模块处于禁用状态")
		return &S3Client{}
	}

	host, secure, err := parseEndpoint(cfg.Endpoint)
	if err != nil {
		panic("解析 S3 endpoint 失败: " + err.Error())
	}

	mc, err := minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: secure,
		Region: cfg.Region,
	})
	if err != nil {
		panic("minio client 初始化失败: " + err.Error())
	}

	// PublicURL fronts the bucket for downloads (CDN / reverse proxy with our
	// own domain, e.g. https://oss.moyu.moe). Falling back to Endpoint+Bucket
	// works for dev (direct B2 download) but in prod it would leak the raw B2
	// host and bypass the CDN — see S3Config doc comment.
	publicURL := strings.TrimRight(cfg.PublicURL, "/")
	if publicURL == "" {
		publicURL = strings.TrimRight(cfg.Endpoint, "/") + "/" + cfg.Bucket
	}

	slog.Info("S3 客户端就绪", "endpoint", host, "bucket", cfg.Bucket, "public_url", publicURL, "tls", secure)

	return &S3Client{
		client:    mc,
		core:      &minio.Core{Client: mc},
		bucket:    cfg.Bucket,
		publicURL: publicURL,
	}
}

// parseEndpoint splits a full URL ("https://s3.xxx.com") into host and TLS flag;
// minio-go requires the host part to have no scheme.
func parseEndpoint(raw string) (host string, secure bool, err error) {
	if !strings.Contains(raw, "://") {
		return raw, true, nil
	}
	u, perr := url.Parse(raw)
	if perr != nil {
		return "", false, perr
	}
	return u.Host, u.Scheme == "https", nil
}

// Ready reports whether the client has been configured. When not configured, all operations return ErrNotConfigured.
func (c *S3Client) Ready() bool { return c.client != nil }

// ErrNotConfigured is returned when S3 is not configured.
var ErrNotConfigured = errors.New("S3 client 未配置")

func (c *S3Client) check() error {
	if !c.Ready() {
		return ErrNotConfigured
	}
	return nil
}

// Bucket returns the bucket name for upper layers (handlers) to build keys.
func (c *S3Client) Bucket() string { return c.bucket }

// PublicURL returns the public download URL for the given s3_key.
func (c *S3Client) PublicURL(s3Key string) string {
	return c.publicURL + "/" + s3Key
}

// ─────────────────────────────────────────────────────────────
// Small files: PresignedPutObject
// ─────────────────────────────────────────────────────────────

// PresignPutObject generates an upload URL with TTL ttl for the given s3_key.
// The client can PUT directly to the returned URL to complete the upload.
func (c *S3Client) PresignPutObject(ctx context.Context, s3Key string, ttl time.Duration) (string, error) {
	if err := c.check(); err != nil {
		return "", err
	}
	u, err := c.client.PresignedPutObject(ctx, c.bucket, s3Key, ttl)
	if err != nil {
		return "", fmt.Errorf("签 PutObject URL 失败: %w", err)
	}
	return u.String(), nil
}

// ─────────────────────────────────────────────────────────────
// Large-file multipart: init / sign-parts / complete / abort
// ─────────────────────────────────────────────────────────────

// InitMultipart starts a new multipart upload and returns the uploadID.
func (c *S3Client) InitMultipart(ctx context.Context, s3Key string) (string, error) {
	if err := c.check(); err != nil {
		return "", err
	}
	uploadID, err := c.core.NewMultipartUpload(ctx, c.bucket, s3Key, minio.PutObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("发起 multipart 失败: %w", err)
	}
	return uploadID, nil
}

// PresignUploadPart generates a presigned URL for one part of a multipart upload (partNumber starts at 1).
func (c *S3Client) PresignUploadPart(ctx context.Context, s3Key, uploadID string, partNumber int, ttl time.Duration) (string, error) {
	if err := c.check(); err != nil {
		return "", err
	}
	q := make(url.Values)
	q.Set("uploadId", uploadID)
	q.Set("partNumber", fmt.Sprintf("%d", partNumber))

	u, err := c.client.Presign(ctx, "PUT", c.bucket, s3Key, ttl, q)
	if err != nil {
		return "", fmt.Errorf("签 UploadPart URL 失败: %w", err)
	}
	return u.String(), nil
}

// CompletedPart is a single part inside a multipart completion request.
type CompletedPart struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
}

// CompleteMultipart merges all parts into the final object. parts does not need to be pre-sorted.
func (c *S3Client) CompleteMultipart(ctx context.Context, s3Key, uploadID string, parts []CompletedPart) error {
	if err := c.check(); err != nil {
		return err
	}
	mParts := make([]minio.CompletePart, 0, len(parts))
	for _, p := range parts {
		mParts = append(mParts, minio.CompletePart{
			PartNumber: p.PartNumber,
			ETag:       p.ETag,
		})
	}
	_, err := c.core.CompleteMultipartUpload(ctx, c.bucket, s3Key, uploadID, mParts, minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("完成 multipart 失败: %w", err)
	}
	return nil
}

// AbortMultipart aborts an in-progress multipart upload.
func (c *S3Client) AbortMultipart(ctx context.Context, s3Key, uploadID string) error {
	if err := c.check(); err != nil {
		return err
	}
	if err := c.core.AbortMultipartUpload(ctx, c.bucket, s3Key, uploadID); err != nil {
		return fmt.Errorf("abort multipart 失败: %w", err)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────
// Object metadata + delete
// ─────────────────────────────────────────────────────────────

// PutObject performs server-side upload (used e.g. for banners where image
// processing must happen first). Intended for small objects (consumes server
// egress bandwidth); for large files use the presigned URL path.
func (c *S3Client) PutObject(ctx context.Context, s3Key string, reader io.Reader, size int64, contentType string) error {
	if err := c.check(); err != nil {
		return err
	}
	_, err := c.client.PutObject(ctx, c.bucket, s3Key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("PutObject 失败: %w", err)
	}
	return nil
}

// StatObject maps to S3 HeadObject, returning size/etag/contentType of the object.
func (c *S3Client) StatObject(ctx context.Context, s3Key string) (minio.ObjectInfo, error) {
	if err := c.check(); err != nil {
		return minio.ObjectInfo{}, err
	}
	return c.client.StatObject(ctx, c.bucket, s3Key, minio.StatObjectOptions{})
}

// IsNotFound reports whether an error from StatObject/DeleteObject is "object not found".
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	return minio.ToErrorResponse(err).Code == "NoSuchKey"
}

// DeleteObject deletes an object. Returns nil even if the object does not exist (idempotent).
func (c *S3Client) DeleteObject(ctx context.Context, s3Key string) error {
	if err := c.check(); err != nil {
		return err
	}
	err := c.client.RemoveObject(ctx, c.bucket, s3Key, minio.RemoveObjectOptions{})
	if err != nil && !IsNotFound(err) {
		return fmt.Errorf("删除对象 %s 失败: %w", s3Key, err)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────
// Orphan multipart cleanup (used by cron)
// ─────────────────────────────────────────────────────────────

// IncompleteUpload represents an unfinished multipart upload for cron-based cleanup.
type IncompleteUpload struct {
	Key       string
	UploadID  string
	Initiated time.Time
}

// ListIncompleteUploads lists all unfinished multipart uploads. prefix narrows the scope; if empty, the whole bucket is listed.
func (c *S3Client) ListIncompleteUploads(ctx context.Context, prefix string) ([]IncompleteUpload, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	var out []IncompleteUpload
	for info := range c.client.ListIncompleteUploads(ctx, c.bucket, prefix, true) {
		if info.Err != nil {
			return nil, info.Err
		}
		out = append(out, IncompleteUpload{
			Key:       info.Key,
			UploadID:  info.UploadID,
			Initiated: info.Initiated,
		})
	}
	return out, nil
}

// RemoveIncompleteUpload aborts an in-progress multipart upload (cleanup).
func (c *S3Client) RemoveIncompleteUpload(ctx context.Context, s3Key string) error {
	if err := c.check(); err != nil {
		return err
	}
	return c.client.RemoveIncompleteUpload(ctx, c.bucket, s3Key)
}
