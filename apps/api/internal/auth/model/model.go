package model

import "time"

// User is the local user table.
//
// Status (Phase 1-4 of OAuth migration): the OAuth-managed columns are still
// present so existing Preload("User") read paths in patch/admin/common/chat
// continue to work. Phase 5-6 slims this struct down to site-local fields
// only and migration 005 drops the OAuth-managed columns from the schema.
//
// IMPORTANT: ID is no longer autoIncrement. New users are inserted via the
// OAuth callback with the integer id returned by /oauth/userinfo (which is
// aligned with kungal/moyu by the migrate-users script).
type User struct {
	ID              int       `gorm:"primaryKey" json:"id"`
	Name            string    `gorm:"uniqueIndex;type:varchar(17);not null" json:"name"`
	Email           string    `gorm:"uniqueIndex;type:varchar(1007);not null" json:"email"`
	Password        string    `gorm:"type:varchar(1007);not null" json:"-"`
	IP              string    `gorm:"type:varchar(233);default:''" json:"-"`
	Avatar          string    `gorm:"type:varchar(233);default:''" json:"avatar"`
	Role            int       `gorm:"default:1" json:"role"`
	Status          int       `gorm:"default:0" json:"status"`
	RegisterTime    time.Time `gorm:"autoCreateTime" json:"register_time"`
	Moemoepoint     int       `gorm:"default:0" json:"moemoepoint"`
	Bio             string    `gorm:"type:varchar(107);default:''" json:"bio"`
	DailyImageCount int       `gorm:"default:0" json:"-"`
	DailyCheckIn    int       `gorm:"default:0" json:"-"`
	DailyUploadSize int       `gorm:"default:0" json:"-"`
	LastLoginTime   string    `gorm:"default:''" json:"-"`
	FollowerCount   int       `gorm:"default:0" json:"follower_count"`
	FollowingCount  int       `gorm:"default:0" json:"following_count"`
	Created         time.Time `gorm:"autoCreateTime" json:"created"`
	Updated         time.Time `gorm:"autoUpdateTime" json:"updated"`
}

func (User) TableName() string { return "user" }
