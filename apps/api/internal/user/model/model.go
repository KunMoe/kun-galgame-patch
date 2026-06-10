package model

import (
	"time"

	patchModel "kun-galgame-patch-api/internal/patch/model"
)

// UserFollowRelation represents a follow relationship.
//
// FK behavior is asymmetric — inherited from the original Prisma schema and
// preserved as-is when the Go API took over (see 000_baseline.up.sql):
//
//   follower_id  → user(id)   ON DELETE CASCADE   (default — deleting a
//                                                   user wipes their outgoing
//                                                   follows)
//   following_id → user(id)   ON DELETE RESTRICT  (a user who is followed
//                                                   by anyone cannot be
//                                                   deleted; SQLSTATE 23503)
//
// The asymmetry is a historical quirk — there's no business reason
// "removing a popular user" should be harder than "removing an unfollowed
// one", and the patch.user_id RESTRICT (see patch model) already gates user
// deletion. Leaving as-is to avoid silent semantic changes; revisit when /
// if the user-delete flow is reworked.
type UserFollowRelation struct {
	ID          int `gorm:"primaryKey;autoIncrement" json:"id"`
	FollowerID  int `gorm:"uniqueIndex:idx_follow;not null" json:"follower_id"`
	FollowingID int `gorm:"uniqueIndex:idx_follow;not null;constraint:OnDelete:RESTRICT" json:"following_id"`
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
	// AvatarImageHash is the image_service hash for a hash-addressed avatar.
	// The frontend prefers it over the legacy Avatar URL (resolveAvatarUrl) —
	// without it, a user who uploaded a new avatar shows their stale legacy one.
	AvatarImageHash string `json:"avatar_image_hash"`
}

// UserFollowItem is UserBasic + a viewer-relative is_followed flag, used
// by the follower / following list modals so each row can render its own
// follow / unfollow button without a per-row round-trip. When the viewer
// is anonymous (or the listed row is the viewer themselves), is_followed
// is false.
type UserFollowItem struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Avatar     string `json:"avatar"`
	IsFollowed bool   `json:"is_followed"`
}
