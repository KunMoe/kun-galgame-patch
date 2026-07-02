<script setup lang="ts">
// Patch comment tab. Single-tier threaded model (matches kungal /galgame/:id):
// root comments each carry their replies (one indent), with the first few shown
// inline and the rest expandable in place. This page owns the comments array;
// the Item/Thread components call the APIs and emit results, which the handlers
// below apply optimistically (no refetch, no loading flash).
const route = useRoute()
const router = useRouter()
const api = useApi()
const userStore = useUserStore()
const { requireLogin } = useAuthModal()

const galgameId = computed(() => Number(route.params.id))

const LIMIT = 30
const page = computed({
  get: () => Number(route.query.page) || 1,
  set: (v: number) => {
    router.push({ query: { ...route.query, page: v }, hash: route.hash })
  }
})

interface CommentListResponse {
  items: PatchPageComment[]
  total: number
}

const { data, pending } = await useAsyncData<CommentListResponse>(
  () => `patch-comments-${galgameId.value}-${page.value}`,
  async () => {
    const res = await api.get<CommentListResponse>(
      `/patch/${galgameId.value}/comment?page=${page.value}&limit=${LIMIT}`
    )
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  {
    default: () => ({ items: [], total: 0 }),
    watch: [page, galgameId],
    // Nuxt 4 makes `data` a shallowRef by default, so the optimistic handlers
    // below (unshift/splice/push + nested like_count/content edits on
    // data.value.items) wouldn't trigger a re-render. deep:true restores the
    // deep reactivity those in-place mutations rely on.
    deep: true
  }
)

const comments = computed(() => data.value?.items ?? [])
// total = APPROVED root-comment count (the server paginates over roots), so it
// drives the paginator directly.
const totalPage = computed(() =>
  Math.max(1, Math.ceil((data.value?.total ?? 0) / LIMIT))
)

// ─── new top-level comment composer ───────────────────
const content = ref('')
const submitting = ref(false)
// <KunMarkdownEditor> is uncontrolled; bump the key to remount it empty after a
// successful post.
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
        useKunMessage('评论已提交，等待版主审核通过后显示', 'info')
      } else if (data.value) {
        res.data.reply = res.data.reply ?? []
        // The create response carries user_id but NOT the resolved `user` (only
        // the list endpoint enriches it via the OAuth batch). The author is the
        // current user, so stamp it — else <Item> throws on comment.user.name.
        data.value.items.unshift({ ...res.data, user: userStore.user })
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
  // Expand the thread so the just-posted reply is visible even when it lands
  // past the inline preview (otherwise it'd hide behind "展开更多").
  expandedRoots.value.add(root.id)
  // total = root count → a reply doesn't change it (keeps totalPage correct).
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
    data.value.items.splice(rootIdx, 1)
    // total = root count → removing a root drops it by exactly 1.
    data.value.total = Math.max(0, data.value.total - 1)
    expandedRoots.value.delete(id)
    return
  }
  for (const c of data.value.items) {
    const i = c.reply?.findIndex((x) => x.id === id) ?? -1
    if (i >= 0) {
      c.reply.splice(i, 1)
      // reply removal doesn't affect the root count / paginator
      return
    }
  }
}

// ─── inline thread expansion ──────────────────────────
// Root ids whose replies are fully expanded in place (a Set so several threads
// can be open at once; a deep-link jump adds the target's root here). Vue 3
// proxies Set mutations, so add/delete are reactive.
const expandedRoots = ref<Set<number>>(new Set())
const toggleExpand = (rootId: number) => {
  if (expandedRoots.value.has(rootId)) expandedRoots.value.delete(rootId)
  else expandedRoots.value.add(rootId)
}

// ─── deep-link: jump to a specific comment, across pages ──
// Links (messages / home / global /comment) point at
// /patch/:id/comment#comment-:cid. Try the current page first; if the target
// isn't here, ask the server which page it's on (GET .../locate), navigate
// there, then scroll. A collapsed reply (only the first 3 show inline) is
// revealed by opening its thread drawer. Once found, scroll + flash it.
const flash = (el: HTMLElement) => {
  el.scrollIntoView({ behavior: 'smooth', block: 'center' })
  el.classList.add('kun-comment-flash')
  setTimeout(() => el.classList.remove('kun-comment-flash'), 2000)
}

const tryScroll = (id: number) => {
  const el = document.getElementById(`comment-${id}`)
  if (el) {
    flash(el)
    return true
  }
  return false
}

// A collapsed reply (beyond the inline preview) only renders once its thread is
// expanded. Expand it in place, then poll briefly for the reply element to
// appear (the extra replies mount on the next tick) and scroll to it.
const revealReplyInline = (rootId: number, id: number) => {
  expandedRoots.value.add(rootId)
  let tries = 0
  const tick = () => {
    if (tryScroll(id) || tries++ > 12) return
    setTimeout(tick, 60)
  }
  nextTick(tick)
}

// Set while waiting for a page navigation's data to load (consumed by watch).
const pendingTarget = ref<{
  id: number
  rootId: number
  isReply: boolean
} | null>(null)

const resolveTarget = async () => {
  const m = route.hash.match(/^#comment-(\d+)$/)
  if (!m) return
  const id = Number(m[1])
  await nextTick()
  if (tryScroll(id)) return

  const res = await api
    .get<{ page: number; root_id: number; is_reply: boolean }>(
      `/patch/comment/${id}/locate?limit=${LIMIT}`
    )
    .catch(() => null)
  if (!res || res.code !== 0) return
  const { page: targetPage, root_id: rootId, is_reply: isReply } = res.data

  if (targetPage !== page.value) {
    // Navigate; the watch(data) below finishes the jump once the page loads.
    pendingTarget.value = { id, rootId, isReply }
    page.value = targetPage
    return
  }
  // Right page but not inline → a collapsed reply.
  if (isReply) revealReplyInline(rootId, id)
}

// Finish a jump after the navigated-to page's data arrives (useAsyncData
// replaces data.value on refetch, so this fires on page change, not on the
// in-place optimistic mutations).
watch(data, async () => {
  const t = pendingTarget.value
  if (!t) return
  pendingTarget.value = null
  await nextTick()
  if (tryScroll(t.id)) return
  if (t.isReply) revealReplyInline(t.rootId, t.id)
})

onMounted(resolveTarget)
watch(() => route.hash, resolveTarget)
</script>

<template>
  <div class="space-y-6">
    <!-- composer -->
    <div
      v-if="userStore.user.id"
      class="border-default/20 bg-content1 shadow-kun-sm flex gap-3 rounded-2xl border p-4"
    >
      <KunAvatar :user="userStore.user" size="sm" :is-navigation="false" />
      <div class="min-w-0 flex-1 space-y-3">
        <KunMarkdownEditor
          :key="composerKey"
          :model-value="content"
          placeholder="发布一条可爱的评论吧～"
          @update:model-value="(val) => (content = val)"
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
        @click="() => startOAuthLogin()"
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
        :expanded="expandedRoots.has(c.id)"
        :can-moderate="userStore.isModerator"
        @liked="onLiked"
        @reply-added="onReplyAdded"
        @edited="onEdited"
        @removed="onRemoved"
        @toggle-expand="toggleExpand"
      />
    </div>
    <KunNull v-else description="暂无评论, 快来抢沙发吧~" />

    <KunPagination
      v-if="totalPage > 1"
      v-model:current-page="page"
      :total-page="totalPage"
      :is-loading="pending"
      class="mt-2"
    />
  </div>
</template>
