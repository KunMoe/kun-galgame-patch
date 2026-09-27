<script setup lang="ts">
const props = defineProps<{ openId: string | null }>()

const chat = useChatStore()
const folder = ref<ChatFolder>('inbox')

const tabs = computed(() => [
  { value: 'inbox', textValue: '全部' },
  {
    value: 'requests',
    textValue: chat.state?.request_count
      ? `消息请求 ${chat.state.request_count}`
      : '消息请求'
  },
  { value: 'archive', textValue: '已归档' }
])

watch(folder, (f) => void chat.loadFolder(f), { immediate: true })

const openFolder = computed(() => {
  const c = props.openId ? chat.model.conversations[props.openId] : undefined
  return c ? chatFolderOf(c) : null
})
watch(
  openFolder,
  (f) => {
    if (f) folder.value = f
  },
  { immediate: true }
)

const items = computed(() => chat.list(folder.value))
const users = computed(() => chat.users)

const peer = (c: ChatConversation) =>
  c.peer_id ? (chat.model.users[c.peer_id] ?? null) : null

const senderLabel = (c: ChatConversation) => {
  const m = c.last_message
  if (!m || m.kind !== 'message') return null
  if (m.sender_id === chat.model.me) return '你'
  return c.kind === 'group'
    ? (chat.model.users[m.sender_id]?.name ?? null)
    : null
}

const draftOf = (c: ChatConversation) =>
  c.me.draft && c.id !== props.openId
    ? { text: c.me.draft.text, entities: c.me.draft.entities }
    : null

const statusOf = (c: ChatConversation) => {
  const m = c.last_message
  if (!m || m.sender_id !== chat.model.me) return undefined
  return m.seq <= c.peer_read_seq ? 'read' : 'sent'
}
</script>

<template>
  <div class="bg-content1 flex h-full flex-col">
    <div class="border-default-200 border-b px-3 pt-3">
      <h1 class="mb-2 text-lg font-semibold">私信</h1>
      <KunTab
        v-model="folder"
        :items="tabs"
        variant="underlined"
        size="sm"
        name="messages-folder"
      />
    </div>

    <div class="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto p-1.5">
      <!-- The item is overflow-hidden, so as a flex child it shrank: forty
           conversations were squeezed into ~17px rows instead of scrolling. -->
      <KunChatConversationItem
        v-for="c in items"
        :key="c.id"
        class="shrink-0"
        :href="`/messages/${c.id}`"
        :kind="c.kind"
        :user="peer(c)"
        :title="c.title ?? undefined"
        :avatar="
          c.photo_image_hash ? imageServiceUrl(c.photo_image_hash) : null
        "
        :last-message="c.last_message"
        :last-message-sender="senderLabel(c)"
        :users="users"
        :current-user-id="chat.model.me"
        :typing="chat.typing[c.id] ?? []"
        :draft="draftOf(c)"
        :unread-count="c.id === props.openId ? 0 : c.me.unread_count"
        :marked-unread="c.me.marked_unread"
        :muted="chatMuted(c)"
        :pinned="c.me.pinned_rank !== null"
        :status="statusOf(c)"
        :selected="c.id === props.openId"
      />

      <KunNull
        v-if="!items.length && chat.folders[folder].loaded"
        :description="
          folder === 'requests'
            ? '没有待处理的消息请求'
            : folder === 'archive'
              ? '没有归档的对话'
              : '还没有私信，去用户主页打个招呼吧'
        "
      />

      <KunButton
        v-if="chat.folders[folder].next"
        variant="light"
        size="sm"
        class="mx-auto my-2 shrink-0"
        :loading="chat.folders[folder].loading"
        @click="chat.loadFolder(folder, true)"
      >
        加载更多
      </KunButton>
    </div>
  </div>
</template>
