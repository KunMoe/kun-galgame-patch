// The comment-area read + its optimistic mutation handlers, lifted out of the
// components that render it.
//
// Lifted for the same reason as useResourceRevisions: the resource detail page
// needs the COUNT before it can lay itself out (the 评论 tab is labelled with
// it), while the rendering lives in CommentSection. Fetching inside the section
// and emitting the count back up would settle the tab label one tick after the
// parent first rendered — a visible flicker. The page awaits this instead.
//
// The wall is a community thread: a flat sequence of posts keyset by post
// number, each naming its parent by id. So this loads FORWARD (没有页码) and
// assembles the two-tier tree itself.
import {
  commentAnchorId,
  commentSurface,
  type CommentTarget
} from '~/shared/utils/commentTarget'

export const COMMENT_LIMIT = 30

export interface CommentGroup {
  root: PatchPageComment
  replies: PatchPageComment[]
}

export const useCommentList = (target: Ref<CommentTarget> | CommentTarget) => {
  const api = useApi()
  const route = useRoute()
  const userStore = useUserStore()

  const resolved = computed(() => unref(target))
  const surface = computed(() => commentSurface(resolved.value))

  const subscription = ref<CommentWallState | null>(null)
  const loadingMore = ref(false)

  const emptyPage = (): PatchCommentPage => ({
    thread_id: 0,
    posts: [],
    next_cursor: '',
    total: 0
  })

  // The wall IS `data`: loadMore and the mutation handlers below write into it.
  // A separate ref seeded from it stays empty on the server, which runs no
  // watchers, so every wall rendered empty there and hydrated a full list over
  // it.
  const { data, pending } = useAsyncData<PatchCommentPage>(
    () => `comments-${surface.value.listUrl}`,
    async () => {
      const res = await api.get<PatchCommentPage>(
        `${surface.value.listUrl}?limit=${COMMENT_LIMIT}`
      )
      return res.code === 0 ? res.data : emptyPage()
    },
    {
      default: emptyPage,
      // Watch the listUrl STRING, not `surface` — that computed builds a fresh
      // object on every evaluation, so its identity always differs and the
      // watcher would refire on any unrelated recompute of the target. On the
      // resource page `target` is a computed over the deep-reactive `detail`, so
      // bumping the download counter alone would have refetched the comments.
      watch: [() => surface.value.listUrl],
      // Nuxt 4 makes `data` a shallowRef; onLiked flips fields on a post in
      // place.
      deep: true
    }
  )

  const posts = computed(() => data.value.posts)
  const total = computed(() => data.value.total)
  const threadId = computed(() => data.value.thread_id)

  // The read receipt is a POST the reader makes, never something inferred from
  // the GET above: a read face with a write side effect cannot be cached,
  // retried or prefetched safely. It answers with the viewer's standing on this
  // wall, which is what gives the follow toggle a state to render, and it
  // creates nothing for a wall the viewer has never written on or followed —
  // otherwise opening one game page would enrol them in its notifications.
  //
  // Declared above the watcher that calls it during setup: declared below it,
  // every wall 500'd with "Cannot access 'reportRead' before initialization".
  const wallBody = () => {
    const t = resolved.value
    return {
      kind: t.kind,
      id: t.kind === 'resource' ? t.resourceId : t.galgameId
    }
  }

  const reportRead = async () => {
    if (!userStore.user.id || import.meta.server) return
    const body = wallBody()
    if (!body.id) return
    const res = await api
      .post<CommentWallState>('/community/wall/read', body)
      .catch(() => null)
    if (res?.code === 0 && res.data) {
      subscription.value = res.data
    }
  }

  watch([() => surface.value.listUrl, threadId], reportRead, {
    immediate: true
  })

  const setLevel = async (level: CommentNotificationLevel) => {
    const res = await api.post<CommentWallState>(
      '/community/wall/notification',
      { ...wallBody(), level }
    )
    if (res.code === 0 && res.data) {
      subscription.value = res.data
    } else {
      useKunMessage(res.message || '操作失败', 'error')
    }
  }

  const hasMore = computed(() => data.value.next_cursor !== '')

  const loadMore = async () => {
    if (!hasMore.value || loadingMore.value) return
    loadingMore.value = true
    try {
      const page = data.value
      const res = await api.get<PatchCommentPage>(
        `${surface.value.listUrl}?after=${page.next_cursor}&limit=${COMMENT_LIMIT}`
      )
      if (res.code !== 0) return
      const seen = new Set(page.posts.map((p) => p.id))
      page.posts = [
        ...page.posts,
        ...res.data.posts.filter((p) => !seen.has(p.id))
      ]
      page.next_cursor = res.data.next_cursor
      page.total = res.data.total
    } finally {
      loadingMore.value = false
    }
  }

  // Two tiers, assembled from the flat sequence. A reply whose root has not been
  // loaded yet stands on its own rather than disappearing — the wall reads
  // forward, so that only happens transiently while paging.
  const groups = computed<CommentGroup[]>(() => {
    const list: CommentGroup[] = []
    const byRoot = new Map<number, CommentGroup>()
    for (const p of posts.value) {
      const rootId = p.root_comment_id
      if (rootId == null) {
        const group: CommentGroup = { root: p, replies: [] }
        byRoot.set(p.id, group)
        list.push(group)
        continue
      }
      const owner = byRoot.get(rootId)
      if (owner) owner.replies.push(p)
      else list.push({ root: p, replies: [] })
    }
    return list
  })

  const hasComments = computed(() => groups.value.length > 0)

  // ─── optimistic mutation handlers ───────────────────
  const findPost = (id: number) => posts.value.find((p) => p.id === id)

  const onLiked = (id: number, liked: boolean) => {
    const p = findPost(id)
    if (!p || p.is_liked === liked) return
    p.like_count = Math.max(0, p.like_count + (liked ? 1 : -1))
    p.is_liked = liked
  }

  const onCommentAdded = (comment: PatchPageComment) => {
    const page = data.value
    if (page.posts.some((p) => p.id === comment.id)) return
    page.posts = [...page.posts, comment]
    page.total += 1
    expandedRoots.value.add(comment.root_comment_id ?? comment.id)
    // A new thread id reaches reportRead through its watcher.
    if (!page.thread_id && comment.thread_id) {
      page.thread_id = comment.thread_id
    } else {
      reportRead()
    }
  }

  const onEdited = (updated: PatchPageComment) => {
    const page = data.value
    page.posts = page.posts.map((p) => (p.id === updated.id ? updated : p))
  }

  // A delete is a tombstone upstream: the post keeps its number so the wall's
  // numbering never collapses. Removing the row here instead would make replies
  // under it look orphaned until the next read. `total` is the thread's
  // posts_count, which counts the stub too; decrementing it here showed one
  // fewer comment than the same wall after a reload.
  const onRemoved = (id: number) => {
    const page = data.value
    page.posts = page.posts.map((p) =>
      p.id === id
        ? { ...p, deleted: true, held: false, content: '', content_html: '' }
        : p
    )
  }

  // ─── inline thread expansion ────────────────────────
  const expandedRoots = ref<Set<number>>(new Set())
  const toggleExpand = (rootId: number) => {
    if (expandedRoots.value.has(rootId)) expandedRoots.value.delete(rootId)
    else expandedRoots.value.add(rootId)
  }

  // ─── deep-link: jump to a specific comment ──────────
  // Landing on a deep-link target is NOT a one-shot scroll. Everything above the
  // target keeps growing after the list first paints — rendered markdown, inline
  // images, the editor — so a single scrollIntoView aims at coordinates that then
  // move out from under it. Measured on a cross-page jump: the target settled
  // ~1900px below where we had scrolled to, off-screen, with plenty of page left.
  //
  // So re-assert the position over a short window, and stop the instant the
  // reader takes over — a correction that fights someone's own scrolling is worse
  // than landing slightly off.
  const OFF_CENTER_TOLERANCE = 120
  const SETTLE_TRIES = 12
  const SETTLE_INTERVAL = 150
  const LOAD_GUARD = 50

  const offCenterBy = (el: HTMLElement) => {
    const r = el.getBoundingClientRect()
    return Math.abs(r.top + r.height / 2 - window.innerHeight / 2)
  }

  const scrollToStable = (el: HTMLElement) => {
    el.scrollIntoView({ behavior: 'smooth', block: 'center' })

    let cancelled = false
    const yield_ = () => (cancelled = true)
    const listen = { passive: true, once: true } as const
    window.addEventListener('wheel', yield_, listen)
    window.addEventListener('touchstart', yield_, listen)
    window.addEventListener('keydown', yield_, listen)
    const cleanup = () => {
      window.removeEventListener('wheel', yield_)
      window.removeEventListener('touchstart', yield_)
      window.removeEventListener('keydown', yield_)
    }

    let tries = 0
    const settle = () => {
      if (cancelled || tries++ > SETTLE_TRIES) {
        cleanup()
        return
      }
      // Instant, not smooth: this is a correction, not a journey. Animating each
      // pass would visibly stutter as the layout keeps shifting. Harmless no-op
      // when the page is simply too short to centre the target.
      if (offCenterBy(el) > OFF_CENTER_TOLERANCE) {
        el.scrollIntoView({ block: 'center' })
      }
      setTimeout(settle, SETTLE_INTERVAL)
    }
    // Let the initial smooth scroll finish before measuring it.
    setTimeout(settle, 400)
  }

  const flash = (el: HTMLElement) => {
    scrollToStable(el)
    el.classList.add('kun-comment-flash')
    setTimeout(() => el.classList.remove('kun-comment-flash'), 2000)
  }

  const tryScroll = (postId: number) => {
    const el = document.getElementById(commentAnchorId(postId))
    if (!el) return false
    flash(el)
    return true
  }

  const jumpTo = async (postId: number) => {
    // Reveal it if it is a collapsed reply of an already-loaded root.
    const known = findPost(postId)
    if (known?.root_comment_id) {
      expandedRoots.value.add(known.root_comment_id)
    }
    await nextTick()
    if (tryScroll(postId)) return

    // Otherwise read forward until it arrives. The wall is ordered, so a post
    // that exists is always ahead of where we have read to.
    let guard = 0
    while (!findPost(postId) && hasMore.value && guard++ < LOAD_GUARD) {
      await loadMore()
    }
    const found = findPost(postId)
    if (!found) return
    if (found.root_comment_id) {
      expandedRoots.value.add(found.root_comment_id)
    }
    await nextTick()
    tryScroll(postId)
  }

  // A client-side navigation mounts the wall while its first page is still in
  // flight; jumping then searched an empty list with nothing more to load and
  // gave up, so only a full page load ever landed on the post.
  const loaded = async () => {
    if (!pending.value) return
    await new Promise<void>((resolve) => {
      const stop = watch(pending, (value) => {
        if (value) return
        stop()
        resolve()
      })
    })
  }

  const resolveDeepLink = async () => {
    if (!/^#(post|comment)-\d+$/.test(route.hash)) return
    await loaded()
    const post = route.hash.match(/^#post-(\d+)$/)
    if (post) {
      await jumpTo(Number(post[1]))
      return
    }

    // A link minted before the cutover. Only the server's map table can turn an
    // old comment id into the post that now holds it, and an id that resolves to
    // the OTHER wall is left alone — its page number meant nothing here.
    const legacy = route.hash.match(/^#comment-(\d+)$/)
    if (!legacy) return
    const res = await api
      .get<{
        post_id: number
        resource_id?: number | null
      }>(`/patch/comment/locate?legacy_id=${legacy[1]}`)
      .catch(() => null)
    if (!res || res.code !== 0) return
    if ((res.data.resource_id ?? null) !== surface.value.resourceId) return
    await jumpTo(res.data.post_id)
  }

  onMounted(resolveDeepLink)
  watch(() => route.hash, resolveDeepLink)

  return {
    posts,
    groups,
    total,
    threadId,
    subscription,
    setLevel,
    hasComments,
    hasMore,
    loadMore,
    loadingMore,
    pending,
    expandedRoots,
    toggleExpand,
    onLiked,
    onCommentAdded,
    onEdited,
    onRemoved
  }
}
