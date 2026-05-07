<script setup lang="ts">
const route = useRoute()
const api = useApi()
const userId = computed(() => Number(route.params.id))

interface ListResponse {
  items: UserComment[]
  total: number
}

const { data, pending } = await useAsyncData<ListResponse>(
  () => `user-${userId.value}-comments`,
  async () => {
    const res = await api.get<ListResponse>(
      `/user/${userId.value}/comment?page=1&limit=20`
    )
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  { default: () => ({ items: [], total: 0 }) }
)
</script>

<template>
  <div>
    <KunLoading v-if="pending" description="加载中..." />
    <div v-else-if="data?.items?.length" class="space-y-3">
      <NuxtLink
        v-for="c in data.items"
        :key="c.id"
        :to="`/patch/${c.galgame_id}/comment`"
        class="border-default/20 bg-background hover:bg-default-100 block rounded-lg border p-4 transition-colors"
      >
        <div class="text-default-500 mb-1 text-sm">
          评论在
          <span class="text-primary">
            {{ getPreferredLanguageText(c.patch_name) }}
          </span>
        </div>
        <p class="whitespace-pre-wrap line-clamp-3">{{ c.content }}</p>
        <div class="text-default-500 mt-2 flex items-center gap-4 text-xs">
          <div class="flex items-center gap-1">
            <KunIcon name="lucide:thumbs-up" class="size-3.5" />
            {{ c.like }}
          </div>
          <span>{{ formatDistanceToNow(c.created) }}</span>
        </div>
      </NuxtLink>
    </div>
    <KunNull v-else description="该用户暂无评论" />
  </div>
</template>
