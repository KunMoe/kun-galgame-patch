// The comment-area read + its optimistic mutation handlers, lifted out of the
// components that render it.
//
// Lifted for the same reason as useResourceRevisions: the resource detail page
// needs the COUNT before it can lay itself out (the 评论 tab is labelled with
// it), while the rendering lives in CommentSection. Fetching inside the section
// and emitting the count back up would settle the tab label one tick after the
// parent first rendered — a visible flicker. The page awaits this instead.
//
// The wall is read as pages of top-level comments, each carrying its first few
// replies and a count of the rest, which load per comment.
import {
  commentAnchorId,
  commentSurface,
  type CommentTarget
} from '~/shared/utils/commentTarget'

export const COMMENT_LIMIT = 30
const REPLY_PAGE_LIMIT = 20
// The server sends this many replies of every root with the wall.
export const REPLY_PREVIEW = 3

export type CommentSort = 'oldest' | 'newest' | 'top'

export interface CommentGroup {
  root: PatchPageComment
  replies: PatchPageComment[]
  // Visible replies the reader has not loaded yet.
  remaining: number
}

export const useCommentList = (target: Ref<CommentTarget> | CommentTarget) => {
  const api = useApi()
  const route = useRoute()
  const userStore = useUserStore()

  const resolved = computed(() => unref(target))
  const surface = computed(() => commentSurface(resolved.value))

  const sort = ref<CommentSort>('oldest')
  const subscription = ref<CommentWallState | null>(null)
  const loadingMore = ref(false)
  const loadingReplies = ref<Set<number>>(new Set())
  const postedHere = new Set<number>()
  // Roots whose last reply page has been read. reply_count is a count taken
  // with the page, so it can promise replies that are no longer there.
  const exhausted = ref<Set<number>>(new Set())

  const emptyPage = (): PatchCommentPage => ({
    thread_id: 0,
    pinned: [],
    posts: [],
    replies: [],
    next_cursor: '',
    total: 0
  })

  const pageUrl = (cursor = '') => {
    const q = new URLSearchParams({
      sort: sort.value,
      limit: String(COMMENT_LIMIT)
    })
    if (cursor) q.set('cursor', cursor)
    return `${surface.value.listUrl}?${q}`
  }

  // The wall IS `data`: loadMore and the mutation handlers below write into it.
  // A separate ref seeded from it stays empty on the server, which runs no
  // watchers, so every wall rendered empty there and hydrated a full list over
  // it.
  const { data, pending } = useAsyncData<PatchCommentPage>(
    () => `comments-${surface.value.listUrl}-${sort.value}`,
    async () => {
      const res = await api.get<PatchCommentPage>(pageUrl())
      return res.code === 0 ? res.data : emptyPage()
    },
    {
      default: emptyPage,
      // Watch the listUrl STRING, not `surface` — that computed builds a fresh
      // object on every evaluation, so its identity always differs and the
      // watcher would refire on any unrelated recompute of the target. On the
      // resource page `target` is a computed over the deep-reactive `detail`, so
      // bumping the download counter alone would have refetched the comments.
      watch: [() => surface.value.listUrl, sort],
      // Nuxt 4 makes `data` a shallowRef; onLiked flips fields on a post in
      // place.
      deep: true
    }
  )

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

  const mergeReplies = (incoming: PatchPageComment[]) => {
    const page = data.value
    const seen = new Set(page.replies.map((p) => p.id))
    page.replies = [...page.replies, ...incoming.filter((p) => !seen.has(p.id))]
  }

  const loadMore = async () => {
    if (!hasMore.value || loadingMore.value) return
    loadingMore.value = true
    try {
      const page = data.value
      const res = await api.get<PatchCommentPage>(pageUrl(page.next_cursor))
      if (res.code !== 0) return
      const seen = new Set(page.posts.map((p) => p.id))
      page.posts = [
        ...page.posts,
        ...res.data.posts.filter((p) => !seen.has(p.id))
      ]
      mergeReplies(res.data.replies)
      page.next_cursor = res.data.next_cursor
      page.total = res.data.total
    } finally {
      loadingMore.value = false
    }
  }

  const repliesByRoot = computed(() => {
    const map = new Map<number, PatchPageComment[]>()
    for (const r of data.value.replies) {
      const rootId = r.root_comment_id
      if (rootId == null) continue
      const list = map.get(rootId)
      if (list) list.push(r)
      else map.set(rootId, [r])
    }
    for (const list of map.values()) {
      list.sort((a, b) => a.post_number - b.post_number)
    }
    return map
  })

  const groupOf = (root: PatchPageComment): CommentGroup => {
    const replies = repliesByRoot.value.get(root.id) ?? []
    const loadedVisible = replies.filter((r) => r.status === 0).length
    return {
      root,
      replies,
      remaining: exhausted.value.has(root.id)
        ? 0
        : Math.max(0, (root.reply_count ?? 0) - loadedVisible)
    }
  }

  // A pinned comment also comes back at its own place in the list, so it is
  // drawn once, on top.
  const pinnedGroups = computed(() => data.value.pinned.map(groupOf))
  const groups = computed<CommentGroup[]>(() => {
    const pinned = new Set(data.value.pinned.map((p) => p.id))
    return data.value.posts.filter((p) => !pinned.has(p.id)).map(groupOf)
  })

  const hasComments = computed(
    () => pinnedGroups.value.length > 0 || groups.value.length > 0
  )

  const loadReplies = async (rootId: number) => {
    if (loadingReplies.value.has(rootId)) return false
    loadingReplies.value.add(rootId)
    try {
      // A reply posted from this page is the newest of its root, so paging
      // after it would skip every reply between the loaded ones and it.
      const after = Math.max(
        0,
        ...(repliesByRoot.value.get(rootId) ?? [])
          .filter((r) => !postedHere.has(r.id))
          .map((r) => r.post_number)
      )
      const res = await api.get<PatchCommentReplies>(
        `/patch/comment/${rootId}/replies?after=${after}&limit=${REPLY_PAGE_LIMIT}`
      )
      if (res.code !== 0) {
        useKunMessage(res.message || '加载回复失败', 'error')
        return false
      }
      mergeReplies(res.data.posts)
      expandedRoots.value.add(rootId)
      if (!res.data.next_cursor) exhausted.value.add(rootId)
      return true
    } finally {
      loadingReplies.value.delete(rootId)
    }
  }

  // ─── optimistic mutation handlers ───────────────────
  // A pinned root is two objects, one in `pinned` and one in `posts`.
  const copiesOf = (id: number) => {
    const page = data.value
    return [...page.pinned, ...page.posts, ...page.replies].filter(
      (p) => p.id === id
    )
  }
  const findPost = (id: number) => copiesOf(id)[0]

  const bumpReplyCount = (rootId: number | null, delta: number) => {
    if (rootId == null) return
    for (const root of copiesOf(rootId)) {
      root.reply_count = Math.max(0, (root.reply_count ?? 0) + delta)
    }
  }

  const replace = (updated: PatchPageComment) => {
    for (const p of copiesOf(updated.id)) {
      Object.assign(p, {
        ...updated,
        reply_count: p.reply_count,
        pinned_at: p.pinned_at
      })
    }
  }

  const onLiked = (id: number, liked: boolean) => {
    for (const p of copiesOf(id)) {
      if (p.is_liked === liked) continue
      p.like_count = Math.max(0, p.like_count + (liked ? 1 : -1))
      p.is_liked = liked
    }
  }

  const onCommentAdded = (comment: PatchPageComment) => {
    const page = data.value
    if (findPost(comment.id)) return
    if (comment.root_comment_id == null) {
      const root = { ...comment, reply_count: 0 }
      page.posts =
        sort.value === 'oldest' ? [...page.posts, root] : [root, ...page.posts]
    } else {
      postedHere.add(comment.id)
      page.replies = [...page.replies, comment]
      if (comment.status === 0) bumpReplyCount(comment.root_comment_id, 1)
      expandedRoots.value.add(comment.root_comment_id)
    }
    page.total += 1
    // A new thread id reaches reportRead through its watcher.
    if (!page.thread_id && comment.thread_id) {
      page.thread_id = comment.thread_id
    } else {
      reportRead()
    }
  }

  // The edit face answers no reply count or pin, so those stay as they were.
  const onEdited = (updated: PatchPageComment) => replace(updated)

  // A delete is a tombstone upstream: the post keeps its number so the wall's
  // numbering never collapses. Removing the row here instead would make replies
  // under it look orphaned until the next read. `total` is the thread's
  // posts_count, which counts the stub too; decrementing it here showed one
  // fewer comment than the same wall after a reload.
  const onRemoved = (id: number, source: CommentDeleteSource) => {
    const page = data.value
    const post = findPost(id)
    if (!post || post.deleted) return
    if (post.status === 0) bumpReplyCount(post.root_comment_id, -1)
    for (const p of copiesOf(id)) {
      Object.assign(p, {
        deleted: true,
        held: false,
        status: 2,
        delete_source: source,
        content: '',
        content_html: ''
      })
    }
    page.pinned = page.pinned.filter((p) => p.id !== id)
  }

  // A post a moderator deleted while it was held comes back held, and a held
  // post is its author's alone to see.
  const onRestored = (updated: PatchPageComment) => {
    const post = findPost(updated.id)
    if (updated.held && updated.user?.id !== userStore.user.id) {
      const page = data.value
      const keep = (p: PatchPageComment) => p.id !== updated.id
      page.pinned = page.pinned.filter(keep)
      page.posts = page.posts.filter(keep)
      page.replies = page.replies.filter(keep)
      return
    }
    if (post?.deleted && updated.status === 0) {
      bumpReplyCount(post.root_comment_id, 1)
    }
    replace(updated)
  }

  const onPinned = (updated: PatchPageComment) => {
    const page = data.value
    for (const p of copiesOf(updated.id)) p.pinned_at = updated.pinned_at
    if (!updated.pinned_at) {
      page.pinned = page.pinned.filter((p) => p.id !== updated.id)
      return
    }
    if (page.pinned.some((p) => p.id === updated.id)) return
    const current = findPost(updated.id)
    page.pinned = [
      { ...(current ?? updated), pinned_at: updated.pinned_at },
      ...page.pinned
    ]
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

  const LOAD_GUARD = 50

  const onThisWall = (c: PatchPageComment) => {
    const resourceId = surface.value.resourceId
    return resourceId === null
      ? c.resource_id == null && c.galgame_id === resolved.value.galgameId
      : c.resource_id === resourceId
  }

  // Neither newest nor top can be searched for a post, so a deep link reads the
  // wall oldest-first until the post's root arrives, then that root's replies.
  const load = async (postId: number) => {
    const res = await api
      .get<PatchPageComment>(`/patch/comment/${postId}`)
      .catch(() => null)
    if (!res || res.code !== 0 || !onThisWall(res.data)) return
    const rootId = res.data.root_comment_id ?? res.data.id

    if (!findPost(rootId) && sort.value !== 'oldest') {
      sort.value = 'oldest'
      await nextTick()
      await loaded()
    }
    let guard = 0
    while (!findPost(rootId) && hasMore.value && guard++ < LOAD_GUARD) {
      await loadMore()
    }
    const root = findPost(rootId)
    if (!root || rootId === postId) return
    guard = 0
    while (
      !findPost(postId) &&
      groupOf(root).remaining > 0 &&
      guard++ < LOAD_GUARD
    ) {
      if (!(await loadReplies(rootId))) break
    }
  }

  const jumpTo = async (postId: number) => {
    if (!findPost(postId)) await load(postId)
    const found = findPost(postId)
    if (!found) return
    if (found.root_comment_id) expandedRoots.value.add(found.root_comment_id)
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
    sort,
    pinnedGroups,
    groups,
    total,
    threadId,
    subscription,
    setLevel,
    hasComments,
    hasMore,
    loadMore,
    loadingMore,
    loadReplies,
    loadingReplies,
    pending,
    expandedRoots,
    toggleExpand,
    onLiked,
    onCommentAdded,
    onEdited,
    onRemoved,
    onRestored,
    onPinned
  }
}

export type CommentWall = ReturnType<typeof useCommentList>
