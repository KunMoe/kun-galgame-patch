package service

import (
	"kun-galgame-patch-api/internal/message/repository"
	"kun-galgame-patch-api/internal/user/model"
)

type MessageService struct {
	repo *repository.MessageRepository
}

func New(repo *repository.MessageRepository) *MessageService {
	return &MessageService{repo: repo}
}

// GetMessages retrieves paginated messages for a user
func (s *MessageService) GetMessages(recipientID int, msgType string, page, limit int) ([]model.UserMessage, int64, error) {
	return s.repo.GetMessages(recipientID, msgType, (page-1)*limit, limit)
}

// GetUnreadTypes returns the list of unread message types
func (s *MessageService) GetUnreadTypes(recipientID int) ([]string, error) {
	return s.repo.GetUnreadTypes(recipientID)
}

// MarkAsRead marks messages of a given type (or all) as read
func (s *MessageService) MarkAsRead(recipientID int, msgType string) error {
	return s.repo.MarkAsRead(recipientID, msgType)
}
