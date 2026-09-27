import { defineStore } from 'pinia'
import type { KunChatReactionOption, KunChatTypingEvent } from '@kungal/ui-vue'

export type WithUsers<T> = T & { users?: ChatUser[] }

const PAGE = 50
const GAP_WAIT = 500

const COUNTED: ReadonlySet<ChatUpdateKind> = new Set([
  'new_message',
  'read_inbox',
  'dialog',
  'hide_messages',
  'clear_history',
  'delete_messages'
])

export const useChatStore = defineStore('chat', () => {
  const api = useApi()
  const model = reactive<ChatModel>({
    me: '',
    conversations: {},
    windows: {},
    users: {}
  })
  const state = ref<ChatState | null>(null)
  const status = ref<'idle' | 'ready' | 'scope' | 'down'>('idle')
  const folders = reactive<
    Record<ChatFolder, { next?: string; loaded: boolean; loading: boolean }>
  >({
    inbox: { loaded: false, loading: false },
    archive: { loaded: false, loading: false },
    requests: { loaded: false, loading: false }
  })
  const reactions = ref<KunChatReactionOption[]>([])
  const typing = reactive<Record<string, KunChatTypingEvent[]>>({})
  const openId = ref<string | null>(null)

  let seq = 0
  let syncing: Promise<void> | null = null
  let again = false
  let gapTimer: ReturnType<typeof setTimeout> | null = null
  let stateTimer: ReturnType<typeof setTimeout> | null = null

  const users = computed(() => Object.values(model.users))
  const hasUnread = computed(
    () =>
      !!state.value &&
      (state.value.unread_conversation_count > 0 ||
        state.value.request_count > 0)
  )

  const list = (folder: ChatFolder) =>
    Object.values(model.conversations)
      .filter(
        (c) =>
          chatFolderOf(c) === folder &&
          (c.last_message || c.id === openId.value)
      )
      .sort(compareChatConversations)

  const failed = (code: number) => {
    if (code === 40313) status.value = 'scope'
  }

  const putConversation = (c: WithUsers<ChatConversation>) => {
    mergeChatUsers(model, c.users)
    const { users: _users, ...rest } = c
    model.conversations[c.id] = rest as ChatConversation
  }

  const keepConversation = (c: ChatConversation) => {
    const prev = model.conversations[c.id]
    model.conversations[c.id] = c.pinned_messages
      ? c
      : {
          ...c,
          pinned_seqs: prev?.pinned_seqs,
          pinned_messages: prev?.pinned_messages
        }
  }

  const refreshState = () => {
    if (stateTimer) return
    stateTimer = setTimeout(async () => {
      stateTimer = null
      const r = await api.get<ChatState>('/im/state')
      if (r.code === 0) state.value = r.data
    }, 800)
  }

  const refetch = async (id: string) => {
    const r = await api.get<WithUsers<ChatConversation>>(
      `/im/conversations/${id}`
    )
    if (r.code === 0) putConversation(r.data)
    else if (r.code === 40400) {
      Reflect.deleteProperty(model.conversations, id)
      Reflect.deleteProperty(model.windows, id)
    }
  }

  const apply = (u: ChatUpdate, x: ChatUpdateExtras, known?: Set<string>) => {
    const outcome = applyChatUpdate(model, u, x)
    if (
      outcome === 'refetch' &&
      (u.kind === 'pinned_messages' || !known?.has(u.conversation_id))
    )
      void refetch(u.conversation_id)
    if (COUNTED.has(u.kind)) refreshState()
  }

  const loadFolder = async (folder: ChatFolder, more = false) => {
    const f = folders[folder]
    if (f.loading || (more && !f.next) || (!more && f.loaded)) return
    f.loading = true
    const q = new URLSearchParams({ folder, limit: '30' })
    if (more && f.next) q.set('cursor', f.next)
    const r = await api.get<ChatConversationPage>(`/im/conversations?${q}`)
    f.loading = false
    if (r.code !== 0) return failed(r.code)
    mergeChatUsers(model, r.data.users)
    for (const c of r.data.items) keepConversation(c)
    f.next = r.data.next_cursor || undefined
    f.loaded = true
  }

  const reset = async () => {
    const r = await api.get<ChatState>('/im/state')
    if (r.code !== 0) return failed(r.code)
    state.value = r.data
    seq = r.data.last_update_seq
    model.conversations = {}
    model.windows = {}
    for (const f of Object.values(folders))
      Object.assign(f, { loaded: false, next: undefined })
    await loadFolder('inbox')
    if (openId.value) await open(openId.value)
  }

  const sync = (): Promise<void> => {
    if (syncing) {
      again = true
      return syncing
    }
    syncing = (async () => {
      do {
        again = false
        for (;;) {
          const r = await api.get<ChatUpdatesPage>(
            `/im/updates?after=${seq}&limit=100`
          )
          if (r.code !== 0) return failed(r.code)
          if (r.data.too_long) {
            await reset()
            break
          }
          mergeChatUsers(model, r.data.users)
          const byKey = new Map(
            r.data.messages.map((m) => [`${m.conversation_id}:${m.seq}`, m])
          )
          const known = new Set(r.data.conversations.map((c) => c.id))
          for (const u of r.data.updates) {
            if (u.update_seq <= seq) continue
            apply(
              u,
              { message: byKey.get(`${u.conversation_id}:${u.data.seq}`) },
              known
            )
            seq = u.update_seq
          }
          for (const c of r.data.conversations) {
            if (model.conversations[c.id] || c.last_message) keepConversation(c)
          }
          if (!r.data.has_more) break
        }
      } while (again)
    })().finally(() => {
      syncing = null
    })
    return syncing
  }

  const receive = (u: ChatUpdate, x: ChatUpdateExtras) => {
    if (u.update_seq <= seq) return
    if (syncing || u.update_seq > seq + 1) {
      if (syncing) again = true
      else if (!gapTimer) {
        gapTimer = setTimeout(() => {
          gapTimer = null
          void sync()
        }, GAP_WAIT)
      }
      return
    }
    apply(u, x)
    seq = u.update_seq
  }

  const push = (p: ChatPush) => {
    if (p.type === 'typing') {
      const now = Date.now()
      typing[p.conversation_id] = [
        ...(typing[p.conversation_id] ?? []).filter(
          (t) => t.user_id !== p.user_id && now - t.at < 6000
        ),
        { user_id: p.user_id, at: now }
      ]
      return
    }
    if (p.update.kind === 'new_message' && p.message) {
      typing[p.update.conversation_id] = (
        typing[p.update.conversation_id] ?? []
      ).filter((t) => t.user_id !== p.message!.sender_id)
    }
    receive(p.update, { message: p.message, reactions: p.reactions })
  }

  const start = async (me: string) => {
    if (model.me === me && status.value === 'ready') return
    model.me = me
    const r = await api.get<ChatState>('/im/state')
    if (r.code !== 0) {
      status.value = r.code === 40313 ? 'scope' : 'down'
      return
    }
    state.value = r.data
    seq = r.data.last_update_seq
    status.value = 'ready'
    await loadFolder('inbox')
    const v = await api.get<{ items: KunChatReactionOption[] }>('/im/reactions')
    if (v.code === 0) reactions.value = v.data.items
  }

  const stop = () => {
    model.me = ''
    model.conversations = {}
    model.windows = {}
    state.value = null
    status.value = 'idle'
    seq = 0
    for (const f of Object.values(folders))
      Object.assign(f, { loaded: false, next: undefined })
  }

  const page = async (id: string, params: Record<string, number>) => {
    const q = new URLSearchParams({ limit: String(PAGE) })
    for (const [k, v] of Object.entries(params)) q.set(k, String(v))
    const r = await api.get<ChatMessagePage>(
      `/im/conversations/${id}/messages?${q}`
    )
    if (r.code !== 0) {
      failed(r.code)
      return null
    }
    mergeChatUsers(model, r.data.users)
    return r.data
  }

  const setWindow = (id: string, p: ChatMessagePage) => {
    const pending =
      model.windows[id]?.items.filter(
        (m) => m.status === 'sending' || m.status === 'failed'
      ) ?? []
    model.windows[id] = {
      items: [...p.items, ...pending],
      hasOlder: p.has_more_before,
      hasNewer: p.has_more_after
    }
  }

  const open = async (id: string) => {
    openId.value = id
    if (!model.conversations[id]) await refetch(id)
    else void refetch(id)
    const c = model.conversations[id]
    if (!c || model.windows[id]) return
    const p = await page(
      id,
      c.me.unread_count > 0 ? { around_seq: c.me.last_read_seq + 1 } : {}
    )
    if (p) setWindow(id, p)
  }

  const loadOlder = async (id: string) => {
    const w = model.windows[id]
    const first = w?.items.find((m) => m.seq > 0)
    if (!w || !first) return
    const p = await page(id, { before_seq: first.seq })
    if (!p) return
    w.items = [...p.items, ...w.items]
    w.hasOlder = p.has_more_before
  }

  const loadNewer = async (id: string) => {
    const w = model.windows[id]
    const last = w?.items.filter((m) => m.seq > 0).at(-1)
    if (!w || !last) return
    const p = await page(id, { after_seq: last.seq })
    if (!p) return
    for (const m of p.items) upsertChatMessage({ ...w, hasNewer: false }, m)
    w.items = [...w.items]
    w.hasNewer = p.has_more_after
  }

  const jump = async (id: string, target: number) => {
    const p = await page(id, { around_seq: target })
    if (p) setWindow(id, p)
  }

  const latest = async (id: string) => {
    const p = await page(id, {})
    if (p) setWindow(id, p)
  }

  return {
    model,
    failed,
    refreshState,
    refetch,
    putConversation,
    state,
    hasUnread,
    status,
    folders,
    reactions,
    typing,
    openId,
    users,
    list,
    start,
    stop,
    sync,
    push,
    loadFolder,
    open,
    loadOlder,
    loadNewer,
    jump,
    latest
  }
})
