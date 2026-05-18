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

export interface GalgameDiff {
  changed_keys: Record<string, boolean>
  old: GalgameSnapshot
  new: GalgameSnapshot
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

export interface GalgameContributor {
  id: number
  galgame_id: number
  user_id: number
  created: string
  user?: { id: number; name: string; avatar: string }
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

// PR / galgame field payload (all optional, replace-all semantics on arrays).
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
  aliases?: string[]
  tag_ids?: number[]
  official_ids?: number[]
  engine_ids?: number[]
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

  const listContributors = (gid: number) =>
    api.get<GalgameContributor[]>(`/galgame/${gid}/contributors`)
  const deleteContributor = (gid: number, uid: number) =>
    api.delete(`/galgame/${gid}/contributors/${uid}`)

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

  return {
    listRevisions,
    getRevision,
    getRevisionDiff,
    revert,
    listPRs,
    getPR,
    submitPR,
    submitPRMultipart,
    mergePR,
    declinePR,
    listLinks,
    createLink,
    deleteLink,
    listAliases,
    createAlias,
    deleteAlias,
    listContributors,
    deleteContributor,
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
    deleteSeries
  }
}
