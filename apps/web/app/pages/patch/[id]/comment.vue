<script setup lang="ts">
const route = useRoute()
const api = useApi()
const userStore = useUserStore()

// Comment / reply bodies render via <KunContent>, which handles sanitize +
// spoiler + per-block inline-image lightbox internally. Each <KunContent>
// in the v-for is its own component instance with its own lightbox, so
// images are scoped to that comment and newly-posted / paginated comments
// get clickable images for free (delegated click + live img scan).

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
// KunMilkdownDualEditorProvider is uncontrolled (valueMarkdown is the initial
// value only), so bump this key to remount it empty after a successful post.
const composerKey = ref(0)

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
      composerKey.value++
      // When 评论审核 is on the comment is created pending (status=1) and is
      // NOT yet visible — tell the user instead of refreshing (nothing new to
      // show). Otherwise it's live: refresh to display it.
      if (res.data?.status === 1) {
        useKunMessage('评论已提交，等待管理员审核通过后显示', 'info')
      } else {
        useKunMessage('评论发布成功', 'success')
        await refresh()
      }
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
        <KunMilkdownDualEditorProvider
          :key="composerKey"
          :value-markdown="content"
          @set-markdown="(val) => (content = val)"
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
      <button
        type="button"
        class="text-primary font-medium hover:underline"
        @click="startOAuthLogin"
      >
        登录
      </button>
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
            <KunContent :content="c.content_html" class-name="text-sm" />
            <KunButton
              :variant="c.is_liked ? 'flat' : 'light'"
              color="danger"
              size="xs"
              rounded="full"
              :aria-label="c.is_liked ? '取消点赞' : '点赞'"
              @click="toggleLike(c)"
            >
              <KunIcon
                name="lucide:thumbs-up"
                :class="cn('size-3.5', c.is_liked && 'fill-current')"
              />
              {{ c.like_count }}
            </KunButton>

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
                <KunContent :content="r.content_html" class-name="mt-1.5" />
                <KunButton
                  :variant="r.is_liked ? 'flat' : 'light'"
                  color="danger"
                  size="xs"
                  rounded="full"
                  class-name="mt-1.5"
                  :aria-label="r.is_liked ? '取消点赞' : '点赞'"
                  @click="toggleLike(r)"
                >
                  <KunIcon
                    name="lucide:thumbs-up"
                    :class="cn('size-3.5', r.is_liked && 'fill-current')"
                  />
                  {{ r.like_count }}
                </KunButton>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
    <KunNull v-else description="暂无评论, 快来抢沙发吧~" />
  </div>
</template>
