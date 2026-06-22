// Package constants centralizes business constants.
package constants

// MaxLargeFileSize is the upper bound for a single uploaded patch file (1 GB).
// The artifact service enforces it (and the per-user daily quota) server-side
// too; this is just the instant local check before init.
const MaxLargeFileSize int64 = 1 * 1024 * 1024 * 1024

// Per-user daily upload quotas (reset at 00:00 by cron).
const (
	UserDailyUploadLimit    int64 = 100 * 1024 * 1024      // 100 MB
	CreatorDailyUploadLimit int64 = 5 * 1024 * 1024 * 1024 // 5 GB
)

// AllowedResourceExtensions aligns with the frontend resource form.
var AllowedResourceExtensions = []string{".zip", ".rar", ".7z"}
