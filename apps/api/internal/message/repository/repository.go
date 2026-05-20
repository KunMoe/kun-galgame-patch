package repository

import (
	"kun-galgame-patch-api/internal/user/model"

	"gorm.io/gorm"
)

type MessageRepository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// GetMessages retrieves messages for a user, optionally filtered by type.
//
// NOTE: Count and Find MUST run on independent statements. gorm v2 mutates
// the shared *gorm.DB in place once a chain has been cloned, and Count leaves
// `SELECT count(*)` on the statement. Reusing the same builder for the
// subsequent Find produced `SELECT count(*) ... LIMIT n`, scanning one count
// row into []UserMessage — i.e. an always-empty message list while `total`
// still looked correct. `.Session(&gorm.Session{})` forks a fresh statement
// for each finisher so they can't pollute each other.
func (r *MessageRepository) GetMessages(recipientID int, msgType string, offset, limit int) ([]model.UserMessage, int64, error) {
	var messages []model.UserMessage
	var total int64

	base := r.db.Model(&model.UserMessage{}).Where("recipient_id = ?", recipientID)
	if msgType != "" {
		base = base.Where("type = ?", msgType)
	}

	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := base.Session(&gorm.Session{}).
		Order("created DESC, id DESC").Offset(offset).Limit(limit).
		Find(&messages).Error
	return messages, total, err
}

// GetUnreadTypes returns distinct types of unread messages
func (r *MessageRepository) GetUnreadTypes(recipientID int) ([]string, error) {
	var types []string
	err := r.db.Model(&model.UserMessage{}).
		Where("recipient_id = ? AND status = 0", recipientID).
		Distinct("type").Pluck("type", &types).Error
	return types, err
}

// CreateMessage creates a new message
func (r *MessageRepository) CreateMessage(msg *model.UserMessage) error {
	return r.db.Create(msg).Error
}

// MarkAsRead marks messages as read by type (or all if type is "all")
func (r *MessageRepository) MarkAsRead(recipientID int, msgType string) error {
	query := r.db.Model(&model.UserMessage{}).Where("recipient_id = ? AND status = 0", recipientID)
	if msgType != "all" {
		query = query.Where("type = ?", msgType)
	}
	return query.Update("status", 1).Error
}
