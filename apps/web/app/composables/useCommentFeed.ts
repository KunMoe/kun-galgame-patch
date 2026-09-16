// The mixed comment lists — the global feed, a profile, search, the admin queue
// — all read the same keyset face: a page of rows plus an opaque cursor, with no
// total and therefore no page count. They load more instead of paginating.
export const useCommentFeed = (
  url: Ref<string> | string,
  options: { limit?: number; key?: string; immediate?: boolean } = {}
) => {
  const api = useApi()
  const limit = options.limit ?? 20

  const loadingMore = ref(false)

  const resolvedUrl = computed(() => unref(url))

  const query = (cursor: string) => {
    const joiner = resolvedUrl.value.includes('?') ? '&' : '?'
    const params = new URLSearchParams({ limit: String(limit) })
    if (cursor) params.set('cursor', cursor)
    return `${resolvedUrl.value}${joiner}${params.toString()}`
  }

  // The list IS `data`, extended in place by loadMore. A separate ref seeded
  // from it stays empty on the server, which runs no watchers, and every feed
  // hydrated a full list over an empty one.
  const { data, pending, refresh } = useAsyncData<PatchCommentFeed>(
    () => options.key ?? `comment-feed-${resolvedUrl.value}`,
    async () => {
      const res = await api.get<PatchCommentFeed>(query(''))
      return res.code === 0 ? res.data : { items: [], next_cursor: '' }
    },
    {
      default: () => ({ items: [], next_cursor: '' }),
      immediate: options.immediate ?? true,
      watch: [resolvedUrl]
    }
  )

  const items = computed(() => data.value.items)
  const hasMore = computed(() => data.value.next_cursor !== '')

  const loadMore = async () => {
    if (!hasMore.value || loadingMore.value) return
    loadingMore.value = true
    try {
      const res = await api.get<PatchCommentFeed>(query(data.value.next_cursor))
      if (res.code !== 0) return
      // The upstream cursor is stable, but a row whose anchor this site cannot
      // link to is dropped server-side, so a page can come back short — and a
      // retry must not duplicate what is already on screen.
      const seen = new Set(data.value.items.map((i) => i.id))
      data.value = {
        items: [
          ...data.value.items,
          ...res.data.items.filter((i) => !seen.has(i.id))
        ],
        next_cursor: res.data.next_cursor
      }
    } finally {
      loadingMore.value = false
    }
  }

  return { items, hasMore, loadMore, loadingMore, pending, refresh }
}
