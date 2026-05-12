// Package model defines GORM models for the chat module.
//
// Per decision D9 (2026-04-21), WebSocket/Socket.IO is no longer used; all reads
// and writes go through REST. The tables retain the original Prisma schema,
// just without real-time push.
package model

import (
	"time"

	patchModel "kun-galgame-patch-api/internal/patch/model"
)

// ChatRoom is a chat room (private or group).
//
//   - Type = "PRIVATE": private chat, Link format is "{minUid}-{maxUid}"
//   - Type = "GROUP": group chat, Link is a shareable link
type ChatRoom struct {
	ID              int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Name            string    `gorm:"type:varchar(107)" json:"name"`
	Link            string    `gorm:"uniqueIndex;type:varchar(17)" json:"link"`
	Avatar          string    `gorm:"type:varchar(1007);default:''" json:"avatar"`
	Type            string    `gorm:"default:'PRIVATE'" json:"type"`
	LastMessageTime time.Time `gorm:"autoCreateTime" json:"last_message_time"`
	Created         time.Time `gorm:"autoCreateTime" json:"created"`
	Updated         time.Time `gorm:"autoUpdateTime" json:"updated"`
}

func (ChatRoom) TableName() string { return "chat_room" }

// ChatMember is a chat room member.
type ChatMember struct {
	ID         int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Role       string    `gorm:"default:'MEMBER'" json:"role"` // OWNER / ADMIN / MEMBER
	UserID     int       `gorm:"uniqueIndex:idx_user_room;not null" json:"user_id"`
	ChatRoomID int       `gorm:"uniqueIndex:idx_user_room;not null" json:"chat_room_id"`
	Created    time.Time `gorm:"autoCreateTime" json:"created"`
	Updated    time.Time `gorm:"autoUpdateTime" json:"updated"`

	// Filled by the chat handler from OAuth /users/batch (pkg/userclient).
	User *patchModel.PatchUser `gorm:"-" json:"user,omitempty"`
}

func (ChatMember) TableName() string { return "chat_member" }

// ChatMessage is a chat message.
type ChatMessage struct {
	ID          int        `gorm:"primaryKey;autoIncrement" json:"id"`
	Content     string     `gorm:"type:varchar(2000);default:''" json:"content"`
	FileURL     string     `gorm:"type:varchar(1007);default:''" json:"file_url"`
	Status      string     `gorm:"default:'SENT'" json:"status"` // SENT / EDITED / DELETED
	DeletedAt   *time.Time `json:"deleted_at"`
	DeletedByID *int       `json:"deleted_by_id"`
	ChatRoomID  int        `gorm:"index;not null" json:"chat_room_id"`
	SenderID    int        `gorm:"not null" json:"sender_id"`
	ReplyToID   *int       `json:"reply_to_id"`
	Created     time.Time  `gorm:"autoCreateTime" json:"created"`
	Updated     time.Time  `gorm:"autoUpdateTime" json:"updated"`

	// Filled by the chat handler from OAuth /users/batch (pkg/userclient).
	Sender *patchModel.PatchUser `gorm:"-" json:"sender,omitempty"`
}

func (ChatMessage) TableName() string { return "chat_message" }

// ChatMessageSeen is the seen state of a message (a given user has read a given message).
type ChatMessageSeen struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	ChatMessageID int       `gorm:"uniqueIndex:idx_user_msg_seen;not null" json:"chat_message_id"`
	UserID        int       `gorm:"uniqueIndex:idx_user_msg_seen;not null" json:"user_id"`
	ReadAt        time.Time `gorm:"autoCreateTime" json:"read_at"`
}

func (ChatMessageSeen) TableName() string { return "chat_message_seen" }

// ChatMessageReaction is an emoji reaction on a message.
type ChatMessageReaction struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Emoji         string    `gorm:"type:varchar(10);uniqueIndex:idx_user_msg_emoji" json:"emoji"`
	ChatMessageID int       `gorm:"uniqueIndex:idx_user_msg_emoji;not null" json:"chat_message_id"`
	UserID        int       `gorm:"uniqueIndex:idx_user_msg_emoji;not null" json:"user_id"`
	Created       time.Time `gorm:"autoCreateTime" json:"created"`
	Updated       time.Time `gorm:"autoUpdateTime" json:"updated"`
}

func (ChatMessageReaction) TableName() string { return "chat_message_reaction" }

// ChatMessageEditHistory is the edit history of a message.
type ChatMessageEditHistory struct {
	ID              int       `gorm:"primaryKey;autoIncrement" json:"id"`
	PreviousContent string    `gorm:"type:varchar(2000)" json:"previous_content"`
	ChatMessageID   int       `gorm:"index;not null" json:"chat_message_id"`
	EditedAt        time.Time `gorm:"autoCreateTime" json:"edited_at"`
}

func (ChatMessageEditHistory) TableName() string { return "chat_message_edit_history" }
