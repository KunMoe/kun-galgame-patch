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

func (r *MessageRepository) GetUnreadTypes(recipientID int) ([]string, error) {
	var types []string
	err := r.db.Model(&model.UserMessage{}).
		Where("recipient_id = ? AND status = 0", recipientID).
		Distinct("type").Pluck("type", &types).Error
	return types, err
}

// GetUnreadCounts returns the unread total per message type. Types with zero
// unread rows are absent rather than present-and-zero.
func (r *MessageRepository) GetUnreadCounts(recipientID int) (map[string]int, error) {
	var rows []struct {
		Type  string
		Total int
	}
	err := r.db.Model(&model.UserMessage{}).
		Select("type, count(*) AS total").
		Where("recipient_id = ? AND status = 0", recipientID).
		Group("type").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int, len(rows))
	for _, row := range rows {
		counts[row.Type] = row.Total
	}
	return counts, nil
}

func (r *MessageRepository) MarkAsRead(recipientID int, msgType string) error {
	query := r.db.Model(&model.UserMessage{}).Where("recipient_id = ? AND status = 0", recipientID)
	if msgType != "all" {
		query = query.Where("type = ?", msgType)
	}
	return query.Update("status", 1).Error
}
