package model

import (
	"time"

	patchModel "kun-galgame-patch-api/internal/patch/model"
)

// UserFollowRelation represents a follow relationship
type UserFollowRelation struct {
	ID          int `gorm:"primaryKey;autoIncrement" json:"id"`
	FollowerID  int `gorm:"uniqueIndex:idx_follow;not null" json:"follower_id"`
	FollowingID int `gorm:"uniqueIndex:idx_follow;not null" json:"following_id"`
}

func (UserFollowRelation) TableName() string { return "user_follow_relation" }

// UserMessage represents a user message
type UserMessage struct {
	ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Type        string    `gorm:"not null" json:"type"`
	Content     string    `gorm:"type:varchar(10007)" json:"content"`
	Status      int       `gorm:"default:0" json:"status"`
	Link        string    `gorm:"type:varchar(1007);default:''" json:"link"`
	SenderID    *int      `json:"sender_id"`
	RecipientID *int      `json:"recipient_id"`
	Created     time.Time `gorm:"autoCreateTime" json:"created"`
	Updated     time.Time `gorm:"autoUpdateTime" json:"updated"`

	// Sender is the resolved sender brief, batch-filled by the message
	// handler from OAuth /users/batch (pkg/userclient). NOT a GORM column —
	// after the OAuth migration display fields live on OAuth, the local
	// user_message row only carries sender_id. nil for system messages
	// (sender_id NULL) or when the OAuth lookup misses; the frontend then
	// renders the "系统" placeholder.
	Sender *patchModel.PatchUser `gorm:"-" json:"sender,omitempty"`
}

func (UserMessage) TableName() string { return "user_message" }

// UserBasic contains basic user info (used for list display)
type UserBasic struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}
