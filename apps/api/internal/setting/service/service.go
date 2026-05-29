// Package service is the source of truth for site-wide admin settings, backed
// by the site_setting table (see migration 012). It replaces the previous
// Redis-backed GetSetting/SetSetting: durable, audited, per-site.
//
// Reads hit the table directly (PK lookup) — the only callers are on write
// paths (publish / comment create), which are low-frequency, so no cache layer
// is warranted. If a setting ever becomes read-hot, add an in-process cache
// invalidated on write (or Postgres LISTEN/NOTIFY) without changing this API.
package service

import (
	"kun-galgame-patch-api/internal/setting/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Setting keys. No "admin:" Redis-namespace prefix — the table is per-site, so
// there's nothing to disambiguate. Single source of truth for the key strings,
// shared by the admin (write/read) and patch (enforcement read) packages.
const (
	KeyCommentVerify = "comment_verify"
	KeyCreatorOnly   = "creator_only"
)

type Service struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Service { return &Service{db: db} }

// GetBool reports whether the setting is enabled. A missing key → false
// (settings default off). A read error also yields false: failing safe means
// an outage can't silently flip a gate like comment-verify / creator-only on.
func (s *Service) GetBool(key string) bool {
	var row model.SiteSetting
	if err := s.db.Select("value").Where("key = ?", key).First(&row).Error; err != nil {
		return false
	}
	return row.Value == "true"
}

// SetBool upserts the setting, recording the admin who changed it + the time.
func (s *Service) SetBool(key string, enabled bool, updatedBy int) error {
	val := "false"
	if enabled {
		val = "true"
	}
	row := model.SiteSetting{Key: key, Value: val, UpdatedBy: &updatedBy}
	// INSERT … ON CONFLICT (key) DO UPDATE — one round-trip, no read-modify-write.
	return s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "key"}},
		DoUpdates: clause.Assignments(map[string]any{
			"value":      val,
			"updated_by": updatedBy,
			"updated_at": gorm.Expr("now()"),
		}),
	}).Create(&row).Error
}
