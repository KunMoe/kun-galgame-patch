<script setup lang="ts">
defineOptions({ name: 'user-comment' })

const route = useRoute()
const userId = computed(() => Number(route.params.id))

const { items, hasMore, loadMore, loadingMore, pending } = useCommentFeed(
  computed(() => `/user/${userId.value}/comment`)
)

const patchName = (c: UserComment) =>
  c.patch?.name ? getPreferredLanguageText(c.patch.name) : `补丁 #${c.galgame_id}`
</script>

<template>
  <div>
    <KunLoading v-if="pending" description="加载中..." />
    <div v-else-if="items.length" class="space-y-3">
      <NuxtLink
        v-for="c in items"
        :key="c.id"
        :to="c.link"
        class="border-default/20 bg-content1 shadow-kun-sm hover:bg-default-100 block rounded-lg border p-4 transition-colors"
      >
        <div class="text-default-500 mb-1 text-sm">
          {{ c.resource_id ? '评论了' : '评论在' }}
          <span class="text-primary">{{ patchName(c) }}</span>
          <template v-if="c.resource_id">的补丁资源</template>
        </div>
        <p class="line-clamp-3 whitespace-pre-wrap">{{ c.content }}</p>
        <div class="text-default-500 mt-2 flex items-center gap-4 text-xs">
          <div class="flex items-center gap-1">
            <KunIcon name="lucide:thumbs-up" class="size-3.5" />
            {{ c.like_count }}
          </div>
          <span>{{ formatDistanceToNow(c.created) }}</span>
        </div>
      </NuxtLink>
    </div>
    <KunNull v-else description="该用户暂无评论" />

    <div v-if="hasMore" class="mt-6 flex justify-center">
      <KunButton
        variant="light"
        color="primary"
        :loading="loadingMore"
        :disabled="loadingMore"
        @click="loadMore"
      >
        加载更多
      </KunButton>
    </div>
  </div>
</template>
