<script setup lang="ts">
useKunDisableSeo('通知消息')

interface ListResponse {
  items: Message[]
  total: number
}

const api = useApi()
const messageStore = useMessageStore()

const { data, pending } = await useAsyncData<ListResponse>(
  'message-notice',
  async () => {
    // /message/all returns every message regardless of type — see
    // apps/api/internal/message/handler GetAllMessages.
    const res = await api.get<ListResponse>('/message/all?page=1&limit=50')
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  { default: () => ({ items: [], total: 0 }) }
)

// Entering the notifications inbox marks everything read and clears the bell's
// "new" dot. Client-only (onMounted) so it fires when the user actually views
// the page, not during SSR prefetch. type:'all' → all user_message rows
// (notifications only; chat is separate). Always fired (not gated on the
// store) so a direct navigation / refresh on this page also marks read; the
// PUT is idempotent when there's nothing unread.
onMounted(async () => {
  const res = await api.put('/message/read', { type: 'all' })
  if (res.code === 0) messageStore.clear()
})
</script>

<template>
  <div class="space-y-3">
    <KunHeader name="通知消息" description="全部通知消息" />
    <KunLoading v-if="pending" description="加载中..." />
    <template v-else-if="data?.items?.length">
      <MessageCard v-for="m in data.items" :key="m.id" :msg="m" />
    </template>
    <KunNull v-else description="暂无消息" />
  </div>
</template>
