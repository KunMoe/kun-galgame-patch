// The comment-area read + its optimistic mutation handlers, lifted out of the
// components that render it.
//
// Lifted for the same reason as useResourceRevisions: the resource detail page
// needs the COUNT before it can lay itself out (the 评论 tab is labelled with
// it), while the rendering lives in CommentSection. Fetching inside the section
// and emitting the count back up would settle the tab label one tick after the
// parent first rendered — a visible flicker. The page awaits this instead.
//
// It also owns the mutation handlers, because they patch the very array it
// holds: every CommentRow emits its RESULT and this applies it in place, so no
// refetch and no loading flash.
import {
  commentSurface,
  type CommentTarget
} from '~/shared/utils/commentTarget'

export const COMMENT_LIMIT = 30

interface CommentListResponse {
  items: PatchPageComment[]
  total: number
}

interface Options {
  // When set, the current page lives in the URL under this query key, so
  // back-nav and shared links restore it. Used by the game page's 评论 tab,
  // which pages alongside its own ?tab= rather than clobbering it. Omitted for
  // an area embedded in a page whose URL already means something else (the
  // resource detail tabs) — there the page is a plain ref and paging leaves the
  // URL alone.
  routeQueryKey?: string
}

export const useCommentList = (
  target: Ref<CommentTarget> | CommentTarget,
  options: Options = {}
) => {
  const api = useApi()
  const route = useRoute()
  const router = useRouter()

  const resolved = computed(() => unref(target))
  const surface = computed(() => commentSurface(resolved.value))

  const queryKey = options.routeQueryKey
  const page = queryKey
    ? computed({
        get: () => Number(route.query[queryKey]) || 1,
        set: (v: number) => {
          router.push({
            query: { ...route.query, [queryKey]: v },
            hash: route.hash
          })
        }
      })
    : ref(1)

  // Not awaited, so this stays a plain (non-async) composable that callers can
  // use without threading another await through their setup. Nuxt resolves the
  // pending fetch before it renders on the server either way, so `total` is
  // already correct on the first paint — which is what the resource page's tab
  // label depends on.
  const { data, pending } = useAsyncData<CommentListResponse>(
    () => `comments-${surface.value.listUrl}-${page.value}`,
    async () => {
      const res = await api.get<CommentListResponse>(
        `${surface.value.listUrl}?page=${page.value}&limit=${COMMENT_LIMIT}`
      )
      return res.code === 0 ? res.data : { items: [], total: 0 }
    },
    {
      default: () => ({ items: [], total: 0 }),
      // Watch the listUrl STRING, not `surface` — that computed builds a fresh
      // object on every evaluation, so its identity always differs and the
      // watcher would refire on any unrelated recompute of the target. On the
      // resource page `target` is a computed over the deep-reactive `detail`, so
      // bumping the download counter alone would have refetched the comments.
      watch: [page, () => surface.value.listUrl],
      // Nuxt 4 makes `data` a shallowRef by default, so the optimistic handlers
      // below (unshift/splice/push + nested like_count/content edits on
      // data.value.items) wouldn't trigger a re-render. deep:true restores the
      // deep reactivity those in-place mutations rely on.
      deep: true
    }
  )

  const items = computed(() => data.value?.items ?? [])
  // total = APPROVED ROOT-comment count (the server paginates over roots), so it
  // drives the paginator directly. Note it is NOT the total number of comments —
  // replies aren't in it.
  const total = computed(() => data.value?.total ?? 0)
  const totalPages = computed(() => Math.max(1, Math.ceil(total.value / COMMENT_LIMIT)))
  const hasComments = computed(() => total.value > 0)

  // ─── optimistic mutation handlers ───────────────────
  const findComment = (id: number): PatchPageComment | undefined => {
    for (const c of items.value) {
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

  // A brand-new root comment from the area's own composer.
  const onCommentAdded = (comment: PatchPageComment) => {
    if (!data.value) return
    data.value.items.unshift(comment)
    data.value.total++
  }

  const onReplyAdded = (reply: PatchPageComment) => {
    if (!data.value) return
    // A reply always attaches to a ROOT (parent_id = root id) — one tier.
    const root = data.value.items.find((c) => c.id === reply.parent_id)
    if (!root) return
    if (!root.reply) root.reply = []
    root.reply.push(reply)
    // Expand the thread so the just-posted reply is visible even when it lands
    // past the inline preview (otherwise it'd hide behind "展开更多").
    expandedRoots.value.add(root.id)
    // total = root count → a reply doesn't change it (keeps totalPages correct).
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

  // ─── inline thread expansion ────────────────────────
  // Root ids whose replies are fully expanded in place (a Set so several threads
  // can be open at once; a deep-link jump adds the target's root here). Vue 3
  // proxies Set mutations, so add/delete are reactive.
  const expandedRoots = ref<Set<number>>(new Set())
  const toggleExpand = (rootId: number) => {
    if (expandedRoots.value.has(rootId)) expandedRoots.value.delete(rootId)
    else expandedRoots.value.add(rootId)
  }

  // ─── deep-link: jump to a specific comment, across pages ──
  // Links (notifications / home / the global feed) point at
  // <area>#comment-:cid. Try the current page first; if the target isn't here,
  // ask the server which page it's on (GET /patch/comment/:id/locate), go
  // there, then scroll. A collapsed reply (only the first few show inline) is
  // revealed by expanding its thread. Once found, scroll + flash it.
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

  const tryScroll = (id: number) => {
    const el = document.getElementById(`comment-${id}`)
    if (el) {
      flash(el)
      return true
    }
    return false
  }

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

  const resolveDeepLink = async () => {
    const m = route.hash.match(/^#comment-(\d+)$/)
    if (!m) return
    const id = Number(m[1])
    await nextTick()
    if (tryScroll(id)) return

    const res = await api
      .get<{
        page: number
        root_id: number
        is_reply: boolean
        resource_id?: number
      }>(`/patch/comment/${id}/locate?limit=${COMMENT_LIMIT}`)
      .catch(() => null)
    if (!res || res.code !== 0) return
    const {
      page: targetPage,
      root_id: rootId,
      is_reply: isReply,
      resource_id: resourceId
    } = res.data

    // The comment may belong to the OTHER comment area (a resource comment while
    // we're the patch tab, or vice versa). Its page number is computed within
    // its own listing, so applying it here would jump to a page the comment
    // isn't on — leave the anchor unresolved instead.
    if ((resourceId ?? null) !== surface.value.resourceId) return

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

  onMounted(resolveDeepLink)
  watch(() => route.hash, resolveDeepLink)

  return {
    items,
    total,
    totalPages,
    hasComments,
    pending,
    page,
    expandedRoots,
    toggleExpand,
    onLiked,
    onCommentAdded,
    onReplyAdded,
    onEdited,
    onRemoved
  }
}
