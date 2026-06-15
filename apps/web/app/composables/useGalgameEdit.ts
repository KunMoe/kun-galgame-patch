// useGalgameEdit — typed client for the galgame editing surface that
// docs/galgame_wiki/00-handbook-for-downstream.md §15 makes MANDATORY for moyu:
// revisions, PRs, links/aliases/contributors, and tag/official/engine/series
// CRUD. Every call goes through OUR backend proxy (/api/v1/...), which forwards
// the user's session/access_token to the Wiki Service and relays Wiki's
// {code,message,data} verbatim (see internal/patch/handler/galgame_edit.go).
//
// Wiki owns authorization (creator/admin for revert·merge·decline·PUT galgame;
// admin/moderator for PUT·DELETE taxonomy; any logged-in user for POST
// taxonomy). We do NOT re-check it client-side — on a permission failure the
// backend forwards Wiki's code+message and the caller shows it.

export interface WikiPage<T> {
  items: T[]
  total: number
}

export interface GalgameRevision {
  id: number
  galgame_id: number
  revision: number
  user_id: number
  action: 'created' | 'updated' | 'merged' | 'reverted' | 'declined' | string
  note: string
  is_minor: boolean
  reverted_to: number | null
  created: string
}

// Wiki revision/PR snapshots are an open shape (vndb_id, name_*, intro_*,
// aliases, tag_ids, official_ids, engine_ids, links, ...). Keep it permissive
// and let the diff view iterate keys generically.
export type GalgameSnapshot = Record<string, unknown>

export interface GalgameRevisionDetail extends GalgameRevision {
  snapshot: GalgameSnapshot
}

// K-PR 2026-05-22: diff / PR detail responses now include a `names` map of
// taxonomy id → display name covering every tag / official / engine / series
// referenced in either snapshot. Missing keys = the entity was soft/hard-
// deleted Wiki-side; consumers should fall back to `已删除 #<id>`. Avoids
// the previous N+1 follow-up to resolve display names.
// See docs/galgame_wiki/02-revisions-and-prs.md §names callout.
export interface GalgameDiffNames {
  tags: Record<string, string>
  officials: Record<string, string>
  engines: Record<string, string>
  series: Record<string, string>
}

export interface GalgameDiff {
  changed_keys: Record<string, boolean>
  old: GalgameSnapshot
  new: GalgameSnapshot
  names?: GalgameDiffNames
}

export interface GalgamePR {
  id: number
  galgame_id: number
  user_id: number
  status: 0 | 1 | 2 // 0 pending, 1 merged, 2 declined
  note: string
  base_revision: number
  snapshot: GalgameSnapshot
  completed_by: number | null
  revision_id: number | null
  created: string
}

export interface GalgamePRDetail {
  pr: GalgamePR
  changed_keys: Record<string, boolean>
  // K-PR 2026-05-22: same names map as GalgameDiff.names, covering both
  // the base revision and the PR snapshot. See GalgameDiffNames docstring.
  names?: GalgameDiffNames
}

export interface GalgameLink {
  id: number
  galgame_id: number
  name: string
  link: string
  created: string
  updated: string
}

export interface GalgameAlias {
  id: number
  galgame_id: number
  name: string
  created: string
  updated: string
}

// W3 / Wiki U3 — taxonomy revision (multi-polymorphic single-table on the
// Wiki side; entity column distinguishes tag/official/engine/series). snapshot
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

export interface WikiTag {
  id: number
  name: string
  aliases: string[]
  category: 'content' | 'sexual' | 'technical' | string
  galgame_count: number
}

export interface WikiOfficial {
  id: number
  name: string
  aliases: string[]
  category: 'company' | 'individual' | 'amateur' | string
  lang: string
  link: string
  description: string
  galgame_count: number
}

export interface WikiEngine {
  id: number
  name: string
  description: string
  alias: string[]
}

export interface WikiSeries {
  id: number
  name: string
  description: string
}

// CoverInput / ScreenshotInput mirror docs/galgame_wiki/03-relations.md §封面 /
// 截图 (W2 / Wiki PR5). Same shape used in both responses (PatchDetail.galgame
// .covers / .screenshots) and request payloads — single round trip.
export interface CoverInput {
  image_hash: string
  sort_order: number
  sexual?: number
  violence?: number
  source?: string
  source_key?: string
}
export interface ScreenshotInput extends CoverInput {
  caption?: string
}

// PR / galgame field payload (all optional, replace-all semantics on arrays).
// Presence semantics (docs/galgame_wiki/00-handbook §15 PR2-5): omit a field =
// keep集合 unchanged; `[]` = clear all; non-empty array = authoritative full
// replace — caller MUST resubmit the FULL current set, never deltas.
export interface GalgameEditFields {
  name_en_us?: string
  name_ja_jp?: string
  name_zh_cn?: string
  name_zh_tw?: string
  intro_en_us?: string
  intro_ja_jp?: string
  intro_zh_cn?: string
  intro_zh_tw?: string
  content_limit?: string
  age_limit?: string
  original_language?: string
  release_date?: string | null
  release_date_tba?: boolean
  aliases?: string[]
  tag_ids?: number[]
  official_ids?: number[]
  engine_ids?: number[]
  covers?: CoverInput[]
  screenshots?: ScreenshotInput[]
  links?: { name: string; link: string }[]
  series_id?: number
  note?: string
  is_minor?: boolean
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
  const config = useRuntimeConfig()
  const apiBase = (config.public.apiBase as string) || ''

  // ─── Revisions ──────────────────────────────────────
  const listRevisions = (
    gid: number,
    opts?: { page?: number; limit?: number; include_minor?: boolean }
  ) =>
    api.get<WikiPage<GalgameRevision>>(
      `/galgame/${gid}/revisions${qs(opts as Q)}`
    )

  const getRevision = (gid: number, rev: number) =>
    api.get<GalgameRevisionDetail>(`/galgame/${gid}/revisions/${rev}`)

  const getRevisionDiff = (gid: number, rev: number) =>
    api.get<GalgameDiff>(`/galgame/${gid}/revisions/${rev}/diff`)

  const revert = (gid: number, revision: number) =>
    api.post(`/galgame/${gid}/revert`, { revision })

  // ─── PRs ────────────────────────────────────────────
  const listPRs = (gid: number, opts?: { page?: number; limit?: number }) =>
    api.get<WikiPage<GalgamePR>>(`/galgame/${gid}/prs${qs(opts as Q)}`)

  const getPR = (gid: number, prid: number) =>
    api.get<GalgamePRDetail>(`/galgame/${gid}/prs/${prid}`)

  const submitPR = (gid: number, body: GalgameEditFields) =>
    api.post<GalgamePR>(`/galgame/${gid}/prs`, body as Record<string, unknown>)

  // multipart variant: PR proposal carrying a new banner thumbnail.
  const submitPRMultipart = async (
    gid: number,
    body: GalgameEditFields,
    file: File
  ) => {
    const fd = new FormData()
    fd.append('data', JSON.stringify(body))
    fd.append('file', file, file.name)
    const r = await $fetch
      .raw<{ code: number; message: string; data: unknown }>(
        `${apiBase}/galgame/${gid}/prs`,
        { method: 'POST', body: fd, credentials: 'include' }
      )
      .catch((e) => e?.response)
    return (r?._data ?? {
      code: -1,
      message: '提交失败',
      data: null
    }) as { code: number; message: string; data: unknown }
  }

  // W2 / PR3b — upload a file to image_service via our backend proxy and
  // return the content hash + URLs. Use for screenshots (Wiki accepts no
  // multipart for them) and for any other place that needs a raw image hash.
  // For galgame covers, prefer the PUT /galgame/:gid multipart flow which lets
  // Wiki auto-promote the upload to covers[sort_order=0]; this composable
  // method can still be used to add NON-pinned covers via the JSON path.
  // preset must be allowlisted for our OAuth client on image_service — the
  // GalgameImageUploadPreset union (types/patch.d.ts) is the source of truth:
  //   - 'galgame_screenshot' → galgame screenshots (the only current caller)
  //   - 'topic'              → free-form gallery image (the default; also used
  //                            directly by the editor uploader + admin doc)
  // (galgame_banner is wiki-side via the multipart PUT /galgame/:gid flow;
  // avatars go through OAuth's /auth/me/avatar — neither hits this endpoint.)
  interface ImageServiceUploadResult {
    hash: string
    url: string
    variant_urls: Record<string, string>
    width: number
    height: number
    size_bytes: number
    deduplicated: boolean
  }
  const uploadImageService = async (
    file: File,
    preset: GalgameImageUploadPreset = 'topic'
  ) => {
    const fd = new FormData()
    fd.append('preset', preset)
    fd.append('file', file, file.name)
    const r = await $fetch
      .raw<{ code: number; message: string; data: ImageServiceUploadResult }>(
        `${apiBase}/upload/image-service`,
        { method: 'POST', body: fd, credentials: 'include' }
      )
      .catch((e) => e?.response)
    return (r?._data ?? {
      code: -1,
      message: '上传失败',
      data: null
    }) as {
      code: number
      message: string
      data: ImageServiceUploadResult | null
    }
  }

  const mergePR = (gid: number, prid: number) =>
    api.put(`/galgame/${gid}/prs/${prid}/merge`)

  const declinePR = (gid: number, prid: number) =>
    api.put(`/galgame/${gid}/prs/${prid}/decline`)

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
    api.get<{ items: WikiTag[]; total: number }>(
      `/tag/search${qs({ q, category, limit })}`
    )
  const createTag = (body: {
    name: string
    category: string
    description?: string
    alias?: string[]
  }) => api.post<WikiTag>('/tag', body)
  const updateTag = (body: {
    tag_id: number
    name: string
    category: string
    description?: string
    alias?: string[]
  }) => api.put<WikiTag>('/tag', body)
  // Two-stage safe delete (docs/galgame_wiki/04-taxonomy.md, 00 §15.1):
  // without force, Wiki rejects with code:7 + reference count if the tag is
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
    api.get<{ items: WikiOfficial[]; total: number }>(
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
  }) => api.post<WikiOfficial>('/official', body)
  const updateOfficial = (body: {
    official_id: number
    name: string
    category: string
    link?: string
    lang?: string
    description?: string
    alias?: string[]
  }) => api.put<WikiOfficial>('/official', body)
  const deleteOfficial = (id: number, force = false) =>
    api.delete<{
      deleted: boolean
      forced: boolean
      purged_relations: number
      purged_aliases: number
    }>(`/official/${id}${force ? '?force=true' : ''}`)

  // ─── Taxonomy: engine ───────────────────────────────
  const engineList = () => api.get<WikiEngine[]>('/engine')
  const createEngine = (body: {
    name: string
    description?: string
    alias?: string[]
  }) => api.post<WikiEngine>('/engine', body)
  const updateEngine = (body: {
    engine_id: number
    name: string
    description?: string
    alias?: string[]
  }) => api.put<WikiEngine>('/engine', body)
  // engine has no alias table → response has no purged_aliases.
  const deleteEngine = (id: number, force = false) =>
    api.delete<{
      deleted: boolean
      forced: boolean
      purged_relations: number
    }>(`/engine/${id}${force ? '?force=true' : ''}`)

  // ─── Taxonomy: series ───────────────────────────────
  const seriesList = (opts?: { page?: number; limit?: number }) =>
    api.get<WikiPage<WikiSeries>>(`/series${qs(opts as Q)}`)
  const seriesSearch = (keywords: string) =>
    api.get<unknown[]>(`/series/search${qs({ keywords })}`)
  const seriesDetail = (id: number) => api.get<WikiSeries>(`/series/${id}`)
  const createSeries = (body: {
    name: string
    description?: string
    galgame_ids: number[]
  }) => api.post<WikiSeries>('/series', body)
  const seriesModal = (ids: number[]) =>
    api.post<unknown[]>('/series/modal', { ids })
  const updateSeries = (
    id: number,
    body: { name?: string; description?: string; galgame_ids?: number[] }
  ) => api.put(`/series/${id}`, body)
  const deleteSeries = (id: number) => api.delete(`/series/${id}`)

  // ─── W3 / PR4 — Taxonomy 修订历史 + 回滚（4 实体 × 3 端点 = 12 个方法）─
  // 全部由通用 WikiEditProxy 代理；Wiki 端鉴权（GET 公开、revert 需 admin/
  // moderator）；snapshot 形态因 entity 而异（TagSnapshot / OfficialSnapshot /
  // EngineSnapshot / SeriesSnapshot），UI 层用泛型 Record 展示，无需逐型建模。
  // docs/galgame_wiki/04-taxonomy.md §修订与回滚 + 00-handbook §15.
  type TaxKind = 'tag' | 'official' | 'engine' | 'series'

  const taxListRevisions = (
    kind: TaxKind,
    id: number,
    opts?: { page?: number; limit?: number }
  ) =>
    api.get<WikiPage<TaxonomyRevision>>(
      `/${kind}/${id}/revisions${qs(opts as Q)}`
    )

  const taxGetRevision = (kind: TaxKind, id: number, rev: number) =>
    api.get<TaxonomyRevision>(`/${kind}/${id}/revisions/${rev}`)

  const taxRevert = (kind: TaxKind, id: number, revision: number) =>
    api.post<{ reverted_to: number }>(`/${kind}/${id}/revert`, { revision })

  // ─── Taxonomy detail pages (tag / official "view-by-id" pages) ─────────
  // Wiki's `GET /<entity>/:name?<entity>_id=X` returns the entity itself +
  // the associated galgame list (paginated, with optional sort + NSFW filter).
  // `:name` is cosmetic per Wiki convention (Wikipedia-style URL beauty);
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
  // Backend (WikiTaxonomyDetailProxy) rewrites Wiki's flat `galgame` brief
  // array into moyu's enriched `GalgameCard` shape so tag/official detail
  // pages can render the same <GalgameCard> as the home / galgame index —
  // the FE no longer has to map between two shapes. Wire field is
  // standardized on `galgames` here.
  const tagDetail = (id: number, opts?: TaxonomyListOpts) =>
    api.get<{
      tag?: WikiTag & { description?: string }
      galgames?: GalgameCard[]
      total?: number
    }>(`/tag/_${qs({ tag_id: id, ...(opts as Q) })}`)

  const officialDetail = (id: number, opts?: TaxonomyListOpts) =>
    api.get<{
      official?: WikiOfficial & { description?: string }
      galgames?: GalgameCard[]
      total?: number
    }>(`/official/_${qs({ official_id: id, ...(opts as Q) })}`)

  return {
    listRevisions,
    getRevision,
    getRevisionDiff,
    revert,
    listPRs,
    getPR,
    submitPR,
    submitPRMultipart,
    uploadImageService,
    mergePR,
    declinePR,
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
