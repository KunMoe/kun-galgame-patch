package upload

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"kun-galgame-patch-api/internal/constants"
	"kun-galgame-patch-api/internal/middleware"
	settingService "kun-galgame-patch-api/internal/setting/service"

	"github.com/gofiber/fiber/v3"
)

const tiersSettingKey = "upload_tiers"

// MaxFileCapGiB is the artifact service's own per-file limit for moyu's client
// (oauth_clients.artifact_max_file_size, 20 GiB in production); a site limit
// above it would only move the refusal from moyu to artifact.
const MaxFileCapGiB = 20

const maxDailyGiB = 100000

// TierSetting is one role's limits in whole GiB; DailyGiB 0 means no daily
// limit.
type TierSetting struct {
	MaxFileGiB int `json:"max_file_gib"`
	DailyGiB   int `json:"daily_gib"`
}

type Tiers struct {
	User      TierSetting `json:"user"`
	Creator   TierSetting `json:"creator"`
	Moderator TierSetting `json:"moderator"`
	Admin     TierSetting `json:"admin"`
}

var DefaultTiers = Tiers{
	User:      TierSetting{MaxFileGiB: 10, DailyGiB: 100},
	Creator:   TierSetting{MaxFileGiB: 10, DailyGiB: 300},
	Moderator: TierSetting{MaxFileGiB: 10, DailyGiB: 5000},
	Admin:     TierSetting{MaxFileGiB: 20, DailyGiB: 0},
}

const oneGiBBytes int64 = 1024 * 1024 * 1024

func (t TierSetting) tier() constants.UploadTier {
	daily := constants.UnlimitedDailyUpload
	if t.DailyGiB > 0 {
		daily = int64(t.DailyGiB) * oneGiBBytes
	}
	return constants.UploadTier{MaxFileSize: int64(t.MaxFileGiB) * oneGiBBytes, DailyLimit: daily}
}

func (t TierSetting) validate(role string) error {
	if t.MaxFileGiB < 1 || t.MaxFileGiB > MaxFileCapGiB {
		return fmt.Errorf("%s 的单文件上限必须在 1 到 %d GB 之间", role, MaxFileCapGiB)
	}
	if t.DailyGiB != 0 && (t.DailyGiB < t.MaxFileGiB || t.DailyGiB > maxDailyGiB) {
		return fmt.Errorf("%s 的每日上限必须为 0 (不限), 或在单文件上限到 %d GB 之间", role, maxDailyGiB)
	}
	return nil
}

func (t Tiers) Validate() error {
	for _, r := range []struct {
		name string
		t    TierSetting
	}{{"普通用户", t.User}, {"创作者", t.Creator}, {"版主", t.Moderator}, {"管理员", t.Admin}} {
		if err := r.t.validate(r.name); err != nil {
			return err
		}
	}
	return nil
}

type stringSettings interface {
	GetString(key string) string
	SetString(key, value string, updatedBy int) error
}

// TierStore reads the tiers from site settings on every upload call; the
// setting service caches the row, so a change reaches other instances within
// its TTL.
type TierStore struct {
	settings stringSettings
}

func NewTierStore(settings *settingService.Service) *TierStore {
	return &TierStore{settings: settings}
}

// Load falls back to DefaultTiers when nothing is stored, and per role when a
// stored role fails validation, so a bad row never blocks uploads.
func (s *TierStore) Load() Tiers {
	if s == nil || s.settings == nil {
		return DefaultTiers
	}
	raw := s.settings.GetString(tiersSettingKey)
	if raw == "" {
		return DefaultTiers
	}
	t := DefaultTiers
	if err := json.Unmarshal([]byte(raw), &t); err != nil {
		slog.Error("upload tiers setting is not valid JSON; using the defaults", "error", err)
		return DefaultTiers
	}
	keep := func(got *TierSetting, def TierSetting, role string) {
		if err := got.validate(role); err != nil {
			slog.Error("upload tier setting rejected; using the default for this role", "role", role, "error", err)
			*got = def
		}
	}
	keep(&t.User, DefaultTiers.User, "user")
	keep(&t.Creator, DefaultTiers.Creator, "creator")
	keep(&t.Moderator, DefaultTiers.Moderator, "moderator")
	keep(&t.Admin, DefaultTiers.Admin, "admin")
	return t
}

func (s *TierStore) Save(t Tiers, updatedBy int) error {
	if err := t.Validate(); err != nil {
		return err
	}
	raw, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return s.settings.SetString(tiersSettingKey, string(raw), updatedBy)
}

func (s *TierStore) For(c fiber.Ctx) constants.UploadTier {
	t := s.Load()
	switch {
	// ast-grep-ignore: staff-power-through-permission -- an upload quota tier follows the role, it grants no power
	case middleware.IsAdmin(c):
		return t.Admin.tier()
	case middleware.HasRole(c, "moderator"):
		return t.Moderator.tier()
	case middleware.HasRole(c, "creator"):
		return t.Creator.tier()
	default:
		return t.User.tier()
	}
}
