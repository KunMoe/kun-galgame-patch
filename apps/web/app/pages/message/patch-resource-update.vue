<script setup lang="ts">
useKunSeoMeta({
  title: '补丁更新通知',
  description: '订阅的补丁更新通知'
})

interface ListResponse {
  items: Message[]
  total: number
}

const api = useApi()
const { data, pending } = await useAsyncData<ListResponse>(
  'message-patch-update',
  async () => {
    const res = await api.get<ListResponse>(
      '/message?type=patchResourceUpdate&page=1&limit=50'
    )
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  { default: () => ({ items: [], total: 0 }) }
)
</script>

<template>
  <div class="space-y-3">
    <KunHeader name="补丁更新通知" description="您收藏的补丁有更新" />
    <KunLoading v-if="pending" description="加载中..." />
    <template v-else-if="data?.items?.length">
      <MessageCard v-for="m in data.items" :key="m.id" :msg="m" />
    </template>
    <KunNull v-else description="暂无更新通知" />
  </div>
</template>
