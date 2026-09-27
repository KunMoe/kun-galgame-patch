// Mirrors apps/api/internal/community/following (Item / Group / Page / ItemPage)
// and communityclient.ActivitySetting.
interface FollowingActivityItem {
  id: number
  site: string
  verb: string
  object_kind: string
  object_label: string
  title: string
  excerpt: string
  url: string
  path: string | null
  cover_image_hash: string
  work_id: number | null
  content_limit: 'sfw' | 'nsfw'
  occurred_at: string
}

interface FollowingActivityGroup {
  id: number
  site: string
  actor: KunUser | null
  verb: string
  object_kind: string
  object_label: string
  day: string
  item_count: number
  latest_at: string
  items: FollowingActivityItem[]
}

interface FollowingActivityPage {
  groups: FollowingActivityGroup[]
  next_cursor: string
  seen_mark: string | null
}

interface FollowingActivityItemPage {
  items: FollowingActivityItem[]
  next_cursor: string
}

interface FollowingActivitySetting {
  user_id: number
  hidden: boolean
  updated_at: string | null
}
