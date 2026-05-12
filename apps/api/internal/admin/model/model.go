package model

import (
	"time"

	patchModel "kun-galgame-patch-api/internal/patch/model"
)

// AdminLog represents an admin action log entry
type AdminLog struct {
	ID      int    `gorm:"primaryKey;autoIncrement" json:"id"`
	Type    string `gorm:"not null" json:"type"`
	Content string `gorm:"type:varchar(10007)" json:"content"`
	Status  int    `gorm:"default:0" json:"status"`
	UserID  int    `gorm:"not null" json:"user_id"`
	// Filled by the admin handler from OAuth /users/batch (pkg/userclient).
	User    *patchModel.PatchUser `gorm:"-" json:"user,omitempty"`
	Created time.Time             `gorm:"autoCreateTime" json:"created"`
	Updated time.Time             `gorm:"autoUpdateTime" json:"updated"`
}

func (AdminLog) TableName() string { return "admin_log" }
