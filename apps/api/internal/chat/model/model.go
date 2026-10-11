package model

import (
	"time"

	patchModel "kun-galgame-patch-api/internal/patch/model"
)

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

type RoomSummaryView struct {
	ID              int       `json:"id"`
	Link            string    `json:"link"`
	Type            string    `json:"type"`
	Name            string    `json:"name"`
	Avatar          string    `json:"avatar"`
	AvatarImageHash string    `json:"avatar_image_hash,omitempty"`
	LastMessage     string    `json:"last_message"`
	LastMessageTime time.Time `json:"last_message_time"`
	Created         time.Time `json:"created"`
	Updated         time.Time `json:"updated"`
}

type ChatMember struct {
	ID         int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Role       string    `gorm:"default:'MEMBER'" json:"role"`
	UserID     int       `gorm:"uniqueIndex:idx_user_room;not null" json:"user_id"`
	ChatRoomID int       `gorm:"uniqueIndex:idx_user_room;not null" json:"chat_room_id"`
	Created    time.Time `gorm:"autoCreateTime" json:"created"`
	Updated    time.Time `gorm:"autoUpdateTime" json:"updated"`

	User *patchModel.PatchUser `gorm:"-" json:"user,omitempty"`
}

func (ChatMember) TableName() string { return "chat_member" }

type ChatMessage struct {
	ID          int        `gorm:"primaryKey;autoIncrement" json:"id"`
	Content     string     `gorm:"type:varchar(2000);default:''" json:"content"`
	FileURL     string     `gorm:"type:varchar(1007);default:''" json:"file_url"`
	Status      string     `gorm:"default:'SENT'" json:"status"`
	DeletedAt   *time.Time `json:"deleted_at"`
	DeletedByID *int       `gorm:"constraint:OnDelete:SET NULL" json:"deleted_by_id"`
	ChatRoomID  int        `gorm:"index;not null" json:"chat_room_id"`
	SenderID    int        `gorm:"not null" json:"sender_id"`
	ReplyToID   *int       `gorm:"constraint:OnDelete:SET NULL" json:"reply_to_id"`
	Created     time.Time  `gorm:"autoCreateTime" json:"created"`
	Updated     time.Time  `gorm:"autoUpdateTime" json:"updated"`

	Sender *patchModel.PatchUser `gorm:"-" json:"sender,omitempty"`

	ContentHTML  string             `gorm:"-" json:"content_html"`
	Reaction     []ChatReactionView `gorm:"-" json:"reaction"`
	QuoteMessage *ChatQuoteView     `gorm:"-" json:"quote_message,omitempty"`
}

func (ChatMessage) TableName() string { return "chat_message" }

type ChatReactionView struct {
	ID    int                   `json:"id"`
	Emoji string                `json:"emoji"`
	User  *patchModel.PatchUser `json:"user"`
}

type ChatQuoteView struct {
	ID         int    `json:"id"`
	SenderName string `json:"sender_name"`
	Content    string `json:"content"`
}

type ChatMessageSeen struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	ChatMessageID int       `gorm:"uniqueIndex:idx_user_msg_seen;not null" json:"chat_message_id"`
	UserID        int       `gorm:"uniqueIndex:idx_user_msg_seen;not null" json:"user_id"`
	ReadAt        time.Time `gorm:"autoCreateTime" json:"read_at"`
}

func (ChatMessageSeen) TableName() string { return "chat_message_seen" }

type ChatMessageReaction struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Emoji         string    `gorm:"type:varchar(10);uniqueIndex:idx_user_msg_emoji" json:"emoji"`
	ChatMessageID int       `gorm:"uniqueIndex:idx_user_msg_emoji;not null" json:"chat_message_id"`
	UserID        int       `gorm:"uniqueIndex:idx_user_msg_emoji;not null" json:"user_id"`
	Created       time.Time `gorm:"autoCreateTime" json:"created"`
	Updated       time.Time `gorm:"autoUpdateTime" json:"updated"`
}

func (ChatMessageReaction) TableName() string { return "chat_message_reaction" }

type ChatMessageEditHistory struct {
	ID              int       `gorm:"primaryKey;autoIncrement" json:"id"`
	PreviousContent string    `gorm:"type:varchar(2000)" json:"previous_content"`
	ChatMessageID   int       `gorm:"index;not null" json:"chat_message_id"`
	EditedAt        time.Time `gorm:"autoCreateTime" json:"edited_at"`
}

func (ChatMessageEditHistory) TableName() string { return "chat_message_edit_history" }
