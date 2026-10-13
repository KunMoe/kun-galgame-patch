import type { KunUIColor } from '@kungal/ui-core'

export const KUN_RATING_PLAY_STATE_OPTIONS = [
  { value: 'wish', label: '想玩', icon: 'lucide:bookmark' },
  { value: 'doing', label: '游玩中', icon: 'lucide:play' },
  { value: 'done_one_route', label: '单线通关', icon: 'lucide:flag' },
  { value: 'done_main', label: '主线通关', icon: 'lucide:flag' },
  { value: 'done_all', label: '全线通关', icon: 'lucide:trophy' },
  { value: 'on_hold', label: '搁置中', icon: 'lucide:pause' },
  { value: 'dropped', label: '已弃坑', icon: 'lucide:x' }
] as const

export type KunRatingPlayState =
  | (typeof KUN_RATING_PLAY_STATE_OPTIONS)[number]['value']
  | 'done'

// A finish another application recorded without saying how much. The form
// offers it only to the reader who already has it, so a save leaves it as is.
export const KUN_RATING_PLAY_STATE_DONE = {
  value: 'done',
  label: '已通关',
  icon: 'lucide:flag'
} as const

export const KUN_RATING_PLAY_STATE_MAP: Record<string, string> = {
  ...Object.fromEntries(
    KUN_RATING_PLAY_STATE_OPTIONS.map((o) => [o.value, o.label])
  ),
  [KUN_RATING_PLAY_STATE_DONE.value]: KUN_RATING_PLAY_STATE_DONE.label
}

export const KUN_RATING_RECOMMEND_OPTIONS = [
  { value: 'strong_yes', label: '强烈推荐' },
  { value: 'yes', label: '推荐' },
  { value: 'neutral', label: '中立' },
  { value: 'no', label: '不推荐' },
  { value: 'strong_no', label: '强烈不推荐' }
] as const

export type KunRatingRecommend =
  (typeof KUN_RATING_RECOMMEND_OPTIONS)[number]['value']

export const KUN_RATING_RECOMMEND_MAP: Record<string, string> =
  Object.fromEntries(
    KUN_RATING_RECOMMEND_OPTIONS.map((o) => [o.value, o.label])
  )

export const KUN_RATING_RECOMMEND_COLOR: Record<string, KunUIColor> = {
  strong_yes: 'warning',
  yes: 'success',
  neutral: 'secondary',
  no: 'default',
  strong_no: 'danger'
}

export const KUN_RATING_SPOILER_OPTIONS = [
  { value: 'none', label: '无剧透' },
  { value: 'portion', label: '部分剧透' },
  { value: 'serious', label: '严重剧透' }
] as const

export type KunRatingSpoiler =
  (typeof KUN_RATING_SPOILER_OPTIONS)[number]['value']

export const KUN_RATING_SPOILER_MAP: Record<string, string> =
  Object.fromEntries(KUN_RATING_SPOILER_OPTIONS.map((o) => [o.value, o.label]))

export const KUN_RATING_SPOILER_COLOR: Record<string, KunUIColor> = {
  none: 'success',
  portion: 'warning',
  serious: 'danger'
}

export const KUN_RATING_ASPECTS = [
  { key: 'art', label: '画面' },
  { key: 'story', label: '剧情' },
  { key: 'music', label: '音乐' },
  { key: 'character', label: '角色' },
  { key: 'route', label: '分支' },
  { key: 'system', label: '系统' },
  { key: 'voice', label: '配音' },
  { key: 'replay_value', label: '重玩' }
] as const satisfies ReadonlyArray<{
  key: keyof GalgameRatingAspects
  label: string
}>

// Both are catalog's: it refuses a report above the ceiling, and leaves one
// under the floor out of every median.
export const KUN_PLAYTIME_HOURS_MAX = 1000
export const KUN_PLAYTIME_MINUTES_FLOOR = 10

export const KUN_RATING_SUMMARY_MAX = 2000
