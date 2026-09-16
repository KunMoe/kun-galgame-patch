<script setup lang="ts">
useKunDisableSeo('评论管理')

const api = useApi()

// The queue is a search over the comment walls, or the newest comments when
// there is no keyword. There is no 待审核 filter any more: the community
// primitive has no pre-moderation switch, and a newcomer's held first posts are
// released through ITS review queue, not from here.
const keyword = ref('')
const submitted = ref('')

const url = computed(() =>
  submitted.value
    ? `/admin/comment?q=${encodeURIComponent(submitted.value)}`
    : '/admin/comment/recent'
)

const { items, hasMore, loadMore, loadingMore, pending, refresh } =
  useCommentFeed(url, { limit: 30, key: 'admin-comments' })

const search = () => {
  const q = keyword.value.trim()
  if (q && q.length < 2) {
    useKunMessage('搜索评论需要至少 2 个字符', 'warn')
    return
  }
  submitted.value = q
}

const handleDelete = async (id: number) => {
  const ok = await useKunAlert({
    title: '删除评论',
    type: 'danger',
    message: '确定要删除这条评论吗？评论会保留位置并显示为「已删除」，作者会收到通知。'
  })
  if (!ok) return
  const res = await api.delete(`/admin/comment/${id}`, { reason: '' })
  if (res.code === 0) {
    useKunMessage('已删除', 'success')
    await refresh()
  } else {
    useKunMessage(res.message || '删除失败', 'error')
  }
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h1 class="text-2xl font-bold">评论管理</h1>
      <div class="flex items-center gap-2">
        <KunInput
          v-model="keyword"
          placeholder="搜索评论内容（2-100 字）"
          @keydown.enter="search"
        />
        <KunButton color="primary" size="sm" @click="search">搜索</KunButton>
      </div>
    </div>

    <KunLoading v-if="pending" description="加载中..." />
    <div v-else class="space-y-3">
      <KunCard v-for="c in items" :key="c.id" :bordered="true">
        <div class="flex items-start gap-3">
          <KunAvatar v-if="c.user" :user="c.user" size="sm" />
          <div class="min-w-0 flex-1 space-y-1">
            <div class="flex flex-wrap items-center gap-2 text-sm">
              <span class="font-semibold">{{ c.user?.name ?? '未知用户' }}</span>
              <span class="text-default-500">
                在
                <NuxtLink :to="c.link" class="text-primary hover:underline">
                  {{
                    c.patch?.name
                      ? getPreferredLanguageText(c.patch.name)
                      : `补丁 #${c.galgame_id}`
                  }}
                </NuxtLink>
                <template v-if="c.resource_id">的补丁资源</template>
              </span>
              <span class="text-default-400 text-xs">
                {{ formatDate(c.created, { isShowYear: true, isPrecise: true }) }}
              </span>
            </div>
            <p class="break-words whitespace-pre-wrap">{{ c.content }}</p>
          </div>
          <div class="flex shrink-0 gap-2">
            <KunButton
              size="sm"
              variant="light"
              color="danger"
              @click="handleDelete(c.id)"
            >
              删除
            </KunButton>
          </div>
        </div>
      </KunCard>
    </div>

    <KunNull
      v-if="!pending && !items.length"
      :description="submitted ? '没有匹配的评论' : '暂无评论'"
    />

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
