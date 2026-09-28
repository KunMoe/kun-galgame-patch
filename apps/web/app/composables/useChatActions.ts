import type {
  KunChatFormattedText,
  KunChatReaction,
  KunChatReplyQuote
} from '@kungal/ui-vue'
import type { WithUsers } from '~/stores/chatStore'

const TYPING_EVERY = 5000
const typingSentAt: Record<string, number> = {}

export interface ChatSendOptions {
  replyTo?: ChatMessage | null
  quote?: KunChatReplyQuote | null
  photo?: {
    image_hash: string
    width: number
    height: number
    thumbhash?: string | null
    url?: string | null
  }
  mediaGroupId?: string
}

export const useChatActions = () => {
  const api = useApi()
  const store = useChatStore()
  const model = store.model
  const post = async (id: string, pending: ChatMessage) => {
    const body: Record<string, unknown> = {
      client_message_id: pending.client_message_id,
      text: pending.text,
      entities: pending.entities
    }
    if (pending.reply_to) body.reply_to_seq = pending.reply_to.seq
    if (pending.reply_quote)
      body.reply_quote = {
        text: pending.reply_quote.text,
        offset: pending.reply_quote.offset
      }
    if (pending.media)
      body.media = { type: 'photo', image_hash: pending.media.image_hash }
    if (pending.media_group_id) body.media_group_id = pending.media_group_id
    const r = await api.post<WithUsers<ChatMessage>>(
      `/im/conversations/${id}/messages`,
      body
    )
    const w = model.windows[id]
    if (r.code !== 0) {
      const at =
        w?.items.findIndex(
          (m) => m.client_message_id === pending.client_message_id
        ) ?? -1
      if (w && at >= 0) w.items[at] = { ...w.items[at]!, status: 'failed' }
      store.failed(r.code)
      return r
    }
    const { users: u, ...msg } = r.data
    mergeChatUsers(model, u)
    if (w) upsertChatMessage(w, msg)
    const c = model.conversations[id]
    if (c && (!c.last_message || msg.seq >= c.last_message.seq)) {
      c.last_message = msg
      c.last_seq = Math.max(c.last_seq, msg.seq)
    }
    return r
  }

  const send = (
    id: string,
    text: KunChatFormattedText,
    o: ChatSendOptions = {}
  ) => {
    const w = model.windows[id]
    const replyTo = o.replyTo
    const pending: ChatMessage = {
      id: '',
      conversation_id: id,
      seq: 0,
      sender_id: model.me,
      kind: 'message',
      text: text.text,
      entities: text.entities,
      media: o.photo ? { type: 'photo', ...o.photo } : null,
      media_group_id: o.mediaGroupId ?? null,
      reply_to: replyTo
        ? {
            seq: replyTo.seq,
            sender_id: replyTo.sender_id,
            text: replyTo.text,
            entities: replyTo.entities,
            media_type: replyTo.media?.type ?? null,
            deleted: false
          }
        : null,
      reply_quote: o.quote ?? null,
      service_action: null,
      context: null,
      reactions: [],
      silent: false,
      pinned_at: null,
      edited_at: null,
      created_at: new Date().toISOString(),
      client_message_id: crypto.randomUUID(),
      status: 'sending'
    }
    if (w) w.items.push(pending)
    return post(id, pending)
  }

  const retry = (message: ChatMessage) => {
    const w = model.windows[message.conversation_id]
    const at = w?.items.indexOf(message) ?? -1
    if (!w || at < 0) return
    w.items[at] = { ...message, status: 'sending' }
    return post(message.conversation_id, w.items[at]!)
  }

  const discard = (message: ChatMessage) => {
    const w = model.windows[message.conversation_id]
    if (w) w.items = w.items.filter((m) => m !== message)
  }

  const replaceMessage = (msg: ChatMessage) => {
    const w = model.windows[msg.conversation_id]
    const at =
      w?.items.findIndex(
        (m) =>
          m.seq === msg.seq && m.status !== 'sending' && m.status !== 'failed'
      ) ?? -1
    if (w && at >= 0)
      w.items[at] = {
        ...msg,
        client_message_id: w.items[at]!.client_message_id
      }
  }

  const edit = async (message: ChatMessage, text: KunChatFormattedText) => {
    const r = await api.patch<WithUsers<ChatMessage>>(
      `/im/messages/${message.id}`,
      { ...text }
    )
    if (r.code === 0) {
      const { users: u, ...msg } = r.data
      mergeChatUsers(model, u)
      replaceMessage(msg)
    }
    return r
  }

  const remove = async (id: string, seqs: number[], forEveryone: boolean) => {
    const r = await api.post<{ seqs: number[] }>(
      `/im/conversations/${id}/messages/delete`,
      {
        seqs,
        for_everyone: forEveryone
      }
    )
    if (r.code === 0) {
      const gone = new Set(r.data.seqs)
      const w = model.windows[id]
      if (w) w.items = w.items.filter((m) => m.seq === 0 || !gone.has(m.seq))
    }
    return r
  }

  const react = async (message: ChatMessage, reaction: string | null) => {
    const r = await api.put<{ reactions: KunChatReaction[] }>(
      `/im/messages/${message.id}/reaction`,
      { reaction }
    )
    if (r.code === 0)
      replaceMessage({ ...message, reactions: r.data.reactions })
    return r
  }

  const pin = (id: string, target: number, pinned: boolean) =>
    pinned
      ? api.put(`/im/conversations/${id}/pins/${target}`)
      : api.delete(`/im/conversations/${id}/pins/${target}`)

  const read = async (id: string, maxSeq: number) => {
    const c = model.conversations[id]
    if (!c || (maxSeq <= c.me.last_read_seq && !c.me.marked_unread)) return
    const r = await api.post<{ last_read_seq: number; unread_count: number }>(
      `/im/conversations/${id}/read`,
      {
        max_seq: maxSeq
      }
    )
    if (r.code === 0) {
      c.me.last_read_seq = Math.max(c.me.last_read_seq, r.data.last_read_seq)
      c.me.unread_count = r.data.unread_count
      c.me.marked_unread = false
      store.refreshState()
    }
  }

  const accept = async (id: string) => {
    const r = await api.post(`/im/conversations/${id}/accept`)
    if (r.code === 0) {
      await store.refetch(id)
      store.refreshState()
    }
    return r
  }

  const setDialog = async (id: string, patch: Record<string, unknown>) => {
    const r = await api.patch<ChatDialog>(`/im/conversations/${id}/me`, patch)
    const c = model.conversations[id]
    if (r.code === 0 && c) Object.assign(c.me, r.data)
    return r
  }

  const clearHistory = async (id: string, remove: boolean) => {
    const r = await api.post(`/im/conversations/${id}/clear-history`, {
      remove
    })
    if (r.code === 0) store.refreshState()
    return r
  }

  const report = (message: ChatMessage, reason: string, note?: string) =>
    api.post(`/im/messages/${message.id}/report`, {
      reason,
      note: note || undefined
    })

  const saveDraft = (
    id: string,
    text: KunChatFormattedText,
    replyToSeq: number | null
  ) =>
    api.put(`/im/conversations/${id}/draft`, {
      ...text,
      reply_to_seq: replyToSeq
    })

  const sendTyping = (id: string) => {
    const now = Date.now()
    if (now - (typingSentAt[id] ?? 0) < TYPING_EVERY) return
    typingSentAt[id] = now
    void api.post(`/im/conversations/${id}/typing`)
  }

  const openDirect = async (userId: number | string) => {
    const r = await api.put<WithUsers<ChatConversation>>(`/im/direct/${userId}`)
    if (r.code !== 0) {
      store.failed(r.code)
      return r
    }
    store.putConversation(r.data)
    return r
  }

  const uploadPhoto = (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return api.post<{
      image_hash: string
      width: number
      height: number
      thumbhash?: string
      url: string
    }>('/im/images', form)
  }

  return {
    send,
    retry,
    discard,
    edit,
    remove,
    react,
    pin,
    read,
    accept,
    setDialog,
    clearHistory,
    report,
    saveDraft,
    sendTyping,
    openDirect,
    uploadPhoto
  }
}
