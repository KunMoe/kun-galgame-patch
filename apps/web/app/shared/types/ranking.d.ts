interface RankingUser {
  id: number
  name: string
  avatar: string
  cosmetics?: UserCosmetics
  moemoepoint: number
  patch_count: number
  resource_count: number
  comment_count: number
}

interface RankingPatch {
  id: number
  name: KunLanguage
  banner: string
  view: number
  download: number
  count?: {
    favorite_by?: number
    resource?: number
    comment?: number
  }
}
