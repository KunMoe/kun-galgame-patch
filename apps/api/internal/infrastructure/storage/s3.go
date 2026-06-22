// Package storage wraps S3 / S3-compatible (Backblaze B2 / MinIO / R2 etc.) operations using minio-go v7.
//
// Scope (post-artifact migration, 2026-06): large-file patch upload/download moved
// to the centralized artifact service (kun-galgame-infra). This client now serves
// only two remaining moyu-owned needs against its own B2 bucket:
//   - PutObject + PublicURL: user personal-page image uploads (internal/user)
//   - DeleteObject: reclaiming legacy s3_key blobs when a pre-artifact resource is
//     updated / deleted / purged (dual-read; see internal/patch + internal/admin)
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strings"

	"kun-galgame-patch-api/pkg/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Client wraps minio.Client for the remaining server-side object operations
// (user-image PutObject + legacy-blob DeleteObject).
type S3Client struct {
	client    *minio.Client
	bucket    string
	publicURL string // public download prefix, e.g. "https://oss.moyu.moe"
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

// PublicURL returns the public download URL for the given s3_key.
func (c *S3Client) PublicURL(s3Key string) string {
	return c.publicURL + "/" + s3Key
}

// PutObject performs a server-side upload (used for user-page images, where the
// image is processed/resized first). Intended for small objects — it consumes
// server egress bandwidth.
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

// IsNotFound reports whether an error from DeleteObject is "object not found".
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
