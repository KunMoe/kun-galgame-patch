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

func (r *ChatRepository) FindRoomByLink(link string) (*model.ChatRoom, error) {
	var room model.ChatRoom
	err := r.db.Where("link = ?", link).First(&room).Error
	return &room, err
}

func (r *ChatRepository) ListRoomsByUser(userID int) ([]model.ChatRoom, error) {
	var rooms []model.ChatRoom
	err := r.db.
		Joins("JOIN chat_member ON chat_member.chat_room_id = chat_room.id").
		Where("chat_member.user_id = ?", userID).
		Where(`EXISTS (
			SELECT 1 FROM chat_message
			WHERE chat_message.chat_room_id = chat_room.id
			  AND chat_message.deleted_at IS NULL
		)`).
		Order("chat_room.last_message_time DESC, chat_room.id DESC").
		Find(&rooms).Error
	return rooms, err
}

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

func (r *ChatRepository) IsMember(userID, roomID int) (bool, error) {
	var count int64
	err := r.db.Model(&model.ChatMember{}).
		Where("user_id = ? AND chat_room_id = ?", userID, roomID).
		Count(&count).Error
	return count > 0, err
}

func (r *ChatRepository) AddMember(userID, roomID int) error {
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.ChatMember{
		UserID:     userID,
		ChatRoomID: roomID,
		Role:       "MEMBER",
	}).Error
}

func (r *ChatRepository) FindOrCreatePrivateRoom(userID, peerUID int) (*model.ChatRoom, error) {
	if userID == peerUID {
		return nil, fmt.Errorf("cannot start a private chat with yourself")
	}
	low, high := userID, peerUID
	if low > high {
		low, high = high, low
	}
	link := fmt.Sprintf("%d-%d", low, high)

	var room model.ChatRoom
	if err := r.db.Where("link = ?", link).First(&room).Error; err == nil {
		_ = r.AddMember(userID, room.ID)
		_ = r.AddMember(peerUID, room.ID)
		return &room, nil
	} else if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	room = model.ChatRoom{Link: link, Type: "PRIVATE"}
	err := r.db.Transaction(func(tx *gorm.DB) error {
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
		var existing model.ChatRoom
		if e := r.db.Where("link = ?", link).First(&existing).Error; e == nil {
			return &existing, nil
		}
		return nil, err
	}
	return &room, nil
}

func (r *ChatRepository) ListMessages(roomID, after, before, limit int) ([]model.ChatMessage, error) {
	var msgs []model.ChatMessage

	if before > 0 {
		err := r.db.
			Where("chat_room_id = ? AND id < ?", roomID, before).
			Order("id DESC").Limit(limit).Find(&msgs).Error
		reverseMessages(msgs)
		return msgs, err
	}

	if after > 0 {
		err := r.db.
			Where("chat_room_id = ? AND id > ?", roomID, after).
			Order("id ASC").Limit(limit).Find(&msgs).Error
		return msgs, err
	}

	err := r.db.
		Where("chat_room_id = ?", roomID).
		Order("id DESC").Limit(limit).Find(&msgs).Error
	reverseMessages(msgs)
	return msgs, err
}

func reverseMessages(m []model.ChatMessage) {
	for i, j := 0, len(m)-1; i < j; i, j = i+1, j-1 {
		m[i], m[j] = m[j], m[i]
	}
}

func (r *ChatRepository) ListMessagesByIDsInRoom(roomID int, ids []int) ([]model.ChatMessage, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var msgs []model.ChatMessage
	err := r.db.
		Where("chat_room_id = ? AND id IN ?", roomID, ids).
		Order("id ASC").Find(&msgs).Error
	return msgs, err
}

func (r *ChatRepository) LatestMessagePerRoom(roomIDs []int) (map[int]model.ChatMessage, error) {
	out := map[int]model.ChatMessage{}
	if len(roomIDs) == 0 {
		return out, nil
	}
	var msgs []model.ChatMessage
	err := r.db.
		Where(`id IN (
			SELECT MAX(id) FROM chat_message
			WHERE chat_room_id IN ?
			GROUP BY chat_room_id
		)`, roomIDs).
		Find(&msgs).Error
	if err != nil {
		return out, err
	}
	for i := range msgs {
		out[msgs[i].ChatRoomID] = msgs[i]
	}
	return out, nil
}

func (r *ChatRepository) ListMembers(roomID int) ([]model.ChatMember, error) {
	var members []model.ChatMember
	err := r.db.Where("chat_room_id = ?", roomID).
		Order("created ASC, id ASC").
		Find(&members).Error
	return members, err
}

func (r *ChatRepository) CreateMessage(m *model.ChatMessage) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		return tx.Model(&model.ChatRoom{}).Where("id = ?", m.ChatRoomID).
			UpdateColumn("last_message_time", m.Created).Error
	})
}

func (r *ChatRepository) GetMessage(id int) (*model.ChatMessage, error) {
	var m model.ChatMessage
	err := r.db.First(&m, id).Error
	return &m, err
}

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

func (r *ChatRepository) SoftDeleteMessage(id, deletedByUID int, deletedAt any) error {
	return r.db.Model(&model.ChatMessage{}).Where("id = ?", id).Updates(map[string]any{
		"status":        "DELETED",
		"deleted_at":    deletedAt,
		"deleted_by_id": deletedByUID,
	}).Error
}

func (r *ChatRepository) ToggleReaction(messageID, userID int, emoji string) (added bool, err error) {
	var existing model.ChatMessageReaction
	err = r.db.Where("chat_message_id = ? AND user_id = ? AND emoji = ?", messageID, userID, emoji).
		First(&existing).Error
	if err == nil {
		return false, r.db.Delete(&existing).Error
	}
	if err != gorm.ErrRecordNotFound {
		return false, err
	}
	return true, r.db.Create(&model.ChatMessageReaction{
		ChatMessageID: messageID,
		UserID:        userID,
		Emoji:         emoji,
	}).Error
}

func (r *ChatRepository) ListReactionsByMessageIDs(ids []int) ([]model.ChatMessageReaction, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rs []model.ChatMessageReaction
	err := r.db.Where("chat_message_id IN ?", ids).Order("id ASC").Find(&rs).Error
	return rs, err
}

func (r *ChatRepository) GetMessagesByIDs(ids []int) ([]model.ChatMessage, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var msgs []model.ChatMessage
	err := r.db.Where("id IN ?", ids).Find(&msgs).Error
	return msgs, err
}

func (r *ChatRepository) MarkSeen(roomID, userID int, messageIDs []int) error {
	if len(messageIDs) == 0 {
		return nil
	}

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
			UserID:        userID,
		})
	}
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&records).Error
}

// CountUnread counts every message the user has not seen: messages in the rooms
// they belong to, sent by somebody else, not withdrawn, with no chat_message_seen
// row for this user. The seen rows are written by MarkSeen when the chat page
// loads a room, so an unopened room contributes its whole tail.
func (r *ChatRepository) CountUnread(userID int) (int, error) {
	var total int64
	err := r.db.Model(&model.ChatMessage{}).
		Joins("JOIN chat_member ON chat_member.chat_room_id = chat_message.chat_room_id AND chat_member.user_id = ?", userID).
		Joins("LEFT JOIN chat_message_seen ON chat_message_seen.chat_message_id = chat_message.id AND chat_message_seen.user_id = ?", userID).
		Where("chat_message.sender_id <> ?", userID).
		Where("chat_message.status <> ?", "DELETED").
		Where("chat_message_seen.id IS NULL").
		Count(&total).Error
	return int(total), err
}

// MarkRoomSeen marks every message in the room as seen for the user. Opening a
// room is the read event, and the room may hold more unread than the page loads
// (PAGE = 50), so this writes the tail the client never fetched.
func (r *ChatRepository) MarkRoomSeen(roomID, userID int) error {
	return r.db.Exec(`
		INSERT INTO chat_message_seen (chat_message_id, user_id)
		SELECT m.id, ?
		FROM chat_message m
		WHERE m.chat_room_id = ?
		  AND m.sender_id <> ?
		  AND NOT EXISTS (
			SELECT 1 FROM chat_message_seen s
			WHERE s.chat_message_id = m.id AND s.user_id = ?
		  )
		ON CONFLICT (user_id, chat_message_id) DO NOTHING
	`, userID, roomID, userID, userID).Error
}
