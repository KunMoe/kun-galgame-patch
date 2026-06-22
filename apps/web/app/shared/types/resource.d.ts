// Matches apps/api/internal/patch/model/model.go PatchResource.
interface PatchResource {
  id: number
  storage: string
  name: string
  model_name: string
  size: string
  type: string[]
  language: string[]
  platform: string[]
  note: string
  note_html: string
  blake3?: string
  s3_key?: string
  // artifact-service blob id (current path); s3_key is legacy direct-B2.
  artifact_uuid?: string
  content?: string
  // Resolved absolute download URL for artifact-backed rows, filled by the
  // /link (and detail) endpoints after the access gates pass. Empty for legacy
  // rows (the FE builds those from content via resolveDownloadLinks).
  download_url?: string
  code?: string
  password?: string
  like_count: number
  is_liked?: boolean
  // Per-resource subscription state (filled on GET /resource/:id for the
  // logged-in viewer). When true the user is notified on this resource's
  // file/link updates. Drives the "收藏资源" button.
  is_favorite?: boolean
  status?: number
  download: number
  user_id?: number
  galgame_id: number
  created: string
  // `update_time` is the canonical 更改时间: creation time on insert, bumped
  // to now() only on a real re-edit (matches next-api update_time: new Date()).
  // `updated` is gorm autoUpdateTime — bumps on ANY row write (incl.
  // download/like increments), so it is NOT a reliable edit timestamp.
  update_time?: Date | string
  updated: string
  user: KunUser
  // Populated only by global resource lists (/api/v1/resource, /api/v1/home).
  patch?: PatchSummary | null
}

// Backwards-compat alias: `note_html` is now part of PatchResource itself.
type PatchResourceHtml = PatchResource

type HomeResource = PatchResource

// D11 + D12: the resource detail page receives a lightweight owning-patch card
// that is really just the enricher GalgameCard shape. We re-use GalgameCard
// rather than redefine it so the fields stay in lockstep with the backend.
interface PatchResourceDetail {
  resource: PatchResourceHtml
  patch: GalgameCard | null
  // Other resources of the same patch; topped up with random popular ones
  // from other patches when the same patch has < 5 (mirrors next-web).
  recommendations: PatchResource[]
  // Whether the viewer has favorited the owning galgame (false when anon).
  patch_is_favorite?: boolean
}
