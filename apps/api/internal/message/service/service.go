package service

import (
	"context"

	"kun-galgame-patch-api/internal/message/repository"
	"kun-galgame-patch-api/internal/user/model"
)

type ReadForwarder interface {
	ForwardRead(ctx context.Context, userID int, ids []int64)
}

type MessageService struct {
	repo      *repository.MessageRepository
	forwarder ReadForwarder
}

func New(repo *repository.MessageRepository, forwarder ReadForwarder) *MessageService {
	return &MessageService{repo: repo, forwarder: forwarder}
}

func (s *MessageService) GetMessages(recipientID int, msgType string, page, limit int) ([]model.UserMessage, int64, error) {
	return s.repo.GetMessages(recipientID, msgType, (page-1)*limit, limit)
}

func (s *MessageService) GetUnreadTypes(recipientID int) ([]string, error) {
	return s.repo.GetUnreadTypes(recipientID)
}

func (s *MessageService) MarkAsRead(ctx context.Context, recipientID int, msgType string) error {
	ids, err := s.repo.MarkAsRead(recipientID, msgType)
	if err != nil {
		return err
	}
	if s.forwarder != nil {
		s.forwarder.ForwardRead(ctx, recipientID, ids)
	}
	return nil
}
