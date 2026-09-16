<script setup lang="ts">
defineOptions({ name: 'comment-feed' })

useKunSeoMeta({
  title: '最新评论',
  description:
    '鲲 Galgame 补丁站的全站最新评论流，看其他玩家对各款 Galgame 中文汉化补丁的安装体验、剧情讨论和评分反馈。'
})

// Keyset, not pages: the site feed is ordered by creation time and answers a
// cursor, so there is no total to divide and no ?page= to restore.
const { items, hasMore, loadMore, loadingMore, pending } = useCommentFeed(
  '/comment',
  { key: 'comment-feed' }
)
</script>

<template>
  <div class="container mx-auto my-4 space-y-6">
    <KunHeader name="最新评论" description="浏览全站的最新补丁评论" />
    <KunLoading v-if="pending" description="加载评论中..." />
    <div v-else class="space-y-4">
      <CommentCard v-for="c in items" :key="c.id" :comment="c" />
    </div>
    <KunNull v-if="!pending && !items.length" description="暂无评论" />
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
  </div>
</template>
