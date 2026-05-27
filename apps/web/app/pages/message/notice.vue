<script setup lang="ts">
useKunDisableSeo('通知消息')

interface ListResponse {
  items: Message[]
  total: number
}

const api = useApi()
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
