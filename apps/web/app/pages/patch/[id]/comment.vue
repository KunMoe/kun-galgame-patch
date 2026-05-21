<script setup lang="ts">
import DOMPurify from 'isomorphic-dompurify'

const route = useRoute()
const api = useApi()
const userStore = useUserStore()

// DOMPurify allows <a> by default but strips data-* attrs unless we whitelist
// them. The mention renderer emits data-id so the frontend can wire up
// click-to-profile behaviour later.
const sanitize = (html: string) =>
  DOMPurify.sanitize(html, { ADD_ATTR: ['data-id'] })

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
  if (!userStore.user.id) {
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
  if (!userStore.user.id) {
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
    <!-- composer -->
    <div
      v-if="userStore.user.id"
      class="border-default/20 bg-background flex gap-3 rounded-2xl border p-4"
    >
      <KunAvatar :user="userStore.user" size="sm" :is-navigation="false" />
      <div class="min-w-0 flex-1 space-y-3">
        <textarea
          v-model="content"
          placeholder="写下你的评论..."
          rows="3"
          class="border-default/20 bg-default-50 focus:border-primary w-full rounded-xl border p-3 text-sm transition-colors outline-none"
        />
        <div class="flex justify-end">
          <KunButton
            color="primary"
            rounded="full"
            :loading="submitting"
            :disabled="submitting"
            @click="submit"
          >
            <KunIcon name="lucide:send-horizontal" class="size-4" />
            发布评论
          </KunButton>
        </div>
      </div>
    </div>
    <div
      v-else
      class="border-default/20 bg-default-50 rounded-2xl border p-5 text-center text-sm"
    >
      请
      <NuxtLink to="/login" class="text-primary font-medium hover:underline">
        登录
      </NuxtLink>
      后发表评论
    </div>

    <KunLoading v-if="pending" description="加载评论中..." />
    <div v-else-if="comments && comments.length" class="space-y-4">
      <div
        v-for="c in comments"
        :key="c.id"
        class="border-default/20 bg-background hover:border-primary/30 rounded-2xl border p-5 transition-colors"
      >
        <div class="flex items-start gap-3">
          <KunAvatar :user="c.user" size="sm" />
          <div class="min-w-0 flex-1 space-y-2">
            <div class="flex flex-wrap items-center gap-2">
              <span class="font-semibold">{{ c.user.name }}</span>
              <span class="text-default-400 text-xs">
                {{ formatDate(c.created, { isPrecise: true, isShowYear: true }) }}
              </span>
            </div>
            <div class="kun-prose text-sm" v-html="sanitize(c.content_html)" />
            <button
              type="button"
              :class="
                cn(
                  'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs transition-colors',
                  c.is_liked
                    ? 'bg-danger/10 text-danger'
                    : 'text-default-500 hover:bg-danger/10 hover:text-danger'
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

            <div
              v-if="c.reply?.length"
              class="border-default/20 mt-3 space-y-3 border-l-2 pl-4"
            >
              <div
                v-for="r in c.reply"
                :key="r.id"
                class="bg-default-50 rounded-xl p-3 text-sm"
              >
                <div class="flex items-center gap-2">
                  <KunAvatar :user="r.user" size="xs" />
                  <span class="font-semibold">{{ r.user.name }}</span>
                  <span class="text-default-400 text-xs">
                    {{
                      formatDate(r.created, { isPrecise: true, isShowYear: true })
                    }}
                  </span>
                </div>
                <div class="kun-prose mt-1.5" v-html="sanitize(r.content_html)" />
                <button
                  type="button"
                  :class="
                    cn(
                      'mt-1.5 inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs transition-colors',
                      r.is_liked
                        ? 'bg-danger/10 text-danger'
                        : 'text-default-500 hover:bg-danger/10 hover:text-danger'
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
