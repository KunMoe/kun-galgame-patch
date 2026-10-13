// A rating is NextMoe's: one per person per work, the same on every NextMoe
// site. moyu reads them from its mirror and writes them with the reader's token.

interface GalgameRatingAspects {
  art: number | null
  story: number | null
  music: number | null
  character: number | null
  route: number | null
  system: number | null
  voice: number | null
  replay_value: number | null
}

interface GalgameRating {
  id: number
  work_id: number
  // null when OAuth no longer knows the author.
  user: KunUser | null
  overall: number
  aspect_scores: GalgameRatingAspects
  recommend: string
  spoiler_level: string
  short_summary: string
  // The author's play state now, not when they rated; '' when none.
  play_state: string
  created: string
  updated: string
  can_delete: boolean
}

interface GalgameWorkRecord {
  play_state: string
  // The most minutes any NextMoe application reported, not only this site.
  minutes: number
  rating: GalgameRating | null
}

interface GalgameRatingInput {
  overall: number
  recommend: string
  spoiler_level: string
  short_summary: string
  aspect_scores: GalgameRatingAspects | null
}

interface GalgameWorkRecordInput {
  play_state?: string
  minutes?: number
  rating?: GalgameRatingInput
}
