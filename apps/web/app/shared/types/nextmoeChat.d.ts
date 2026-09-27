type ChatMessage = import('@kungal/ui-vue').KunChatMessage
type ChatUser = import('@kungal/ui-vue').KunChatUser

interface ChatDraft {
  text: string
  entities: import('@kungal/ui-vue').KunChatEntity[]
  reply_to_seq: number | null
  updated_at: string
}

interface ChatDialog {
  role: 'owner' | 'admin' | 'member'
  accepted: boolean
  last_read_seq: number
  unread_count: number
  marked_unread: boolean
  cleared_through_seq: number
  visible_from_seq: number
  muted_until: string | null
  archived: boolean
  pinned_rank: number | null
  draft: ChatDraft | null
}

interface ChatConversation {
  object: 'conversation'
  id: string
  kind: 'direct' | 'group'
  title: string | null
  about: string | null
  photo_image_hash: string | null
  peer_id: string | null
  member_count: number
  last_seq: number
  peer_read_seq: number
  created_at: string
  me: ChatDialog
  last_message: ChatMessage | null
  pinned_seqs?: number[]
  pinned_messages?: ChatMessage[]
}

type ChatFolder = 'inbox' | 'archive' | 'requests'

interface ChatConversationPage {
  items: ChatConversation[]
  users: ChatUser[]
  next_cursor?: string
}

interface ChatMessagePage {
  items: ChatMessage[]
  users: ChatUser[]
  has_more_before: boolean
  has_more_after: boolean
}

interface ChatState {
  last_update_seq: number
  unread_conversation_count: number
  unread_message_count: number
  request_count: number
}

type ChatUpdateKind =
  | 'new_message'
  | 'edit_message'
  | 'delete_messages'
  | 'read_inbox'
  | 'read_outbox'
  | 'message_reactions'
  | 'pinned_messages'
  | 'dialog'
  | 'hide_messages'
  | 'clear_history'
  | 'member'
  | 'conversation'

interface ChatUpdate {
  update_seq: number
  kind: ChatUpdateKind
  conversation_id: string
  data: Record<string, unknown>
  created_at: string
}

interface ChatUpdatesPage {
  updates: ChatUpdate[]
  messages: ChatMessage[]
  conversations: ChatConversation[]
  users: ChatUser[]
  last_update_seq: number
  has_more: boolean
  too_long: boolean
}

interface ChatSettings {
  allow_incoming: 'all' | 'following' | 'none'
  accept_requests: boolean
  allow_group_invites: 'all' | 'following' | 'none'
}

interface ChatRealtimeToken {
  token: string
  expires_at: string
  url: string
}

type ChatPush =
  | {
      type: 'update'
      update: ChatUpdate
      message?: ChatMessage
      reactions?: import('@kungal/ui-vue').KunChatReaction[]
    }
  | { type: 'typing'; conversation_id: string; user_id: string }
