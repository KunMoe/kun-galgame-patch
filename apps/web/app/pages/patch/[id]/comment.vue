<script setup lang="ts">
import DOMPurify from 'isomorphic-dompurify'

const route = useRoute()
const api = useApi()
const userStore = useUserStore()

// DOMPurify allows <a> by default but strips data-* attrs unless we whitelist
// them. The mention renderer emits data-uid so the frontend can wire up
// click-to-profile behaviour later.
const sanitize = (html: string) =>
  DOMPurify.sanitize(html, { ADD_ATTR: ['data-uid'] })

const galgameId = computed(() => Number(route.params.id))

// /patch/:id/comment requires page+limit and returns the standard paginated
// envelope { items, total } (apps/api/internal/patch/dto/dto.go
// GetPatchCommentRequest marks both as required).
interface CommentListResponse {
  items: PatchPageComment[]
  total: number
}

const { data, pending, refresh } = await useAsyncData<CommentListResponse>(
  () => `patch-comments-${galgameId.value}`,
  async () => {
    const res = await api.get<CommentListResponse>(
      `/patch/${galgameId.value}/comment?page=1&limit=30`
    )
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  { default: () => ({ items: [], total: 0 }) }
)

const comments = computed(() => data.value?.items ?? [])

const content = ref('')
const submitting = ref(false)

const submit = async () => {
  if (!userStore.user.uid) {
    useKunMessage('请先登录', 'warn')
    return
  }
  if (!content.value.trim()) {
    useKunMessage('评论内容不能为空', 'warn')
    return
  }
  submitting.value = true
  try {
    const res = await api.post<PatchPageComment>(
      `/patch/${galgameId.value}/comment`,
      { content: content.value }
    )
    if (res.code === 0) {
      content.value = ''
      useKunMessage('评论发布成功', 'success')
      await refresh()
    } else {
      useKunMessage(res.message || '发布失败', 'error')
    }
  } finally {
    submitting.value = false
  }
}

const renderComment = (c: PatchPageComment): PatchPageComment => c

// Optimistic toggle for the heart on each comment / reply.
//
// Backend returns { liked: boolean }; we apply it to the local row plus
// adjust the displayed like_count by the resulting delta. On error we leave
// state untouched so the next refresh reconciles.
const toggleLike = async (c: PatchPageComment) => {
  if (!userStore.user.uid) {
    useKunMessage('请先登录后再点赞', 'warn')
    return
  }
  const res = await api.put<{ liked: boolean }>(
    `/patch/comment/${c.id}/like`
  )
  if (res.code === 0) {
    const liked = res.data.liked
    const delta = liked === c.is_liked ? 0 : liked ? 1 : -1
    c.is_liked = liked
    c.like_count = Math.max(0, c.like_count + delta)
  } else {
    useKunMessage(res.message || '操作失败', 'error')
  }
}
</script>

<template>
  <div class="space-y-6">
    <div v-if="userStore.user.uid" class="space-y-2">
      <textarea
        v-model="content"
        placeholder="写下你的评论..."
        rows="3"
        class="border-default/20 bg-background w-full rounded-lg border p-3 text-sm"
      />
      <div class="flex justify-end">
        <KunButton
          color="primary"
          :loading="submitting"
          :disabled="submitting"
          @click="submit"
        >
          发布评论
        </KunButton>
      </div>
    </div>
    <div
      v-else
      class="border-default/20 rounded-lg border p-4 text-center text-sm"
    >
      请
      <NuxtLink to="/login" class="text-primary hover:underline">登录</NuxtLink>
      后发表评论
    </div>

    <KunLoading v-if="pending" description="加载评论中..." />
    <div v-else-if="comments && comments.length" class="space-y-3">
      <div
        v-for="c in comments"
        :key="c.id"
        class="border-default/20 space-y-2 rounded-lg border p-4"
      >
        <div class="flex items-start gap-3">
          <KunAvatar :user="c.user" size="sm" />
          <div class="flex-1 space-y-1">
            <div class="flex flex-wrap items-center gap-2">
              <span class="font-semibold">{{ c.user.name }}</span>
              <span class="text-default-500 text-xs">
                {{ formatDate(c.created, { isPrecise: true, isShowYear: true }) }}
              </span>
            </div>
            <div class="kun-prose" v-html="sanitize(c.content_html)" />
            <button
              type="button"
              :class="
                cn(
                  'flex items-center gap-1 text-xs transition-colors',
                  c.is_liked
                    ? 'text-danger-500'
                    : 'text-default-500 hover:text-danger-500'
                )
              "
              :aria-label="c.is_liked ? '取消点赞' : '点赞'"
              @click="toggleLike(c)"
            >
              <KunIcon
                name="lucide:thumbs-up"
                :class="cn('size-3.5', c.is_liked ? 'fill-current' : '')"
              />
              {{ c.like_count }}
            </button>
            <div v-if="c.reply?.length" class="mt-3 space-y-2 border-l-2 border-default/20 pl-3">
              <div
                v-for="r in c.reply"
                :key="r.id"
                class="bg-default-50 rounded p-2 text-sm"
              >
                <div class="flex items-center gap-2">
                  <KunAvatar :user="r.user" size="xs" />
                  <span class="font-semibold">{{ r.user.name }}</span>
                  <span class="text-default-500 text-xs">
                    {{
                      formatDate(r.created, { isPrecise: true, isShowYear: true })
                    }}
                  </span>
                </div>
                <div class="kun-prose mt-1" v-html="sanitize(r.content_html)" />
                <button
                  type="button"
                  :class="
                    cn(
                      'mt-1 flex items-center gap-1 text-xs transition-colors',
                      r.is_liked
                        ? 'text-danger-500'
                        : 'text-default-500 hover:text-danger-500'
                    )
                  "
                  :aria-label="r.is_liked ? '取消点赞' : '点赞'"
                  @click="toggleLike(r)"
                >
                  <KunIcon
                    name="lucide:thumbs-up"
                    :class="cn('size-3.5', r.is_liked ? 'fill-current' : '')"
                  />
                  {{ r.like_count }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
    <KunNull v-else description="暂无评论, 快来抢沙发吧~" />
  </div>
</template>
