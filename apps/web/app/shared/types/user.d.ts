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

interface UserResourceItem {
  id: number
  galgame_id: number
  patch_name: KunLanguage
  patch_banner: string
  size: string
  type: string[]
  language: string[]
  platform: string[]
  created: string
}

interface UserContribute {
  id: number
  galgame_id: number
  patch_name: KunLanguage
  created: string
}

interface UserComment {
  id: number
  content: string
  like: number
  user_id: number
  galgame_id: number
  patch_name: KunLanguage
  created: string
  quoted_user_uid?: number | null
  quoted_username?: string | null
}

interface UserFavoriteItem extends GalgameCard {}

interface UserGalgameItem extends GalgameCard {}
