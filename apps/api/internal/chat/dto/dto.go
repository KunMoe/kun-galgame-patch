package dto

// ─── Room ───────────────────────────────────────────

// CreateRoomRequest creates a group chat room.
type CreateRoomRequest struct {
	Name   string `json:"name" validate:"required,min=1,max=107"`
	Avatar string `json:"avatar" validate:"max=1007"`
}

// JoinRoomRequest joins a group chat via its link.
type JoinRoomRequest struct {
	Link string `json:"link" validate:"required,min=1,max=17"`
}

// StartPrivateChatRequest is the body of POST /api/v1/chat/room/private.
// peer_uid is the OTHER user the caller wants to chat with; the server
// resolves "current user + peer" to a private room (creating it on first
// call). The returned room's link follows the format "<minUID>-<maxUID>"
// so both directions converge to the same row.
type StartPrivateChatRequest struct {
	PeerUID int `json:"peer_uid" validate:"required,min=1"`
}

// ─── Messages ───────────────────────────────────────

// ListMessagesQuery drives the message-fetch modes:
//
//   - ids="1,2,3" → refresh exactly those messages (in-place patch after an
//                   edit/delete/reaction — never scrolls, never re-pages)
//   - before > 0  → older page: messages with id < before (scroll-up history)
//   - after  > 0  → new messages: id > after (the 5s forward poll)
//   - neither     → latest page: the most recent `limit` messages
//                   (initial load)
//
// `ids` wins over before/after; before wins over after.
type ListMessagesQuery struct {
	IDs    string `query:"ids" validate:"omitempty,max=2000"`
	After  int    `query:"after" validate:"min=0"`
	Before int    `query:"before" validate:"min=0"`
	Limit  int    `query:"limit" validate:"min=1,max=100"`
}

// CreateMessageRequest sends a message. FileURL is optional (attachment).
type CreateMessageRequest struct {
	Content   string `json:"content" validate:"max=2000"`
	FileURL   string `json:"file_url" validate:"max=1007"`
	ReplyToID *int   `json:"reply_to_id" validate:"omitempty,min=1"`
}

// UpdateMessageRequest edits a message.
type UpdateMessageRequest struct {
	Content string `json:"content" validate:"required,min=1,max=2000"`
}

// ReactionRequest toggles an emoji reaction.
type ReactionRequest struct {
	Emoji string `json:"emoji" validate:"required,min=1,max=10"`
}

// SeenRequest batches seen-markers. Pass message ids.
type SeenRequest struct {
	MessageIDs []int `json:"message_ids" validate:"required,min=1,max=200,dive,min=1"`
}
