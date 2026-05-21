<script setup lang="ts">
import { useIntervalFn } from '@vueuse/core'
import DOMPurify from 'isomorphic-dompurify'

// Chat room. Feature-parity port of the next-web chat window MINUS realtime
// (D9: REST-only). New messages arrive via a 5s incremental poll; edits /
// deletes / reactions are reflected by reloading the recent window after the
// local action (no socket push). Supported: markdown rendering, right-click
// context menu, reply (banner + quote + jump), edit modal, delete tombstone,
// emoji reactions, emoji/sticker picker.

useKunSeoMeta({ title: '聊天', description: '聊天会话' })

const route = useRoute()
const api = useApi()
const userStore = useUserStore()

const link = computed(() => String(route.params.link))

const { data: room } = await useAsyncData<ChatRoomDetail | null>(
  () => `chat-room-${link.value}`,
  async () => {
    const res = await api.get<ChatRoomDetail>(`/chat/room/${link.value}`)
    return res.code === 0 ? res.data : null
  }
)

const isGroup = computed(() => room.value?.type !== 'PRIVATE')
const myUid = computed(() => userStore.user.id)

const messages = ref<ChatMessageItem[]>([])
const input = ref('')
const sending = ref(false)
const loading = ref(false)
const scrollArea = ref<HTMLElement | null>(null)
const inputEl = ref<HTMLTextAreaElement | null>(null)

const sanitize = (html: string) =>
  DOMPurify.sanitize(html || '', { ADD_ATTR: ['data-uid', 'target'] })

// Pagination model (REST-only, no realtime):
//   - loadLatest(): the most recent page, used on entry + after any in-place
//     mutation (edit/delete/reaction). Jumps to the bottom.
//   - loadOlder(): scroll-up history; prepends older messages, preserving the
//     visual scroll position.
//   - poll: `after=<lastId>` appends genuinely new messages every 5s.
const PAGE = 50
const loadingOlder = ref(false)
const hasMoreOlder = ref(true)

const lastId = computed(() =>
  messages.value.length ? messages.value[messages.value.length - 1]!.id : 0
)
const firstId = computed(() =>
  messages.value.length ? messages.value[0]!.id : 0
)

const scrollToBottom = async () => {
  await nextTick()
  const pin = () => {
    const el = scrollArea.value
    if (el) el.scrollTop = el.scrollHeight
  }
  pin()
  // Avatars / stickers / markdown images finish loading AFTER the first
  // layout and grow the transcript, leaving the view stuck near the top.
  // Re-pin across a few frames so the initial load actually lands on the
  // newest message. Cheap and bounded — no permanent listeners.
  requestAnimationFrame(pin)
  setTimeout(pin, 120)
  setTimeout(pin, 360)
}

const atBottom = () =>
  scrollArea.value
    ? scrollArea.value.scrollHeight -
        scrollArea.value.scrollTop -
        scrollArea.value.clientHeight <
      120
    : true

// Latest page → entry point + post-mutation refresh. Always lands at bottom.
const loadLatest = async (silent = false) => {
  if (!room.value) return
  if (!silent) loading.value = true
  let ok = false
  try {
    const res = await api.get<ChatMessageItem[]>(
      `/chat/room/${link.value}/message?limit=${PAGE}`
    )
    if (res.code === 0) {
      messages.value = res.data ?? []
      hasMoreOlder.value = (res.data?.length ?? 0) >= PAGE
      ok = true
    }
  } finally {
    if (!silent) loading.value = false
  }
  // Scroll only AFTER `loading` is cleared: while it's true the template
  // shows <KunLoading> and the message list isn't in the DOM yet, so a
  // scroll here would target the placeholder and leave us at the top.
  if (ok) await scrollToBottom()
}

// Older page (scroll-up). Prepends and keeps the viewport anchored on the
// message the user was looking at.
const loadOlder = async () => {
  if (
    !room.value ||
    loadingOlder.value ||
    !hasMoreOlder.value ||
    !messages.value.length
  )
    return
  loadingOlder.value = true
  const el = scrollArea.value
  const prevHeight = el?.scrollHeight ?? 0
  const prevTop = el?.scrollTop ?? 0
  try {
    const res = await api.get<ChatMessageItem[]>(
      `/chat/room/${link.value}/message?before=${firstId.value}&limit=${PAGE}`
    )
    if (res.code === 0) {
      const older = res.data ?? []
      hasMoreOlder.value = older.length >= PAGE
      if (older.length) {
        const seen = new Set(messages.value.map((m) => m.id))
        const fresh = older.filter((m) => !seen.has(m.id))
        messages.value = [...fresh, ...messages.value]
        await nextTick()
        if (el) el.scrollTop = el.scrollHeight - prevHeight + prevTop
      }
    }
  } finally {
    loadingOlder.value = false
  }
}

// Forward poll: only genuinely new messages (id > lastId). No-op until the
// first page is loaded so it never collides with loadLatest.
const pollNew = async () => {
  if (!room.value || !messages.value.length) return
  const res = await api.get<ChatMessageItem[]>(
    `/chat/room/${link.value}/message?after=${lastId.value}&limit=${PAGE}`
  )
  if (res.code !== 0) return
  const incoming = res.data ?? []
  if (!incoming.length) return
  const wasAtBottom = atBottom()
  const seen = new Set(messages.value.map((m) => m.id))
  const fresh = incoming.filter((m) => !seen.has(m.id))
  if (!fresh.length) return
  messages.value = [...messages.value, ...fresh]
  if (wasAtBottom) await scrollToBottom()
}

const onScroll = () => {
  if (scrollArea.value && scrollArea.value.scrollTop < 60) loadOlder()
}

// In-place refresh after an edit / delete / reaction. Re-fetches ONLY the
// currently-loaded message ids and patches them back into the same slots —
// no re-paging, no array reorder, and crucially no scroll, so the view stays
// exactly where the user was instead of jumping to the newest message.
const refreshLoaded = async () => {
  if (!room.value || !messages.value.length) return
  const ids = messages.value.map((m) => m.id)
  const res = await api.get<ChatMessageItem[]>(
    `/chat/room/${link.value}/message?ids=${ids.join(',')}&limit=100`
  )
  if (res.code !== 0) return
  const byId = new Map((res.data ?? []).map((m) => [m.id, m]))
  messages.value = messages.value.map((m) => byId.get(m.id) ?? m)
}

// ─── Reply ────────────────────────────────────────────
const replyTo = ref<ChatMessageItem | null>(null)
const startReply = (m: ChatMessageItem) => {
  replyTo.value = m
  inputEl.value?.focus()
}
const cancelReply = () => (replyTo.value = null)

// Quotes whose target message isn't in the loaded window: clicking them
// can't scroll anywhere, so instead expand the quote in-place to show the
// full quoted content (it's already fully present in quote_message.content,
// the template just visually clamps it). Keyed by the *replying* message id.
const expandedQuotes = reactive(new Set<number>())

// Click handler for a reply quote. If the referenced message is loaded,
// scroll+highlight it; otherwise expand this quote to reveal it in full.
const onQuoteClick = (m: ChatMessageItem) => {
  if (!m.reply_to_id) return
  const el = document.getElementById(`chat-msg-${m.reply_to_id}`)
  if (el) {
    el.scrollIntoView({ behavior: 'smooth', block: 'center' })
    el.classList.add('ring-2', 'ring-secondary')
    setTimeout(() => el.classList.remove('ring-2', 'ring-secondary'), 1500)
    return
  }
  // Not loaded → toggle full quoted content inline.
  if (expandedQuotes.has(m.id)) expandedQuotes.delete(m.id)
  else expandedQuotes.add(m.id)
}

// ─── Send ─────────────────────────────────────────────
const postMessage = async (content: string) => {
  if (!content.trim() || !room.value) return
  sending.value = true
  try {
    const body: Record<string, unknown> = { content }
    if (replyTo.value) body.reply_to_id = replyTo.value.id
    const res = await api.post<ChatMessageItem>(
      `/chat/room/${link.value}/message`,
      body
    )
    if (res.code === 0) {
      input.value = ''
      replyTo.value = null
      // Append the just-sent (already enriched) message and always jump to it.
      if (res.data) {
        const seen = new Set(messages.value.map((m) => m.id))
        if (!seen.has(res.data.id)) {
          messages.value = [...messages.value, res.data]
        }
        await scrollToBottom()
      } else {
        await loadLatest(true)
      }
    } else {
      useKunMessage(res.message || '发送失败', 'error')
    }
  } finally {
    sending.value = false
  }
}
const sendMessage = () => postMessage(input.value.trim())

const onStickerSend = (url: string) => {
  // Send as a markdown image so it renders via the same pipeline.
  postMessage(`![sticker](${url})`)
}
const onEmojiSelect = (emoji: string) => {
  const ta = inputEl.value
  if (!ta) {
    input.value += emoji
    return
  }
  const s = ta.selectionStart ?? input.value.length
  const e = ta.selectionEnd ?? input.value.length
  input.value = input.value.slice(0, s) + emoji + input.value.slice(e)
  nextTick(() => {
    ta.focus()
    ta.selectionStart = ta.selectionEnd = s + emoji.length
  })
}

const handleKeydown = (e: KeyboardEvent) => {
  if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
    e.preventDefault()
    sendMessage()
  }
}

// ─── Context menu ─────────────────────────────────────
const menuOpen = ref(false)
const menuAnchor = ref({ x: 0, y: 0 })
const menuTarget = ref<ChatMessageItem | null>(null)
const isMobile = ref(false)
onMounted(() => {
  isMobile.value = window.matchMedia('(max-width: 768px)').matches
})

const openMenu = (e: MouseEvent, m: ChatMessageItem) => {
  if (m.status === 'DELETED') return
  e.preventDefault()
  menuAnchor.value = { x: e.clientX, y: e.clientY }
  menuTarget.value = m
  menuOpen.value = true
}
const onBubbleClick = (e: MouseEvent, m: ChatMessageItem) => {
  if (isMobile.value) openMenu(e, m)
}

const toggleReaction = async (m: ChatMessageItem, emoji: string) => {
  const res = await api.post<{ added: boolean }>(
    `/chat/message/${m.id}/reaction`,
    { emoji }
  )
  if (res.code === 0) await refreshLoaded()
  else useKunMessage(res.message || '操作失败', 'error')
}

// ─── Edit ─────────────────────────────────────────────
const editOpen = ref(false)
const editTarget = ref<ChatMessageItem | null>(null)
const openEdit = (m: ChatMessageItem) => {
  editTarget.value = m
  editOpen.value = true
}
const saveEdit = async (content: string) => {
  if (!editTarget.value) return
  const res = await api.put(`/chat/message/${editTarget.value.id}`, { content })
  if (res.code === 0) {
    editOpen.value = false
    await refreshLoaded()
  } else {
    useKunMessage(res.message || '编辑失败', 'error')
  }
}

// ─── Delete ───────────────────────────────────────────
const deleteOpen = ref(false)
const deleteTarget = ref<ChatMessageItem | null>(null)
const askDelete = (m: ChatMessageItem) => {
  deleteTarget.value = m
  deleteOpen.value = true
}
const confirmDelete = async () => {
  if (!deleteTarget.value) return
  const res = await api.delete(`/chat/message/${deleteTarget.value.id}`)
  deleteOpen.value = false
  if (res.code === 0) await refreshLoaded()
  else useKunMessage(res.message || '删除失败', 'error')
}

const fmtTime = (d: string | Date) => {
  const date = new Date(d)
  return `${String(date.getHours()).padStart(2, '0')}:${String(
    date.getMinutes()
  ).padStart(2, '0')}`
}

onMounted(async () => {
  if (room.value) {
    await loadLatest()
    resume()
  }
})

const { pause, resume } = useIntervalFn(
  () => {
    if (room.value) pollNew()
  },
  5000,
  { immediate: false }
)
onBeforeUnmount(() => pause())
</script>

<template>
  <div class="flex h-[calc(100vh-12rem)] flex-col">
    <template v-if="room">
      <!-- header -->
      <div class="border-default/20 flex items-center gap-3 border-b px-3 py-2">
        <NuxtLink to="/message/chat" class="md:hidden">
          <KunIcon name="lucide:chevron-left" class="size-5" />
        </NuxtLink>
        <img
          v-if="room.avatar"
          :src="room.avatar"
          :alt="room.name"
          class="bg-default-100 size-10 rounded-full object-cover"
        />
        <div
          v-else
          class="bg-default-100 flex size-10 items-center justify-center rounded-full"
        >
          <KunIcon
            :name="isGroup ? 'lucide:users' : 'lucide:user'"
            class="text-default-500 size-5"
          />
        </div>
        <div>
          <div class="font-semibold">{{ room.name }}</div>
          <div class="text-default-500 text-xs">
            {{
              isGroup ? `群聊 · ${room.member?.length ?? 0} 人` : '私聊'
            }}
          </div>
        </div>
      </div>

      <!-- messages -->
      <div
        ref="scrollArea"
        class="flex-1 space-y-2 overflow-y-auto px-3 py-4"
        @scroll.passive="onScroll"
      >
        <KunLoading v-if="loading" description="加载消息中..." />
        <template v-else-if="messages.length">
          <div
            v-if="loadingOlder"
            class="text-default-400 py-1 text-center text-xs"
          >
            加载更早的消息...
          </div>
          <div
            v-else-if="!hasMoreOlder"
            class="text-default-300 py-1 text-center text-xs"
          >
            没有更早的消息了
          </div>
          <template v-for="m in messages" :key="m.id">
            <!-- deleted tombstone -->
            <div v-if="m.status === 'DELETED'" class="my-1 flex justify-center">
              <span
                class="bg-default-100 text-default-500 rounded-full px-3 py-1 text-xs"
              >
                {{ m.sender_id === myUid ? '您' : `“${m.sender?.name}”` }}
                删除了一条消息
              </span>
            </div>

            <div
              v-else
              :id="`chat-msg-${m.id}`"
              class="group flex items-end gap-2 rounded-lg p-1 transition-shadow"
              :class="m.sender_id === myUid ? 'justify-end' : 'justify-start'"
            >
              <KunAvatar
                v-if="m.sender_id !== myUid"
                :user="m.sender"
                size="sm"
              />
              <div
                class="flex max-w-[72%] flex-col"
                :class="m.sender_id === myUid ? 'items-end' : 'items-start'"
              >
                <div
                  class="relative rounded-xl px-3 py-2 text-sm"
                  :class="
                    m.sender_id === myUid
                      ? 'bg-primary/10'
                      : 'bg-default-100'
                  "
                  @contextmenu="openMenu($event, m)"
                  @click="onBubbleClick($event, m)"
                >
                  <span class="text-primary text-xs font-medium">
                    {{ m.sender?.name }}
                  </span>

                  <!-- reply quote -->
                  <div
                    v-if="m.quote_message"
                    class="border-secondary bg-secondary/10 my-1 cursor-pointer overflow-hidden rounded-lg border-l-3 px-2 py-1"
                    @click="onQuoteClick(m)"
                  >
                    <span class="text-secondary text-xs">
                      {{ m.quote_message.sender_name }}
                    </span>
                    <div
                      class="kun-prose text-xs opacity-80"
                      :class="{ 'line-clamp-2': !expandedQuotes.has(m.id) }"
                      v-html="sanitize(m.quote_message.content)"
                    />
                  </div>

                  <div class="flex flex-wrap items-end gap-2">
                    <div
                      class="kun-prose text-sm break-words"
                      v-html="sanitize(m.content_html)"
                    />
                    <span
                      class="text-default-400 ml-auto translate-y-1 text-xs whitespace-nowrap"
                    >
                      <span v-if="m.status === 'EDITED'">已编辑 </span>
                      {{ fmtTime(m.created) }}
                    </span>
                  </div>
                </div>

                <!-- reaction chips -->
                <div
                  v-if="m.reaction && m.reaction.length"
                  class="mt-1 flex flex-wrap gap-1"
                >
                  <button
                    v-for="r in m.reaction"
                    :key="r.id"
                    type="button"
                    class="bg-default-100 hover:bg-default-200 flex items-center gap-1 rounded-full px-2 py-0.5 text-xs transition-colors"
                    @click="toggleReaction(m, r.emoji)"
                  >
                    <span>{{ r.emoji }}</span>
                    <img
                      v-if="r.user?.avatar"
                      :src="r.user.avatar"
                      :alt="r.user?.name"
                      class="size-4 rounded-full"
                    />
                  </button>
                </div>
              </div>
              <KunAvatar
                v-if="m.sender_id === myUid"
                :user="m.sender"
                size="sm"
                :is-navigation="false"
              />
            </div>
          </template>
        </template>
        <KunNull v-else description="暂无消息, 发一条吧!" />
      </div>

      <!-- reply banner -->
      <div
        v-if="replyTo"
        class="border-default/20 bg-default-50 mx-3 mb-2 flex items-center justify-between rounded-lg border p-2"
      >
        <div class="border-primary min-w-0 border-l-2 pl-3 text-sm">
          <span class="text-primary font-semibold">
            回复给 {{ replyTo.sender?.name }}
          </span>
          <p class="line-clamp-1 opacity-80">{{ replyTo.content }}</p>
        </div>
        <button
          type="button"
          class="text-default-500 hover:text-foreground shrink-0 p-1"
          aria-label="取消回复"
          @click="cancelReply"
        >
          <KunIcon name="lucide:x" class="size-4" />
        </button>
      </div>

      <!-- input -->
      <div class="border-default/20 border-t p-3">
        <div class="flex items-end gap-2">
          <KunPopover position="top-start" inner-class="p-0">
            <template #trigger>
              <KunButton
                is-icon-only
                variant="light"
                aria-label="表情与贴纸"
                :disabled="!myUid"
              >
                <KunIcon name="lucide:smile" class="size-5" />
              </KunButton>
            </template>
            <MessageChatEmojiStickerPicker
              @emoji="onEmojiSelect"
              @sticker="onStickerSend"
            />
          </KunPopover>

          <textarea
            ref="inputEl"
            v-model="input"
            :placeholder="
              myUid ? 'Ctrl + 回车 发送，支持 Markdown' : '请先登录'
            "
            :disabled="!myUid"
            rows="2"
            class="border-default/20 bg-background flex-1 rounded-lg border p-2 text-sm"
            @keydown="handleKeydown"
          />
          <KunButton
            color="primary"
            is-icon-only
            :loading="sending"
            :disabled="sending || !input.trim() || !myUid"
            aria-label="发送"
            @click="sendMessage"
          >
            <KunIcon name="lucide:send-horizontal" class="size-5" />
          </KunButton>
        </div>
      </div>
    </template>
    <KunNull v-else description="聊天会话不存在" />

    <!-- context menu -->
    <MessageChatContextMenu
      v-if="menuTarget"
      :open="menuOpen"
      :anchor="menuAnchor"
      :is-owner="menuTarget.sender_id === myUid"
      :reactions="menuTarget.reaction ?? []"
      @close="menuOpen = false"
      @reply="startReply(menuTarget)"
      @edit="openEdit(menuTarget)"
      @delete="askDelete(menuTarget)"
      @reaction="(e) => toggleReaction(menuTarget!, e)"
    />

    <!-- edit modal -->
    <MessageChatEditModal
      v-if="editTarget"
      v-model:open="editOpen"
      :initial="editTarget.content"
      @save="saveEdit"
    />

    <!-- delete confirm -->
    <KunModal
      :model-value="deleteOpen"
      inner-class-name="max-w-md"
      @update:model-value="(v) => (deleteOpen = v)"
    >
      <div class="space-y-4">
        <div class="space-y-1">
          <h3 class="text-lg font-semibold">删除消息</h3>
          <p class="text-default-500 text-sm">
            消息将被永久删除，所有人不可见且不可恢复，并会留下一条
            「删除了一条消息」的记录。
          </p>
        </div>
        <div class="flex justify-end gap-2">
          <KunButton variant="light" @click="deleteOpen = false">
            取消
          </KunButton>
          <KunButton color="danger" @click="confirmDelete">删除</KunButton>
        </div>
      </div>
    </KunModal>
  </div>
</template>
