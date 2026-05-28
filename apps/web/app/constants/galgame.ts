export const GALGAME_AGE_LIMIT_MAP: Record<string, string> = {
  sfw: 'SFW',
  nsfw: 'NSFW'
}

export const GALGAME_AGE_LIMIT_DETAIL: Record<string, string> = {
  sfw: '本文章内容安全, 无 R18 等内容, 适合在公共场所浏览',
  nsfw: '本文章可能包含 R18 等内容, 不适合在公共场所浏览'
}

export const GALGAME_SORT_FIELD_LABEL_MAP: Record<string, string> = {
  resource_update_time: '补丁更新时间',
  created: '游戏创建时间',
  view: '浏览量',
  download: '下载量',
  // 按游戏发售日期排序（本地镜像 patch.release_date，wiki §17）。
  release_date: '发售日期'
}

const currentYear = new Date().getFullYear()
export const GALGAME_SORT_YEARS = [
  'all',
  'future',
  'unknown',
  ...Array.from({ length: currentYear - 1979 }, (_, i) =>
    String(currentYear - i)
  )
]

export const GALGAME_SORT_YEARS_MAP: Record<string, string> = {
  all: '全部年份',
  future: '未发售',
  unknown: '未知年份'
}

export const GALGAME_SORT_MONTHS = [
  'all',
  '01',
  '02',
  '03',
  '04',
  '05',
  '06',
  '07',
  '08',
  '09',
  '10',
  '11',
  '12'
]
