// Package repository is the data access layer for the chat module.
package repository

import (
	"fmt"

	"kun-galgame-patch-api/internal/chat/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ChatRepository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *ChatRepository {
	return &ChatRepository{db: db}
}

// ─── Room ───────────────────────────────────────────

// FindRoomByLink looks up a room by its link.
func (r *ChatRepository) FindRoomByLink(link string) (*model.ChatRoom, error) {
	var room model.ChatRoom
	err := r.db.Where("link = ?", link).First(&room).Error
	return &room, err
}

// ListRoomsByUser lists all rooms a user has joined, ordered by last message time desc.
func (r *ChatRepository) ListRoomsByUser(uid int) ([]model.ChatRoom, error) {
	var rooms []model.ChatRoom
	err := r.db.
		Joins("JOIN chat_member ON chat_member.chat_room_id = chat_room.id").
		Where("chat_member.user_id = ?", uid).
		Order("chat_room.last_message_time DESC").
		Find(&rooms).Error
	return rooms, err
}

// CreateRoom creates a group room and inserts the owner as the first member.
func (r *ChatRepository) CreateRoom(ownerUID int, name, link, avatar string) (*model.ChatRoom, error) {
	room := &model.ChatRoom{
		Name:   name,
		Link:   link,
		Avatar: avatar,
		Type:   "GROUP",
	}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(room).Error; err != nil {
			return err
		}
		return tx.Create(&model.ChatMember{
			UserID:     ownerUID,
			ChatRoomID: room.ID,
			Role:       "OWNER",
		}).Error
	})
	return room, err
}

// IsMember reports whether a user is a member of a given room.
func (r *ChatRepository) IsMember(uid, roomID int) (bool, error) {
	var count int64
	err := r.db.Model(&model.ChatMember{}).
		Where("user_id = ? AND chat_room_id = ?", uid, roomID).
		Count(&count).Error
	return count > 0, err
}

// AddMember joins a room; idempotent if already a member.
func (r *ChatRepository) AddMember(uid, roomID int) error {
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.ChatMember{
		UserID:     uid,
		ChatRoomID: roomID,
		Role:       "MEMBER",
	}).Error
}

// FindOrCreatePrivateRoom returns the unique private chat room between two
// users, creating it (with both members) if it does not yet exist.
//
// The room link is "<low>-<high>" with low < high so the two directions
// (A→B and B→A) always converge to the same row. The natural unique index
// on chat_room.link prevents duplicate rooms even under concurrent first
// invocations.
func (r *ChatRepository) FindOrCreatePrivateRoom(uid, peerUID int) (*model.ChatRoom, error) {
	if uid == peerUID {
		return nil, fmt.Errorf("cannot start a private chat with yourself")
	}
	low, high := uid, peerUID
	if low > high {
		low, high = high, low
	}
	link := fmt.Sprintf("%d-%d", low, high)

	var room model.ChatRoom
	if err := r.db.Where("link = ?", link).First(&room).Error; err == nil {
		// Defensive: re-affirm membership in case some old row is missing one
		// side (e.g. a legacy chat_member row was deleted by a script).
		_ = r.AddMember(uid, room.ID)
		_ = r.AddMember(peerUID, room.ID)
		return &room, nil
	} else if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	room = model.ChatRoom{Link: link, Type: "PRIVATE"}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Race: if two concurrent callers reach here, the unique index on
		// link makes the second one fail; that path falls through to a
		// follow-up lookup below.
		if err := tx.Create(&room).Error; err != nil {
			return err
		}
		members := []model.ChatMember{
			{UserID: low, ChatRoomID: room.ID, Role: "MEMBER"},
			{UserID: high, ChatRoomID: room.ID, Role: "MEMBER"},
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&members).Error
	})
	if err != nil {
		// Race fallback: someone else inserted the same row first.
		var existing model.ChatRoom
		if e := r.db.Where("link = ?", link).First(&existing).Error; e == nil {
			return &existing, nil
		}
		return nil, err
	}
	return &room, nil
}

// ─── Message ────────────────────────────────────────

// ListMessages fetches new messages with id > after, capped by limit, newest-first
// but returned in ascending order so the frontend can append to the bottom of the
// transcript directly.
func (r *ChatRepository) ListMessages(roomID, after, limit int) ([]model.ChatMessage, error) {
	var msgs []model.ChatMessage
	err := r.db.
		Where("chat_room_id = ? AND id > ?", roomID, after).
		Order("id ASC").
		Limit(limit).
		Find(&msgs).Error
	return msgs, err
}

// ListMembers returns all members of a room. The handler layer attaches the
// user briefs from OAuth /users/batch.
func (r *ChatRepository) ListMembers(roomID int) ([]model.ChatMember, error) {
	var members []model.ChatMember
	err := r.db.Where("chat_room_id = ?", roomID).
		Order("created ASC").
		Find(&members).Error
	return members, err
}

// CreateMessage writes the message and updates the room's last_message_time (in a transaction).
func (r *ChatRepository) CreateMessage(m *model.ChatMessage) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		return tx.Model(&model.ChatRoom{}).Where("id = ?", m.ChatRoomID).
			UpdateColumn("last_message_time", m.Created).Error
	})
}

// GetMessage fetches a single message by ID.
func (r *ChatRepository) GetMessage(id int) (*model.ChatMessage, error) {
	var m model.ChatMessage
	err := r.db.First(&m, id).Error
	return &m, err
}

// UpdateMessageContent edits a message and writes the edit history (in a transaction).
func (r *ChatRepository) UpdateMessageContent(m *model.ChatMessage, oldContent, newContent string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model.ChatMessageEditHistory{
			ChatMessageID:   m.ID,
			PreviousContent: oldContent,
		}).Error; err != nil {
			return err
		}
		return tx.Model(m).Updates(map[string]any{
			"content": newContent,
			"status":  "EDITED",
		}).Error
	})
}

// SoftDeleteMessage soft-deletes a message.
func (r *ChatRepository) SoftDeleteMessage(id, deletedByUID int, deletedAt any) error {
	return r.db.Model(&model.ChatMessage{}).Where("id = ?", id).Updates(map[string]any{
		"status":        "DELETED",
		"deleted_at":    deletedAt,
		"deleted_by_id": deletedByUID,
	}).Error
}

// ─── Reactions ──────────────────────────────────────

// ToggleReaction toggles an emoji reaction. added=true means added, false means removed.
func (r *ChatRepository) ToggleReaction(messageID, uid int, emoji string) (added bool, err error) {
	var existing model.ChatMessageReaction
	err = r.db.Where("chat_message_id = ? AND user_id = ? AND emoji = ?", messageID, uid, emoji).
		First(&existing).Error
	if err == nil {
		// Already exists -> remove
		return false, r.db.Delete(&existing).Error
	}
	if err != gorm.ErrRecordNotFound {
		return false, err
	}
	// Not present -> add
	return true, r.db.Create(&model.ChatMessageReaction{
		ChatMessageID: messageID,
		UserID:        uid,
		Emoji:         emoji,
	}).Error
}

// ─── Seen ───────────────────────────────────────────

// MarkSeen writes seen markers in bulk. Duplicate inserts are ignored via OnConflict DoNothing.
func (r *ChatRepository) MarkSeen(roomID, uid int, messageIDs []int) error {
	if len(messageIDs) == 0 {
		return nil
	}

	// Only accept messages that belong to this room; filter first
	var validIDs []int
	if err := r.db.Model(&model.ChatMessage{}).
		Where("chat_room_id = ? AND id IN ?", roomID, messageIDs).
		Pluck("id", &validIDs).Error; err != nil {
		return fmt.Errorf("校验消息归属失败: %w", err)
	}
	if len(validIDs) == 0 {
		return nil
	}

	records := make([]model.ChatMessageSeen, 0, len(validIDs))
	for _, id := range validIDs {
		records = append(records, model.ChatMessageSeen{
			ChatMessageID: id,
			UserID:        uid,
		})
	}
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&records).Error
}
