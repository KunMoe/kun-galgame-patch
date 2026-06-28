// Package constants centralizes business constants.
package constants

const oneGiB int64 = 1024 * 1024 * 1024

// UnlimitedDailyUpload marks a tier with no per-day ceiling (admin).
const UnlimitedDailyUpload int64 = -1

// UploadTier is a role's patch-file upload allowance. MaxFileSize caps a single
// file; DailyLimit caps the per-user per-day total (reset to 0 by the daily
// cron). DailyLimit == UnlimitedDailyUpload means no daily ceiling.
//
// NOTE: the artifact service additionally enforces a per-SITE cap on moyu's
// OAuth client (artifact_max_file_size / artifact_quota_bytes_daily). For a tier
// to actually apply, that site cap must be >= the largest MaxFileSize here
// (20 GB) and the site daily-byte quota generous enough for all users combined —
// otherwise infra rejects (50004 / 50012) before this per-user check matters.
// The moderator daily 5000 GB exceeds moyu's site daily quota (~2 TB), so for
// that tier the site-wide cap is the effective per-day ceiling.
type UploadTier struct {
	MaxFileSize int64 // single-file cap (bytes)
	DailyLimit  int64 // per-user per-day cap (bytes); UnlimitedDailyUpload = none
}

// Per-role upload tiers (resolved by the handler from the OAuth roles claim).
var (
	UserUploadTier      = UploadTier{MaxFileSize: 1 * oneGiB, DailyLimit: 1 * oneGiB}
	CreatorUploadTier   = UploadTier{MaxFileSize: 5 * oneGiB, DailyLimit: 100 * oneGiB}
	ModeratorUploadTier = UploadTier{MaxFileSize: 10 * oneGiB, DailyLimit: 5000 * oneGiB}
	AdminUploadTier     = UploadTier{MaxFileSize: 20 * oneGiB, DailyLimit: UnlimitedDailyUpload}
)

// AllowedResourceExtensions aligns with the frontend resource form.
var AllowedResourceExtensions = []string{".zip", ".rar", ".7z"}
