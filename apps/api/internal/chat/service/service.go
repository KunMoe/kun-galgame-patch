package service

import (
	"fmt"
	"time"

	"kun-galgame-patch-api/internal/chat/model"
	"kun-galgame-patch-api/internal/chat/repository"
	"kun-galgame-patch-api/internal/infrastructure/markdown"

	"github.com/rs/xid"
)

type ChatService struct {
	repo *repository.ChatRepository
}

func New(repo *repository.ChatRepository) *ChatService {
	return &ChatService{repo: repo}
}

func (s *ChatService) ListRooms(userID int) ([]model.ChatRoom, error) {
	return s.repo.ListRoomsByUser(userID)
}

func (s *ChatService) CreateGroupRoom(ownerUID int, name, avatar string) (*model.ChatRoom, error) {
	link := xid.New().String()
	return s.repo.CreateRoom(ownerUID, name, link, avatar)
}

func (s *ChatService) JoinRoomByLink(userID int, link string) (*model.ChatRoom, error) {
	room, err := s.repo.FindRoomByLink(link)
	if err != nil {
		return nil, fmt.Errorf("房间不存在")
	}
	if err := s.repo.AddMember(userID, room.ID); err != nil {
		return nil, fmt.Errorf("加入失败: %w", err)
	}
	return room, nil
}

func (s *ChatService) StartPrivateChat(userID, peerUID int) (*model.ChatRoom, error) {
	return s.repo.FindOrCreatePrivateRoom(userID, peerUID)
}

type RoomDetail struct {
	model.ChatRoom
	Member []model.ChatMember `json:"member"`
}

func (s *ChatService) GetRoomDetail(userID int, link string) (*RoomDetail, error) {
	room, err := s.resolveRoomForMember(userID, link)
	if err != nil {
		return nil, err
	}
	members, err := s.repo.ListMembers(room.ID)
	if err != nil {
		return nil, err
	}
	return &RoomDetail{ChatRoom: *room, Member: members}, nil
}

func (s *ChatService) GetMessages(userID int, link string, after, before, limit int) ([]model.ChatMessage, error) {
	room, err := s.resolveRoomForMember(userID, link)
	if err != nil {
		return nil, err
	}
	return s.repo.ListMessages(room.ID, after, before, limit)
}

func (s *ChatService) LatestMessagePerRoom(roomIDs []int) (map[int]model.ChatMessage, error) {
	return s.repo.LatestMessagePerRoom(roomIDs)
}

func (s *ChatService) GetMessagesByIDsInRoom(userID int, link string, ids []int) ([]model.ChatMessage, error) {
	room, err := s.resolveRoomForMember(userID, link)
	if err != nil {
		return nil, err
	}
	return s.repo.ListMessagesByIDsInRoom(room.ID, ids)
}

func (s *ChatService) CreateMessage(userID int, link string, content, fileURL string, replyToID *int) (*model.ChatMessage, error) {
	room, err := s.resolveRoomForMember(userID, link)
	if err != nil {
		return nil, err
	}
	if content == "" && fileURL == "" {
		return nil, fmt.Errorf("消息内容不能为空")
	}
	if replyToID != nil {
		target, terr := s.repo.GetMessage(*replyToID)
		if terr != nil || target.ChatRoomID != room.ID {
			return nil, fmt.Errorf("无效的引用消息")
		}
	}
	msg := &model.ChatMessage{
		ChatRoomID: room.ID,
		SenderID:   userID,
		Content:    markdown.NormalizeContentImageURLs(content),
		FileURL:    fileURL,
		ReplyToID:  replyToID,
		Status:     "SENT",
	}
	if err := s.repo.CreateMessage(msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *ChatService) UpdateMessage(userID, messageID int, newContent string) error {
	m, err := s.repo.GetMessage(messageID)
	if err != nil {
		return fmt.Errorf("消息不存在")
	}
	if m.SenderID != userID {
		return fmt.Errorf("仅发送者可以编辑消息")
	}
	if m.Status == "DELETED" {
		return fmt.Errorf("已删除的消息无法编辑")
	}
	return s.repo.UpdateMessageContent(m, m.Content, markdown.NormalizeContentImageURLs(newContent))
}

func (s *ChatService) DeleteMessage(userID int, isPrivileged bool, messageID int) error {
	m, err := s.repo.GetMessage(messageID)
	if err != nil {
		return fmt.Errorf("消息不存在")
	}
	if m.SenderID != userID && !isPrivileged {
		return fmt.Errorf("仅发送者或版主可删除消息")
	}
	now := time.Now()
	return s.repo.SoftDeleteMessage(messageID, userID, now)
}

func (s *ChatService) ToggleReaction(userID, messageID int, emoji string) (bool, error) {
	m, err := s.repo.GetMessage(messageID)
	if err != nil {
		return false, fmt.Errorf("消息不存在")
	}
	ok, err := s.repo.IsMember(userID, m.ChatRoomID)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, fmt.Errorf("您不是该房间的成员")
	}
	return s.repo.ToggleReaction(messageID, userID, emoji)
}

func (s *ChatService) MarkSeen(userID int, link string, messageIDs []int) error {
	room, err := s.resolveRoomForMember(userID, link)
	if err != nil {
		return err
	}
	return s.repo.MarkSeen(room.ID, userID, messageIDs)
}

func (s *ChatService) resolveRoomForMember(userID int, link string) (*model.ChatRoom, error) {
	room, err := s.repo.FindRoomByLink(link)
	if err != nil {
		return nil, fmt.Errorf("房间不存在")
	}
	ok, err := s.repo.IsMember(userID, room.ID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("您不是该房间的成员")
	}
	return room, nil
}

func (s *ChatService) ReactionsByMessageIDs(ids []int) ([]model.ChatMessageReaction, error) {
	return s.repo.ListReactionsByMessageIDs(ids)
}

func (s *ChatService) MessagesByIDs(ids []int) ([]model.ChatMessage, error) {
	return s.repo.GetMessagesByIDs(ids)
}
