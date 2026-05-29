package model

import "time"

// SiteSetting is one durable, audited site-wide configuration entry — the
// source of truth for admin toggles (comment-verify, creator-only, …).
//
// Replaces the old Redis key/value approach: living in the per-site Postgres
// DB makes it persistent, captured in backups, auditable (UpdatedBy/At), and
// free of the cross-site Redis key-collision risk. See migration 012.
//
// Value is stored as text and interpreted by the typed accessors on
// setting.Service (currently bool "true"/"false"); widen to JSON if a non-bool
// setting is ever added.
type SiteSetting struct {
	Key       string    `gorm:"primaryKey;type:varchar(100)" json:"key"`
	Value     string    `gorm:"type:text;not null;default:''" json:"value"`
	// UpdatedBy is the admin user id who last changed the value (audit). No FK
	// to "user" — settings must outlive any user (incl. the purge flow).
	UpdatedBy *int      `json:"updated_by"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (SiteSetting) TableName() string { return "site_setting" }
