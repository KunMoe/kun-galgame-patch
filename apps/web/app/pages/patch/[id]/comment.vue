<script setup lang="ts">
// Patch comment tab. Single-tier threaded model (matches kungal /galgame/:id):
// root comments each carry their replies (one indent), with the first few shown
// inline and the rest behind a ThreadDrawer. This page owns the comments array;
// the Item/Thread components call the APIs and emit results, which the handlers
// below apply optimistically (no refetch, no loading flash).
const route = useRoute()
const api = useApi()
const userStore = useUserStore()
const { requireLogin } = useAuthModal()

const galgameId = computed(() => Number(route.params.id))

interface CommentListResponse {
  items: PatchPageComment[]
  total: number
}

const { data, pending } = await useAsyncData<CommentListResponse>(
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

// ─── new top-level comment composer ───────────────────
const content = ref('')
const submitting = ref(false)
// KunMilkdownDualEditorProvider is uncontrolled; bump the key to remount it
// empty after a successful post.
const composerKey = ref(0)

const submit = async () => {
  if (!requireLogin()) return
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
      if (res.data?.status === 1) {
        useKunMessage('评论已提交，等待管理员审核通过后显示', 'info')
      } else if (data.value) {
        res.data.reply = res.data.reply ?? []
        data.value.items.unshift(res.data)
        data.value.total++
        useKunMessage('评论发布成功', 'success')
      }
    } else {
      useKunMessage(res.message || '发布失败', 'error')
    }
  } finally {
    submitting.value = false
  }
}

// ─── optimistic mutation handlers (this page owns the array) ───
const findComment = (id: number): PatchPageComment | undefined => {
  for (const c of comments.value) {
    if (c.id === id) return c
    const r = c.reply?.find((x) => x.id === id)
    if (r) return r
  }
  return undefined
}

const onLiked = (id: number, liked: boolean) => {
  const c = findComment(id)
  if (!c || c.is_liked === liked) return
  c.like_count = Math.max(0, c.like_count + (liked ? 1 : -1))
  c.is_liked = liked
}

const onReplyAdded = (reply: PatchPageComment) => {
  if (!data.value) return
  // A reply always attaches to a ROOT (parent_id = root id) — one tier.
  const root = data.value.items.find((c) => c.id === reply.parent_id)
  if (!root) return
  reply.reply = reply.reply ?? []
  if (!root.reply) root.reply = []
  root.reply.push(reply)
  data.value.total++
}

const onEdited = (updated: PatchPageComment) => {
  const c = findComment(updated.id)
  if (!c) return
  c.content = updated.content
  c.content_html = updated.content_html
  c.edit = updated.edit
}

const onRemoved = (id: number) => {
  if (!data.value) return
  const rootIdx = data.value.items.findIndex((c) => c.id === id)
  if (rootIdx >= 0) {
    const removed = 1 + (data.value.items[rootIdx]?.reply?.length ?? 0)
    data.value.items.splice(rootIdx, 1)
    data.value.total = Math.max(0, data.value.total - removed)
    if (drawerRoot.value?.id === id) drawerRoot.value = null
    return
  }
  for (const c of data.value.items) {
    const i = c.reply?.findIndex((x) => x.id === id) ?? -1
    if (i >= 0) {
      c.reply.splice(i, 1)
      data.value.total = Math.max(0, data.value.total - 1)
      return
    }
  }
}

// ─── drawer (full thread) ─────────────────────────────
const drawerRoot = ref<PatchPageComment | null>(null)
const openThread = (rootId: number) => {
  drawerRoot.value = data.value?.items.find((c) => c.id === rootId) ?? null
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
    <div v-else-if="comments.length" class="space-y-4">
      <CommentThread
        v-for="c in comments"
        :key="c.id"
        :root="c"
        :galgame-id="galgameId"
        :can-moderate="userStore.isModerator"
        @liked="onLiked"
        @reply-added="onReplyAdded"
        @edited="onEdited"
        @removed="onRemoved"
        @open-thread="openThread"
      />
    </div>
    <KunNull v-else description="暂无评论, 快来抢沙发吧~" />

    <CommentThreadDrawer
      v-model:root="drawerRoot"
      :galgame-id="galgameId"
      :can-moderate="userStore.isModerator"
      @liked="onLiked"
      @reply-added="onReplyAdded"
      @edited="onEdited"
      @removed="onRemoved"
    />
  </div>
</template>
