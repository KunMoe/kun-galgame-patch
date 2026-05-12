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

// ListMessagesQuery polls for new messages.
type ListMessagesQuery struct {
	After int `query:"after" validate:"min=0"`        // max message id from the previous poll, 0 = first request
	Limit int `query:"limit" validate:"min=1,max=100"` // maximum messages per request
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
