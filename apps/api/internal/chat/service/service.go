// Package service is the business logic layer for the chat module.
//
// D9 (2026-04-21): all reads and writes go through REST; no real-time push.
// The frontend polls GetMessages(after=lastMsgId) every 3s for new messages;
// edits/deletes/reaction state changes are not synced via polling and are only
// visible after a page refresh.
package service

import (
	"fmt"
	"time"

	"kun-galgame-patch-api/internal/chat/model"
	"kun-galgame-patch-api/internal/chat/repository"

	"github.com/rs/xid"
	"gorm.io/gorm"
)

type ChatService struct {
	repo *repository.ChatRepository
}

func New(repo *repository.ChatRepository) *ChatService {
	return &ChatService{repo: repo}
}

// ─── Room ───────────────────────────────────────────

// ListRooms lists all rooms the user has joined.
func (s *ChatService) ListRooms(uid int) ([]model.ChatRoom, error) {
	return s.repo.ListRoomsByUser(uid)
}

// CreateGroupRoom creates a group chat. Only role >= 4 is allowed (checked in the handler layer).
func (s *ChatService) CreateGroupRoom(ownerUID int, name, avatar string) (*model.ChatRoom, error) {
	link := xid.New().String() // 20-char sortable unique id
	return s.repo.CreateRoom(ownerUID, name, link, avatar)
}

// JoinRoomByLink joins a room via its link.
func (s *ChatService) JoinRoomByLink(uid int, link string) (*model.ChatRoom, error) {
	room, err := s.repo.FindRoomByLink(link)
	if err != nil {
		return nil, fmt.Errorf("房间不存在")
	}
	if err := s.repo.AddMember(uid, room.ID); err != nil {
		return nil, fmt.Errorf("加入失败: %w", err)
	}
	return room, nil
}

// StartPrivateChat returns the private chat room between the current user
// and peerUID, creating it on first call. The caller then navigates to
// /message/chat/<room.link>.
func (s *ChatService) StartPrivateChat(uid, peerUID int) (*model.ChatRoom, error) {
	return s.repo.FindOrCreatePrivateRoom(uid, peerUID)
}

// RoomDetail is the shape returned by GET /api/v1/chat/room/:link:
// room metadata plus the full member list with each user's profile.
type RoomDetail struct {
	model.ChatRoom
	Member []model.ChatMember `json:"member"`
}

// GetRoomDetail returns the room and its members. Caller must be a member.
func (s *ChatService) GetRoomDetail(uid int, link string) (*RoomDetail, error) {
	room, err := s.resolveRoomForMember(uid, link)
	if err != nil {
		return nil, err
	}
	members, err := s.repo.ListMembers(room.ID)
	if err != nil {
		return nil, err
	}
	return &RoomDetail{ChatRoom: *room, Member: members}, nil
}

// ─── Messages ───────────────────────────────────────

// GetMessages polls for new messages.
//
// Resolves the room by link and checks membership first, then fetches by after/limit.
func (s *ChatService) GetMessages(uid int, link string, after, before, limit int) ([]model.ChatMessage, error) {
	room, err := s.resolveRoomForMember(uid, link)
	if err != nil {
		return nil, err
	}
	return s.repo.ListMessages(room.ID, after, before, limit)
}

// LatestMessagePerRoom passthrough for room-list last-message previews.
func (s *ChatService) LatestMessagePerRoom(roomIDs []int) (map[int]model.ChatMessage, error) {
	return s.repo.LatestMessagePerRoom(roomIDs)
}

// CreateMessage sends a message and updates the room's last_message_time.
func (s *ChatService) CreateMessage(uid int, link string, content, fileURL string, replyToID *int) (*model.ChatMessage, error) {
	room, err := s.resolveRoomForMember(uid, link)
	if err != nil {
		return nil, err
	}
	if content == "" && fileURL == "" {
		return nil, fmt.Errorf("消息内容不能为空")
	}
	msg := &model.ChatMessage{
		ChatRoomID: room.ID,
		SenderID:   uid,
		Content:    content,
		FileURL:    fileURL,
		ReplyToID:  replyToID,
		Status:     "SENT",
	}
	if err := s.repo.CreateMessage(msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// UpdateMessage edits a message. Only the sender may edit.
func (s *ChatService) UpdateMessage(uid, messageID int, newContent string) error {
	m, err := s.repo.GetMessage(messageID)
	if err != nil {
		return fmt.Errorf("消息不存在")
	}
	if m.SenderID != uid {
		return fmt.Errorf("仅发送者可以编辑消息")
	}
	if m.Status == "DELETED" {
		return fmt.Errorf("已删除的消息无法编辑")
	}
	return s.repo.UpdateMessageContent(m, m.Content, newContent)
}

// DeleteMessage soft-deletes a message. Sender or moderator/admin may delete.
func (s *ChatService) DeleteMessage(uid int, isPrivileged bool, messageID int) error {
	m, err := s.repo.GetMessage(messageID)
	if err != nil {
		return fmt.Errorf("消息不存在")
	}
	if m.SenderID != uid && !isPrivileged {
		return fmt.Errorf("仅发送者或管理员可删除消息")
	}
	now := time.Now()
	return s.repo.SoftDeleteMessage(messageID, uid, now)
}

// ToggleReaction toggles an emoji reaction.
func (s *ChatService) ToggleReaction(uid, messageID int, emoji string) (bool, error) {
	if _, err := s.repo.GetMessage(messageID); err != nil {
		return false, fmt.Errorf("消息不存在")
	}
	return s.repo.ToggleReaction(messageID, uid, emoji)
}

// MarkSeen marks messages as seen in bulk.
func (s *ChatService) MarkSeen(uid int, link string, messageIDs []int) error {
	room, err := s.resolveRoomForMember(uid, link)
	if err != nil {
		return err
	}
	return s.repo.MarkSeen(room.ID, uid, messageIDs)
}

// ─── helpers ────────────────────────────────────────

// resolveRoomForMember fetches the room and confirms uid is a member, else returns an error.
func (s *ChatService) resolveRoomForMember(uid int, link string) (*model.ChatRoom, error) {
	room, err := s.repo.FindRoomByLink(link)
	if err != nil {
		return nil, fmt.Errorf("房间不存在")
	}
	ok, err := s.repo.IsMember(uid, room.ID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("您不是该房间的成员")
	}
	return room, nil
}

// ReactionsByMessageIDs / MessagesByIDs are thin passthroughs the handler
// uses to enrich a message page with reactions + reply quotes in two extra
// batched queries (no N+1).
func (s *ChatService) ReactionsByMessageIDs(ids []int) ([]model.ChatMessageReaction, error) {
	return s.repo.ListReactionsByMessageIDs(ids)
}

func (s *ChatService) MessagesByIDs(ids []int) ([]model.ChatMessage, error) {
	return s.repo.GetMessagesByIDs(ids)
}

// IsNotFound exposes an ErrRecordNotFound check for callers to distinguish business errors.
func IsNotFound(err error) bool { return err == gorm.ErrRecordNotFound }
