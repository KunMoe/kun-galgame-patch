package constants

const oneGiB int64 = 1024 * 1024 * 1024

const UnlimitedDailyUpload int64 = -1

type UploadTier struct {
	MaxFileSize int64
	DailyLimit  int64
}

// The per-role tiers are a site setting (upload.DefaultTiers, /admin/setting).
// The forge repacks every eligible patch for letmoe's merged downloads (owner's
// ruling, 2026-10-14). The STORE zips run ~260 GB for the first 789 patches, so
// the creator tier's 100 GiB a day queued them for days.
var BotUploadTier = UploadTier{MaxFileSize: 20 * oneGiB, DailyLimit: 1024 * oneGiB}

var AllowedResourceExtensions = []string{".zip", ".rar", ".7z"}
