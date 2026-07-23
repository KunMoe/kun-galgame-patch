// Hand-owned wire shapes of the galgame revision / PR surfaces moyu consumes
// through its BFF proxy (the surviving /internal platform workflow face plus
// the wiki-proxied edit surface).
//
// History: these used to be friendly aliases over ./generated/galgame-read-api.ts,
// generated from the published galgame-wiki OpenAPI spec with an
// openapi-typescript CI drift gate. That spec (and its portal publish) was
// retired with the bridge read face in open-API Phase 2 route-B W5 (2026-07-23),
// so the generated arm is gone: these definitions are now the vendored truth.
// A backend wire-field change must be mirrored here by hand — the authoritative
// source is the infra handler DTOs (kun-galgame-infra
// apps/api/internal/platform/galgame/dto/, revision/PR responses).
//
// Snapshot stays an open Record because DiffView indexes it by arbitrary key
// (`snap?.[k]`); snapshot-bearing shapes therefore override `snapshot` to the
// flexible bag instead of pinning every scalar field.

// A revision / PR snapshot: an open shape rendered generically by DiffView.
export type GalgameSnapshot = Record<string, unknown>

export interface GalgameRevision {
  action: string
  changed_fields?: string[] | null
  created: string | null
  galgame_id: number
  id: number
  is_minor: boolean
  note: string
  reverted_to?: number
  revision: number
  user_id: number
}
export type GalgameRevisionDetail = GalgameRevision & {
  snapshot: GalgameSnapshot
}

export interface GalgamePR {
  base_revision: number
  completed_by?: number
  completed_time?: string
  created: string | null
  galgame_id: number
  id: number
  message: string
  revision_id?: number
  snapshot: GalgameSnapshot
  status: number
  title: string
  updated: string | null
  user_id: number
}
export interface GalgamePRDetail {
  changed_keys: Record<string, boolean>
  names: GalgameDiffNames
  pr: GalgamePR
}

export interface GalgameDiff {
  changed_keys: Record<string, boolean>
  names: GalgameDiffNames
  old: GalgameSnapshot
  new: GalgameSnapshot
}
export interface GalgameDiffNames {
  engines: Record<string, string>
  officials: Record<string, string>
  series: Record<string, string>
  tags: Record<string, string>
}

export interface GalgameLink {
  created: string | null
  galgame_id: number
  id: number
  link: string
  name: string
  source: string
  source_key: string
  updated: string | null
  user_id: number
}
export interface GalgameAlias {
  created: string | null
  galgame_id: number
  id: number
  name: string
  updated: string | null
}
