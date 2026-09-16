<script setup lang="ts">
useKunDisableSeo('关注的评论区')

const api = useApi()

// The walls this reader follows that have posts they have not read. Following
// one is what creates the row at all: a wall merely opened carries no state, so
// this list is never "every game page you ever visited".
const { data, pending } = await useAsyncData<CommentUnreadResult>(
  'community-unread',
  async () => {
    const res = await api.get<CommentUnreadResult>('/community/unread?limit=50')
    return res.code === 0 ? res.data : { items: [], next_cursor: '' }
  },
  { default: () => ({ items: [], next_cursor: '' }) }
)
</script>

<template>
  <div class="space-y-3">
    <KunHeader
      name="关注的评论区"
      description="你关注的游戏 / 资源评论区里有新评论时会出现在这里，打开评论区即视为已读"
    />
    <KunLoading v-if="pending" description="加载中..." />
    <div v-else-if="data?.items.length" class="space-y-2">
      <NuxtLink
        v-for="item in data.items"
        :key="item.thread_id"
        :to="item.link"
        class="border-default/20 bg-content1 shadow-kun-sm hover:bg-default-100 flex items-center gap-3 rounded-lg border p-4 transition-colors"
      >
        <KunIcon
          name="lucide:message-square"
          class="text-primary size-5 shrink-0"
        />
        <div class="min-w-0 flex-1">
          <p class="truncate font-medium">{{ item.title }}</p>
          <p class="text-default-500 text-xs">
            {{ item.label }} · 最后回复于
            {{ formatDistanceToNow(item.last_posted_at) }}
          </p>
        </div>
        <KunChip size="sm" variant="flat" color="primary">
          {{ item.unread_count }} 条新评论
        </KunChip>
      </NuxtLink>
    </div>
    <KunNull v-else description="暂无未读评论" />
  </div>
</template>
