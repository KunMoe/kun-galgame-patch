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
  content?: string
  code?: string
  password?: string
  like_count: number
  is_liked?: boolean
  status?: number
  download: number
  user_id?: number
  galgame_id: number
  created: string
  update_time?: Date | string
  user: KunUser
  // Populated only by global resource lists (/api/v1/resource, /api/v1/home).
  patch?: PatchSummary | null
}

// Backwards-compat alias: `note_html` is now part of PatchResource itself.
type PatchResourceHtml = PatchResource

interface HomeResource extends PatchResource {}

// D11 + D12: the resource detail page receives a lightweight owning-patch card
// that is really just the enricher GalgameCard shape. We re-use GalgameCard
// rather than redefine it so the fields stay in lockstep with the backend.
interface PatchResourceDetail {
  resource: PatchResourceHtml
  patch: GalgameCard | null
  recommendations: PatchResource[]
}
