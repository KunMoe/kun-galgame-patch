// Builds the dynamic content URLs for the sitemap by enumerating the Go API's
// list endpoints. Consumed by server/api/__sitemap__/urls.ts, which @nuxtjs/sitemap
// pulls in as a `sources` entry (runtime generation, not build-time — the Docker
// build has no Go-API access).
//
// NSFW safety: every moyu list endpoint resolves its filter via the Go side's
// utils.ContentLimitForListBrowse, which DEFAULTS TO sfw when no `content_limit`
// query is present. So we simply DON'T send one — an anonymous crawler gets the
// SEO-safe (SFW) slice and NSFW detail URLs never enter the sitemap. (Unlike the
// forum, which keys NSFW off a cookie and must force it; here the BE default is
// the safe path, so omission is correct and intentional.)
//
// Scale: all page fetches share one global concurrency limiter so a cold render
// can't flood the Go API. Page count is derived from each list's (unfiltered)
// `total`, bounded by MAX_PAGES. The whole thing is cached one layer up.

interface SitemapUrl {
  loc: string
  lastmod?: string
  changefreq?: string
  priority?: number
}

const GLOBAL_CONCURRENCY = 12 // API requests in flight across ALL sources at once
const MAX_PAGES = 400 // per-source ceiling; bounds a cold render against runaway totals

// A bounded queue shared by every apiGet so source-level parallelism can't
// multiply into a request storm against the Go API.
const createLimiter = (max: number) => {
  let active = 0
  const waiters: Array<() => void> = []
  const release = () => {
    active--
    waiters.shift()?.()
  }
  return async <T>(fn: () => Promise<T>): Promise<T> => {
    if (active >= max) await new Promise<void>((resolve) => waiters.push(resolve))
    active++
    try {
      return await fn()
    } finally {
      release()
    }
  }
}

const range = (from: number, to: number): number[] =>
  Array.from({ length: Math.max(0, to - from + 1) }, (_, i) => from + i)

// All moyu list endpoints wrap their payload in the standard envelope
// { code, message, data: ... }; pull the inner data out.
const unwrap = (json: unknown): unknown =>
  (json as { data?: unknown })?.data ?? json

const toIso = (d: unknown): string | undefined => {
  if (typeof d === 'string' || typeof d === 'number' || d instanceof Date) {
    const t = new Date(d)
    if (!Number.isNaN(t.getTime())) return t.toISOString()
  }
  return undefined
}

const num = (row: Record<string, unknown>, key: string) => row[key] as number
const str = (row: Record<string, unknown>, key: string) => row[key] as string

// One paginated list endpoint → sitemap URLs.
//   query:   extra REQUIRED query params (moyu's list DTOs validate:"required"
//            on selected_type / sort_field / sort_order — omitting them is a 400)
//   pageSize: this endpoint's hard `limit` cap (galgame max=24, resource max=50)
//   pick:    pluck the row array from unwrapped data
//   total:   pluck the (unfiltered) total
//   loc:     row → frontend URL
//   lastmod: row → ISO date (optional)
interface PagedSource {
  path: string
  query: string
  pageSize: number
  pick: (data: unknown) => Record<string, unknown>[]
  total: (data: unknown) => number | undefined
  loc: (row: Record<string, unknown>) => string
  lastmod?: (row: Record<string, unknown>) => string | undefined
  priority: number
}

export const buildSitemapUrls = async (
  apiBase: string
): Promise<SitemapUrl[]> => {
  const limit = createLimiter(GLOBAL_CONCURRENCY)

  // apiBase already includes /api/v1 (runtimeConfig.apiBaseSsr / public.apiBase),
  // so `path` is appended verbatim. No content_limit ⇒ BE defaults to sfw.
  const apiGet = (path: string): Promise<unknown | null> =>
    limit(async () => {
      try {
        return await $fetch(`${apiBase}${path}`, {
          headers: { accept: 'application/json' },
          timeout: 15000
        })
      } catch {
        return null // a flaky endpoint drops its rows, never the whole sitemap
      }
    })

  const toUrls = (
    src: PagedSource,
    rows: Record<string, unknown>[]
  ): SitemapUrl[] =>
    rows.map((row) => ({
      loc: src.loc(row),
      lastmod: src.lastmod?.(row),
      changefreq: 'daily',
      priority: src.priority
    }))

  const fetchPage = async (src: PagedSource, page: number) => {
    const body = await apiGet(
      `${src.path}?${src.query}&page=${page}&limit=${src.pageSize}`
    )
    return body ? src.pick(unwrap(body)) : []
  }

  const collect = async (src: PagedSource): Promise<SitemapUrl[]> => {
    const firstBody = await apiGet(
      `${src.path}?${src.query}&page=1&limit=${src.pageSize}`
    )
    if (!firstBody) return []
    const data = unwrap(firstBody)
    const urls = toUrls(src, src.pick(data))

    const total = src.total(data)
    const pages =
      typeof total === 'number' && total > 0
        ? Math.min(Math.ceil(total / src.pageSize), MAX_PAGES)
        : 1
    const rest = await Promise.all(
      range(2, pages).map((p) => fetchPage(src, p).then((r) => toUrls(src, r)))
    )
    for (const r of rest) urls.push(...r)
    return urls
  }

  // Single-GET list (no pagination): the doc post index.
  const collectSingle = async (
    path: string,
    pick: (data: unknown) => Record<string, unknown>[],
    loc: (row: Record<string, unknown>) => string,
    lastmod: (row: Record<string, unknown>) => string | undefined,
    priority: number
  ): Promise<SitemapUrl[]> => {
    const body = await apiGet(path)
    if (!body) return []
    return pick(unwrap(body)).map((row) => ({
      loc: loc(row),
      lastmod: lastmod(row),
      changefreq: 'daily',
      priority
    }))
  }

  const paged: PagedSource[] = [
    {
      // Galgame browse → patch detail. /patch/:id 302-redirects to
      // /patch/:id/introduction, so emit the final URL directly.
      path: '/galgame',
      query: 'selected_type=all&sort_field=created&sort_order=desc',
      pageSize: 24,
      pick: (d) =>
        ((d as { galgames?: [] })?.galgames ?? []) as Record<string, unknown>[],
      total: (d) => (d as { total?: number })?.total,
      loc: (r) => `/patch/${num(r, 'id')}/introduction`,
      lastmod: (r) => toIso(r.resource_update_time ?? r.created),
      priority: 0.8
    },
    {
      // Global resources → standalone resource detail page.
      path: '/resource',
      query: 'sort_field=created&sort_order=desc',
      pageSize: 50,
      pick: (d) =>
        ((d as { items?: [] })?.items ?? []) as Record<string, unknown>[],
      total: (d) => (d as { total?: number })?.total,
      loc: (r) => `/resource/${num(r, 'id')}`,
      lastmod: (r) => toIso(r.update_time ?? r.created),
      priority: 0.6
    }
  ]

  const groups = await Promise.all([
    ...paged.map((src) => collect(src)),
    // Docs: GET /doc/posts → { items: [{ slug, date, ... }], tree }. slug is
    // already "<category>/<name>", matching the /doc/[...slug] route.
    collectSingle(
      '/doc/posts',
      (d) => ((d as { items?: [] })?.items ?? []) as Record<string, unknown>[],
      (r) => `/doc/${str(r, 'slug')}`,
      (r) => toIso(r.date),
      0.7
    )
  ])

  // De-dup defensively (an id shouldn't repeat, but never emit a dup <url>).
  const seen = new Set<string>()
  const out: SitemapUrl[] = []
  for (const url of groups.flat()) {
    if (seen.has(url.loc)) continue
    seen.add(url.loc)
    out.push(url)
  }
  return out
}
