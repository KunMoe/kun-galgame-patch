// useGalgameEdit — typed client for the galgame taxonomy + relation surface
// moyu still proxies to the galgame service: links/aliases relations and
// tag/official/engine/series CRUD (incl. taxonomy revision history + revert).
// Galgame metadata editing (revision history, edit-request PRs, direct edit)
// moved to kungal in the "编辑面归 kungal" wave. Every call goes through OUR
// backend proxy (/api/v1/...), which forwards the user's session/access_token
// to the galgame service and relays galgame's {code,message,data} verbatim (see
// internal/patch/handler/galgame_edit.go).
//
// galgame owns authorization (admin/moderator for PUT·DELETE taxonomy + revert;
// any logged-in user for POST taxonomy). We do NOT re-check it client-side — on
// a permission failure the backend forwards galgame's code+message and the caller
// shows it.

export interface GalgamePage<T> {
  items: T[]
  total: number
}

// galgame-proxied relation shapes are aliased to the generated OpenAPI schemas
// (shared/types/galgame-wiki.ts) so a backend wire change fails the drift gate
// + tsc here instead of breaking at runtime. Re-exported so the relation
// surface keeps importing them from this one composable.
export type { GalgameLink, GalgameAlias } from '~/shared/types/galgame-wiki'

// W3 / galgame U3 — taxonomy revision (multi-polymorphic single-table on the
// galgame side; entity column distinguishes tag/official/engine/series). snapshot
// shape varies per entity; we render it generically as Record<string, unknown>.
// See docs/galgame_wiki/04-taxonomy.md §修订与回滚.
export interface TaxonomyRevision {
  id: number
  entity: 'tag' | 'official' | 'engine' | 'series'
  target_id: number
  revision: number
  action: 'created' | 'updated' | 'deleted' | 'reverted' | string
  user_id: number
  user_role: number
  snapshot: Record<string, unknown>
  changed_fields: string[]
  // `deleted` rows only:
  ref_count?: number
  affected_galgame_ids?: number[]
  note: string
  created: string
}

export interface GalgameTag {
  id: number
  name: string
  aliases: string[]
  category: 'content' | 'sexual' | 'technical' | string
  galgame_count: number
}

export interface GalgameOfficial {
  id: number
  name: string
  aliases: string[]
  category: 'company' | 'individual' | 'amateur' | string
  lang: string
  link: string
  description: string
  galgame_count: number
}

export interface GalgameEngine {
  id: number
  name: string
  description: string
  alias: string[]
}

export interface GalgameSeries {
  id: number
  name: string
  description: string
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

export const useGalgameEdit = () => {
  const api = useApi()

  // ─── Relations ──────────────────────────────────────
  const listLinks = (gid: number) =>
    api.get<GalgameLink[]>(`/galgame/${gid}/links`)
  const createLink = (gid: number, body: { name: string; link: string }) =>
    api.post(`/galgame/${gid}/links`, body)
  const deleteLink = (gid: number, id: number) =>
    api.delete(`/galgame/${gid}/links`, { id })

  const listAliases = (gid: number) =>
    api.get<GalgameAlias[]>(`/galgame/${gid}/aliases`)
  const createAlias = (gid: number, name: string) =>
    api.post(`/galgame/${gid}/aliases`, { name })
  const deleteAlias = (gid: number, id: number) =>
    api.delete(`/galgame/${gid}/aliases`, { id })

  // ─── Taxonomy: tag ──────────────────────────────────
  const tagSearch = (q: string, category?: string, limit = 30) =>
    api.get<{ items: GalgameTag[]; total: number }>(
      `/tag/search${qs({ q, category, limit })}`
    )
  const createTag = (body: {
    name: string
    category: string
    description?: string
    alias?: string[]
  }) => api.post<GalgameTag>('/tag', body)
  const updateTag = (body: {
    tag_id: number
    name: string
    category: string
    description?: string
    alias?: string[]
  }) => api.put<GalgameTag>('/tag', body)
  // Two-stage safe delete (docs/galgame_wiki/04-taxonomy.md, 00 §15.1):
  // without force, galgame rejects with code:7 + reference count if the tag is
  // still used; force=true cascades. Same for official/engine.
  const deleteTag = (id: number, force = false) =>
    api.delete<{
      deleted: boolean
      forced: boolean
      purged_relations: number
      purged_aliases: number
    }>(`/tag/${id}${force ? '?force=true' : ''}`)

  // ─── Taxonomy: official ─────────────────────────────
  const officialSearch = (
    q: string,
    category?: string,
    lang?: string,
    limit = 30
  ) =>
    api.get<{ items: GalgameOfficial[]; total: number }>(
      `/official/search${qs({ q, category, lang, limit })}`
    )
  const createOfficial = (body: {
    name: string
    category: string
    original?: string
    link?: string
    lang?: string
    description?: string
    alias?: string[]
  }) => api.post<GalgameOfficial>('/official', body)
  const updateOfficial = (body: {
    official_id: number
    name: string
    category: string
    link?: string
    lang?: string
    description?: string
    alias?: string[]
  }) => api.put<GalgameOfficial>('/official', body)
  const deleteOfficial = (id: number, force = false) =>
    api.delete<{
      deleted: boolean
      forced: boolean
      purged_relations: number
      purged_aliases: number
    }>(`/official/${id}${force ? '?force=true' : ''}`)

  // ─── Taxonomy: engine ───────────────────────────────
  const engineList = () => api.get<GalgameEngine[]>('/engine')
  const createEngine = (body: {
    name: string
    description?: string
    alias?: string[]
  }) => api.post<GalgameEngine>('/engine', body)
  const updateEngine = (body: {
    engine_id: number
    name: string
    description?: string
    alias?: string[]
  }) => api.put<GalgameEngine>('/engine', body)
  // engine has no alias table → response has no purged_aliases.
  const deleteEngine = (id: number, force = false) =>
    api.delete<{
      deleted: boolean
      forced: boolean
      purged_relations: number
    }>(`/engine/${id}${force ? '?force=true' : ''}`)

  // ─── Taxonomy: series ───────────────────────────────
  const seriesList = (opts?: { page?: number; limit?: number }) =>
    api.get<GalgamePage<GalgameSeries>>(`/series${qs(opts as Q)}`)
  const seriesSearch = (keywords: string) =>
    api.get<unknown[]>(`/series/search${qs({ keywords })}`)
  const seriesDetail = (id: number) => api.get<GalgameSeries>(`/series/${id}`)
  const createSeries = (body: {
    name: string
    description?: string
    galgame_ids: number[]
  }) => api.post<GalgameSeries>('/series', body)
  const seriesModal = (ids: number[]) =>
    api.post<unknown[]>('/series/modal', { ids })
  const updateSeries = (
    id: number,
    body: { name?: string; description?: string; galgame_ids?: number[] }
  ) => api.put(`/series/${id}`, body)
  const deleteSeries = (id: number) => api.delete(`/series/${id}`)

  // ─── W3 / PR4 — Taxonomy 修订历史 + 回滚（4 实体 × 3 端点 = 12 个方法）─
  // 全部由通用 GalgameEditProxy 代理；galgame 端鉴权（GET 公开、revert 需 admin/
  // moderator）；snapshot 形态因 entity 而异（TagSnapshot / OfficialSnapshot /
  // EngineSnapshot / SeriesSnapshot），UI 层用泛型 Record 展示，无需逐型建模。
  // docs/galgame_wiki/04-taxonomy.md §修订与回滚 + 00-handbook §15.
  type TaxKind = 'tag' | 'official' | 'engine' | 'series'

  const taxListRevisions = (
    kind: TaxKind,
    id: number,
    opts?: { page?: number; limit?: number }
  ) =>
    api.get<GalgamePage<TaxonomyRevision>>(
      `/${kind}/${id}/revisions${qs(opts as Q)}`
    )

  const taxGetRevision = (kind: TaxKind, id: number, rev: number) =>
    api.get<TaxonomyRevision>(`/${kind}/${id}/revisions/${rev}`)

  const taxRevert = (kind: TaxKind, id: number, revision: number) =>
    api.post<{ reverted_to: number }>(`/${kind}/${id}/revert`, { revision })

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
      tag?: GalgameTag & { description?: string }
      galgames?: GalgameCard[]
      total?: number
    }>(`/tag/_${qs({ tag_id: id, ...(opts as Q) })}`)

  const officialDetail = (id: number, opts?: TaxonomyListOpts) =>
    api.get<{
      official?: GalgameOfficial & { description?: string }
      galgames?: GalgameCard[]
      total?: number
    }>(`/official/_${qs({ official_id: id, ...(opts as Q) })}`)

  return {
    listLinks,
    createLink,
    deleteLink,
    listAliases,
    createAlias,
    deleteAlias,
    tagSearch,
    createTag,
    updateTag,
    deleteTag,
    officialSearch,
    createOfficial,
    updateOfficial,
    deleteOfficial,
    engineList,
    createEngine,
    updateEngine,
    deleteEngine,
    seriesList,
    seriesSearch,
    seriesDetail,
    createSeries,
    seriesModal,
    updateSeries,
    deleteSeries,
    taxListRevisions,
    taxGetRevision,
    taxRevert,
    tagDetail,
    officialDetail
  }
}
