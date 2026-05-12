// Backend returns flat snake_case counts (not a nested _count wrapper).
// See apps/api/internal/user/dto/dto.go UserInfoResponse.
interface UserInfo {
  id: number
  name: string
  email?: string
  avatar: string
  bio: string
  role: number
  status: number
  moemoepoint: number
  register_time: string
  follower_count: number
  following_count: number
  patch_count: number
  resource_count: number
  comment_count: number
  favorite_count: number
  is_followed: boolean
}

// PatchSummary mirrors apps/api/internal/patch/model.PatchSummary -- the
// nested {id, vndb_id, banner, name: KunLanguage} object filled by the
// backend enricher via Wiki /galgame/batch.
interface PatchSummary {
  id: number
  vndb_id: string
  banner: string
  name: KunLanguage
}

// User's own resource row: a PatchResource plus the owning patch's Wiki
// summary (so the user-profile resource list can render the game name +
// banner without an extra request per row).
interface UserResourceItem {
  id: number
  galgame_id: number
  size: string
  type: string[]
  language: string[]
  platform: string[]
  created: string
  // Filled by user/service.attachPatchSummaries from Wiki; may be missing if
  // the underlying galgame is no longer in Wiki.
  patch?: PatchSummary
}

interface UserComment {
  id: number
  content: string
  like_count: number
  user_id: number
  galgame_id: number
  created: string
  patch?: PatchSummary
}

interface UserFavoriteItem extends GalgameCard {}

interface UserGalgameItem extends GalgameCard {}
