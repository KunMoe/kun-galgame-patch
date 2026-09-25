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
  cosmetics?: UserCosmetics
  bio: string
  roles: string[]
  site_roles: string[]
  moemoepoint: number
  register_time: string
  follower_count?: number
  following_count?: number
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
  // Legacy absolute banner URL — empty post wiki→catalog migration; kept as a
  // resolveBannerUrl fallback. effective_banner_hash is the current cover source
  // (image_service hash of the pinned cover) the resolver prefers.
  banner: string
  effective_banner_hash?: string
  name: KunLanguage
}

// A user's comments come from the same community feed shape every other mixed
// comment list does — including `link`, which is the only thing that knows which
// of the two walls a row is on.
type UserComment = PatchComment

type UserFavoriteItem = GalgameCard

type UserGalgameItem = GalgameCard
