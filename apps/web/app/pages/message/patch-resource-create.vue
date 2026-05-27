<script setup lang="ts">
useKunDisableSeo('新补丁通知')

interface ListResponse {
  items: Message[]
  total: number
}

const api = useApi()
const { data, pending } = await useAsyncData<ListResponse>(
  'message-patch-create',
  async () => {
    const res = await api.get<ListResponse>(
      '/message?type=patchResourceCreate&page=1&limit=50'
    )
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  { default: () => ({ items: [], total: 0 }) }
)
</script>

<template>
  <div class="space-y-3">
    <KunHeader name="新补丁通知" description="您关注的作者发布了新补丁" />
    <KunLoading v-if="pending" description="加载中..." />
    <template v-else-if="data?.items?.length">
      <MessageCard v-for="m in data.items" :key="m.id" :msg="m" />
    </template>
    <KunNull v-else description="暂无新补丁通知" />
  </div>
</template>
