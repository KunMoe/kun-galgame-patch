// Friendly aliases to the generated galgame-wiki OpenAPI schemas
// (./generated/galgame-read-api.ts, produced from the published spec — see
// docs/galgame_wiki/09-openapi-specs.md). These give the wiki-proxied edit
// surface a single, drift-safe source of truth: a backend wire-field change
// fails the openapi-typescript regen (CI drift gate) AND type-checks here,
// instead of breaking silently at runtime.
//
// Snapshot stays an open Record because DiffView indexes it by arbitrary key
// (`snap?.[k]`); snapshot-bearing shapes therefore source every scalar field
// from the spec but override `snapshot` to the flexible bag.
import type { components } from './generated/galgame-read-api'

type WikiSchemas = components['schemas']

// A revision / PR snapshot: an open shape rendered generically by DiffView.
export type GalgameSnapshot = Record<string, unknown>

export type GalgameRevision = Omit<WikiSchemas['RevisionResponse'], 'snapshot'>
export type GalgameRevisionDetail = GalgameRevision & {
  snapshot: GalgameSnapshot
}

export type GalgamePR = Omit<WikiSchemas['PRResponse'], 'snapshot'> & {
  snapshot: GalgameSnapshot
}
export type GalgamePRDetail = Omit<WikiSchemas['PRDetailData'], 'pr'> & {
  pr: GalgamePR
}

export type GalgameDiff = Omit<WikiSchemas['RevisionDiffData'], 'old' | 'new'> & {
  old: GalgameSnapshot
  new: GalgameSnapshot
}
export type GalgameDiffNames = WikiSchemas['SnapshotEntityNames']

export type GalgameLink = WikiSchemas['DetailLink']
export type GalgameAlias = WikiSchemas['DetailAlias']
