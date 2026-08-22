interface SeriesDetail {
  id: number
  name: string
  description?: string
  has_nsfw: boolean
  galgames: GalgameCard[]
  total: number
}
