// Chat message status is a string enum, not a numeric flag.
type ChatMessageStatus = 'SENT' | 'EDITED' | 'DELETED'

interface ChatRoomSummary {
  id: number
  link: string
  type: 'PRIVATE' | 'GROUP' | string
  // For PRIVATE rooms the backend overrides name/avatar with the peer's
  // identity (the room row itself has none).
  name: string
  avatar: string
  // One-line preview of the most recent message ("[贴纸]"/"[图片]"/"[消息已撤回]"
  // for non-text). Present on the room-list endpoint; absent on room detail.
  last_message?: string
  last_message_time?: string | Date | null
  created: string | Date
  updated: string | Date
}

// GET /api/v1/chat/room/:link returns a RoomDetail, the inline ChatRoom plus `member`.
interface ChatRoomMember {
  id: number
  role: 'OWNER' | 'ADMIN' | 'MEMBER' | string
  user_id: number
  chat_room_id: number
  created: string | Date
  updated: string | Date
  user: KunUser
}

interface ChatRoomDetail extends ChatRoomSummary {
  member: ChatRoomMember[]
}

interface ChatMessageReactionItem {
  id: number
  emoji: string
  user: KunUser | null
}

interface ChatQuoteMessage {
  id: number
  sender_name: string
  content: string // rendered HTML, or "该消息已删除"
}

interface ChatMessageItem {
  id: number
  chat_room_id: number
  sender_id: number
  // `content` is raw markdown (the edit modal round-trips it); `content_html`
  // is the backend-rendered + sanitized HTML used for display.
  content: string
  content_html: string
  file_url: string
  status: ChatMessageStatus | string
  deleted_at: string | Date | null
  deleted_by_id: number | null
  reply_to_id: number | null
  created: string | Date
  updated: string | Date
  sender: KunUser
  reaction: ChatMessageReactionItem[]
  quote_message?: ChatQuoteMessage | null
}
