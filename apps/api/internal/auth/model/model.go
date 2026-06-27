package model

import "time"

// User is the slimmed local user table.
//
// After the OAuth migration, identity (name / email / password / avatar /
// bio / status / role) lives on the OAuth server. The local row holds only
// site-local state: counters, daily quotas, follow counts, and the IP /
// last-login fingerprint. The id is aligned with OAuth.users.id by the
// migrate-users script, so look-ups go directly by integer id (no
// oauth_account indirection).
//
// IMPORTANT: ID is no longer autoIncrement. New rows are inserted by the
// OAuth callback with the integer id returned by /oauth/userinfo (see
// migration 005 which drops the IDENTITY/SERIAL on user.id).
type User struct {
	ID              int       `gorm:"primaryKey" json:"id"`
	IP              string    `gorm:"type:varchar(233);default:''" json:"-"`
	Moemoepoint     int       `gorm:"default:0" json:"moemoepoint"`
	DailyImageCount int       `gorm:"default:0" json:"-"`
	DailyCheckIn    int       `gorm:"default:0" json:"-"`
	DailyUploadSize int64     `gorm:"default:0" json:"-"`
	LastLoginTime   string    `gorm:"default:''" json:"-"`
	FollowerCount   int       `gorm:"default:0" json:"follower_count"`
	FollowingCount  int       `gorm:"default:0" json:"following_count"`
	Created         time.Time `gorm:"autoCreateTime" json:"created"`
	Updated         time.Time `gorm:"autoUpdateTime" json:"updated"`
}

func (User) TableName() string { return "user" }
