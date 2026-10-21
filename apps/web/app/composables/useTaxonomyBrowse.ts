// useTaxonomyBrowse — typed client for the two PUBLIC taxonomy browse pages.
// Game information is edited on the NextMoe editor, never here.
//
// Both calls go through OUR backend proxy (/api/v1/...), which composes the
// page from the catalog and relays {code,message,data} (see
// internal/patch/handler/taxonomy_proxy.go).

// BrowseTag / BrowseOfficial are the PUBLIC taxonomy pages' entities. Their ids
// are CATALOG ids since wave A2-2 (P2 / R1).
export interface BrowseTag {
  id: number
  name: string
  // Always []: the canonical tag vocabulary has no alias table (the wiki's
  // 8,700 aliases were deliberately not migrated). The key stays on the wire so
  // the loss is visible instead of inferred from a missing key.
  aliases: string[]
  // The safety axis. `sexual` is the flag the catalog publishes; `category` is
  // derived from it ('sexual' | 'content') so every consumer that keys on that
  // literal keeps working off one boolean. COVERAGE: an unmapped folksonomy tag
  // reads false, which means "this source has no such axis", NOT "confirmed
  // safe". `tier`/`kind` below are a different coordinate system entirely — not
  // a rename of the wiki's old category.
  sexual: boolean
  category: string
  tier: 'core' | 'longtail' | 'hidden' | string
  kind: 'content' | 'meta' | string
  description: string
  galgame_count: number
  descendant_count: number
  parents: TagRef[]
  ancestors: TagTreeNode[]
  subtags: TagSubtag[]
}

export interface BrowseOfficial {
  id: number
  name: string
  aliases: string[]
  // The catalog's label vocabulary: game_brand | bunko | publisher |
  // anime_studio | doujin_circle | group. It REPLACED the wiki's
  // company|individual|amateur — a visible wording change, and the canonical
  // vocabulary from here on.
  category: string
  lang: string
  link: string
  description: string
  galgame_count: number
  // galgame_count split in two: works attributed to this company directly, and
  // works it only reaches through an imprint or subsidiary. A publisher such as
  // VISUAL ARTS is almost entirely the second kind.
  own_galgame_count: number
  imprint_galgame_count: number
  // The 会社 brand logo as an image_service content HASH (wave 170 P3) — moyu's
  // convention for every catalog-derived image, so the URL is built here with
  // imageServiceUrl and the CDN domain stays in one place. '' / absent = no
  // logo: the header renders its name-only form, which is what every label
  // looked like before this field existed.
  logo_hash?: string
}

export interface BrowseSeries {
  id: number
  name: string
  description: string
  galgame_count: number
  // True when any member sits behind the r18 gate, counted before the reader's
  // own gate narrows the list — an all-r18 series answers a page of zero works
  // to an SFW reader, and this is the only field that can say so.
  has_nsfw: boolean
}

type Q = Record<string, string | number | boolean | undefined>

const qs = (q?: Q): string => {
  if (!q) return ''
  const p = new URLSearchParams()
  for (const [k, v] of Object.entries(q)) {
    if (v !== undefined && v !== '') p.set(k, String(v))
  }
  const s = p.toString()
  return s ? `?${s}` : ''
}

export const useTaxonomyBrowse = () => {
  const api = useApi()

  // ─── Taxonomy detail pages (tag / official "view-by-id" pages) ─────────
  // galgame's `GET /<entity>/:name?<entity>_id=X` returns the entity itself +
  // the associated galgame list (paginated, with optional sort + NSFW filter).
  // `:name` is cosmetic per galgame convention (Wikipedia-style URL beauty);
  // the real filter is the *_id query param. We always pass "_" as the path
  // segment to keep the URL short — moyu's standalone detail pages already
  // own the human-readable URL on their side.
  // docs/galgame_wiki/04-taxonomy.md §标签 (Tag) / 开发商 (Official).
  interface TaxonomyListOpts {
    page?: number
    limit?: number
    sort_field?: string
    sort_order?: 'asc' | 'desc'
    content_limit?: 'sfw' | 'nsfw'
  }
  // Backend (GalgameTaxonomyDetailProxy) rewrites galgame's flat `galgame` brief
  // array into moyu's enriched `GalgameCard` shape so tag/official detail
  // pages can render the same <GalgameCard> as the home / galgame index —
  // the FE no longer has to map between two shapes. Wire field is
  // standardized on `galgames` here.
  const tagDetail = (id: number, opts?: TaxonomyListOpts) =>
    api.get<{
      tag?: BrowseTag
      galgames?: GalgameCard[]
      total?: number
    }>(`/tag/_${qs({ tag_id: id, ...(opts as Q) })}`)

  // `moved_to` arrives ALONE, in place of the record, when this label id was
  // merged away in the catalog: the company lives on under that id and the page
  // 301s there. Never alongside the record — one company must not be reachable
  // at two URLs.
  const officialDetail = (id: number, opts?: TaxonomyListOpts) =>
    api.get<{
      official?: BrowseOfficial
      galgames?: GalgameCard[]
      total?: number
      moved_to?: number
    }>(`/official/_${qs({ official_id: id, ...(opts as Q) })}`)

  const seriesDetail = (id: number, opts?: TaxonomyListOpts) =>
    api.get<{
      series?: BrowseSeries
      galgames?: GalgameCard[]
      total?: number
    }>(`/series/_${qs({ series_id: id, ...(opts as Q) })}`)

  return {
    tagDetail,
    officialDetail,
    seriesDetail
  }
}
