<script setup lang="ts">
const props = defineProps<{ keywords: string }>()

// Comments live in the community primitive, and its search face answers a
// keyset cursor with no total — so this lane loads more instead of paginating,
// and its rail count stays blank. It is also absent from 全部: the overview is
// one request per lane on every keyword, and this one is an upstream hop.
const url = computed(
  () => `/search/comment?q=${encodeURIComponent(props.keywords)}`
)

const { items, hasMore, loadMore, loadingMore, pending } = useCommentFeed(url, {
  limit: 24
})

const tooShort = computed(() => props.keywords.trim().length < 2)
</script>

<template>
  <div class="space-y-4">
    <KunNull v-if="tooShort" description="搜索评论需要至少 2 个字符" />
    <template v-else>
      <SearchSkeleton v-if="pending" />
      <div v-else-if="items.length" class="space-y-3">
        <CommentCard v-for="c in items" :key="c.id" :comment="c" />
      </div>
      <KunNull v-else description="没有匹配的评论" />

      <div v-if="hasMore" class="flex justify-center">
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
    </template>
  </div>
</template>
