// Backend returns flat snake_case counts (not a nested _count wrapper).
// See apps/api/internal/user/dto/dto.go UserInfoResponse.
//
// `roles` is the OAuth-side role set for the profile being viewed. Per-site
// numeric `role` / `status` / `email` were dropped in the OAuth migration --
// identity is owned by OAuth and not re-exposed by /user/:id.
interface UserInfo {
  id: number
  name: string
  avatar: string
  bio: string
  roles: string[]
  site_roles: string[]
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
  // The resource's own display name; the user-profile resource tab renders it
  // (info.vue). Backend includes it on every row.
  name?: string
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

type UserFavoriteItem = GalgameCard

type UserGalgameItem = GalgameCard
