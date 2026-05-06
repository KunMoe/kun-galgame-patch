// After D8 / D11 / D12 (2026-04-21), patch-related types are significantly slimmed:
//   - cover/screenshot/char/person/release are owned by the Galgame Wiki (D8)
//   - tag/company also belong to Wiki (D11)
//   - patch itself no longer stores name/introduction/banner/released/content_limit/engine/alias (D12)
//
// All JSON keys are snake_case to match the backend wire format exactly.
// The backend enricher merges patch + Wiki galgame into the shape below.

interface GalgameCard {
  id: number
  name: KunLanguage
  vndb_id: string
  bid: number | null
  banner: string
  view: number
  download: number
  type: string[]
  language: string[]
  platform: string[]
  content_limit: KunContentLimit
  status: number
  created: Date | string
  resource_update_time: Date | string
  count: {
    favorite_by: number
    contribute_by: number
    resource: number
    comment: number
  }
  user?: KunUser
  // Optional: raw Wiki galgame object (includes age_limit, original_language, etc.)
  galgame?: {
    id: number
    vndb_id: string
    name_en_us: string
    name_zh_cn: string
    name_ja_jp: string
    name_zh_tw: string
    banner: string
    content_limit: string
    age_limit: string
    original_language: string
    user_id: number
    resource_update_time: string
  }
}

// Patch header (/patch/:id) -- GalgameCard + is_favorite.
interface PatchHeader extends GalgameCard {
  is_favorite: boolean
}

// Patch detail (/patch/:id/detail) -- GalgameCard plus Wiki's full galgame info.
// introduction_markdown is filled in by the backend via Wiki /galgame/:gid; the
// enricher also resolves tags/officials by name on the server side so the frontend
// can render labels directly.
interface PatchDetailTag {
  id: number
  name: string
  aliases?: string[]
  category: string
  spoiler_level: number
}

interface PatchDetailOfficial {
  id: number
  name: string
  aliases?: string[]
  category: string
  lang: string
}

interface PatchDetail extends GalgameCard {
  introduction_markdown: KunLanguage
  introduction_html: KunLanguage
  updated: string
  tags: PatchDetailTag[]
  officials: PatchDetailOfficial[]
  wiki_engine_ids: number[]
}
