package model

import (
	"time"

	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/userclient"
)

type UserFollowRelation struct {
	ID          int `gorm:"primaryKey;autoIncrement" json:"id"`
	FollowerID  int `gorm:"uniqueIndex:idx_follow;not null" json:"follower_id"`
	FollowingID int `gorm:"uniqueIndex:idx_follow;not null;constraint:OnDelete:RESTRICT" json:"following_id"`
}

func (UserFollowRelation) TableName() string { return "user_follow_relation" }

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

	Sender *patchModel.PatchUser `gorm:"-" json:"sender,omitempty"`

	GalgameName map[string]string `gorm:"-" json:"galgame_name,omitempty"`
}

func (UserMessage) TableName() string { return "user_message" }

type UserBasic struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Avatar          string `json:"avatar"`
	AvatarImageHash string `json:"avatar_image_hash"`
}

type UserFollowItem struct {
	ID         int                   `json:"id"`
	Name       string                `json:"name"`
	Avatar     string                `json:"avatar"`
	Cosmetics  *userclient.Cosmetics `json:"cosmetics,omitempty"`
	IsFollowed bool                  `json:"is_followed"`
}
