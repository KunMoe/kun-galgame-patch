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

// RoomSummaryView is the room-list row. For PRIVATE rooms Name/Avatar are
// overridden with the *peer's* identity (the stored room has no name/avatar);
// LastMessage is a short text preview of the most recent message so the list
// shows real context instead of a placeholder.
type RoomSummaryView struct {
	ID              int       `json:"id"`
	Link            string    `json:"link"`
	Type            string    `json:"type"`
	Name            string    `json:"name"`
	Avatar          string    `json:"avatar"`
	LastMessage     string    `json:"last_message"`
	LastMessageTime time.Time `json:"last_message_time"`
	Created         time.Time `json:"created"`
	Updated         time.Time `json:"updated"`
}

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

	// Enrichment fields (all gorm:"-") filled by the handler before
	// serialization so the frontend can render markdown, reactions and the
	// replied-to quote without extra round-trips.
	//
	//   - ContentHTML: Content rendered through the markdown pipeline +
	//     sanitized. Content itself stays raw markdown (the edit modal needs it).
	//   - Reaction:    flat list of this message's reactions, each with the
	//     reacting user's brief.
	//   - QuoteMessage: the message this one replies to (sender name + HTML),
	//     nil when ReplyToID is nil or the target is gone.
	ContentHTML  string                `gorm:"-" json:"content_html"`
	Reaction     []ChatReactionView    `gorm:"-" json:"reaction"`
	QuoteMessage *ChatQuoteView        `gorm:"-" json:"quote_message,omitempty"`
}

func (ChatMessage) TableName() string { return "chat_message" }

// ChatReactionView is one reaction enriched with the reacting user's brief,
// matching the frontend's `message.reaction[]` shape.
type ChatReactionView struct {
	ID    int                   `json:"id"`
	Emoji string                `json:"emoji"`
	User  *patchModel.PatchUser `json:"user"`
}

// ChatQuoteView is the compact preview of a replied-to message.
type ChatQuoteView struct {
	ID         int    `json:"id"`
	SenderName string `json:"sender_name"`
	Content    string `json:"content"` // rendered HTML (or "该消息已删除")
}

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
