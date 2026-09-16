import type { LocationQuery } from 'vue-router'

export interface SearchCategory {
  value: SearchType
  textValue: string
  icon: string
  /** Measure word + noun, as it reads after 共 N. */
  countUnit: string
}

export const SEARCH_CATEGORIES: SearchCategory[] = [
  {
    value: 'all',
    textValue: '全部',
    icon: 'lucide:layout-grid',
    countUnit: '个结果'
  },
  {
    value: 'galgame',
    textValue: 'Galgame',
    icon: 'lucide:gamepad-2',
    countUnit: '个 Galgame'
  },
  {
    value: 'entity',
    textValue: '资料库',
    icon: 'lucide:library',
    countUnit: '个条目'
  },
  {
    value: 'resource',
    textValue: '补丁资源',
    icon: 'lucide:puzzle',
    countUnit: '个补丁资源'
  },
  {
    value: 'user',
    textValue: '用户',
    icon: 'lucide:user-round',
    countUnit: '位用户'
  },
  {
    value: 'comment',
    textValue: '评论',
    icon: 'lucide:message-square',
    countUnit: '条评论'
  }
]

export const SEARCH_CATEGORY_MAP = Object.fromEntries(
  SEARCH_CATEGORIES.map((category) => [category.value, category])
) as Record<SearchType, SearchCategory>

// A page size per category rather than one for all of them: the galgame lane
// draws the poster grid every other list draws, the other two draw rows.
export const SEARCH_PAGE_SIZE: Record<SearchPagedType, number> = {
  galgame: 24,
  resource: 12,
  user: 12
}

export const isSearchType = (value: unknown): value is SearchType =>
  typeof value === 'string' && value in SEARCH_CATEGORY_MAP

export interface SearchEntityFamilyMeta {
  value: SearchEntityFamily
  textValue: string
  icon: string
  path: string
  /** Reads after N; a 会社 and a 标签 count works, a 角色 does not. */
  countUnit?: string
}

export const SEARCH_ENTITY_FAMILIES: SearchEntityFamilyMeta[] = [
  {
    value: 'character',
    textValue: '角色',
    icon: 'lucide:drama',
    path: '/galgame/character'
  },
  {
    value: 'company',
    textValue: '会社',
    icon: 'lucide:building-2',
    path: '/galgame/official',
    countUnit: '部作品'
  },
  {
    value: 'staff',
    textValue: 'Staff',
    icon: 'lucide:signature',
    path: '/galgame/staff'
  },
  {
    value: 'tag',
    textValue: '标签',
    icon: 'lucide:tag',
    path: '/galgame/tag',
    countUnit: '部作品'
  },
  {
    value: 'series',
    textValue: '系列',
    icon: 'lucide:layers',
    path: '/galgame/series',
    countUnit: '部作品'
  }
]

export const SEARCH_ENTITY_FAMILY_MAP = Object.fromEntries(
  SEARCH_ENTITY_FAMILIES.map((family) => [family.value, family])
) as Record<SearchEntityFamily, SearchEntityFamilyMeta>

// 全部 previews every family at once and each one costs its own catalog
// request, so it asks for a third of what a single family gets.
export const SEARCH_ENTITY_LIMIT = { all: 8, one: 24 }

export const searchEntityPath = (item: SearchEntityItem) =>
  `${SEARCH_ENTITY_FAMILY_MAP[item.family].path}/${item.id}`

export const isSearchEntityFamily = (
  value: unknown
): value is SearchEntityFamily =>
  typeof value === 'string' && value in SEARCH_ENTITY_FAMILY_MAP

// Catalog's five search sorts. relevance is what a keyword defaults to upstream,
// so it travels as the absent value rather than as a token.
export const SEARCH_GALGAME_SORTS = [
  { value: 'relevance', label: '相关度' },
  { value: 'popularity', label: '热门' },
  { value: 'released_desc', label: '最新发售' },
  { value: 'released_asc', label: '最早发售' },
  { value: 'updated', label: '最近更新' }
]

// The uploader types the model name by hand, so the column holds
// claude-3.7-sonnet, Claude-3.7-Sonnet, Claude3.7Sonnet and claude-3-7-sonnet
// as four different strings. A chip therefore carries the family, which the
// narrow lane's substring match folds all of them back into; nine of them cover
// 98% of the rows that name a model at all.
export const SEARCH_AI_MODEL_FAMILIES = [
  'Gemini',
  'Claude',
  'DeepSeek',
  'GPT',
  'Qwen',
  'Grok',
  'Sakura',
  'GalTransl',
  'Kimi'
]

export const SEARCH_GALGAME_YEAR_MIN = 1980

// Catalog answers 400 past ten tag ids.
export const SEARCH_GALGAME_TAG_MAX = 10

const first = (value: unknown): string => {
  const raw = Array.isArray(value) ? value[0] : value
  return typeof raw === 'string' ? raw.trim() : ''
}

const asYear = (value: unknown): string => {
  const year = Number(first(value))
  return Number.isInteger(year) && year >= SEARCH_GALGAME_YEAR_MIN
    ? String(year)
    : ''
}

export const readSearchGalgameFilter = (
  query: Record<string, unknown>
): SearchGalgameFilter => {
  const sort = first(query.sort)
  const companyId = Number(first(query.company_id))
  return {
    sort: SEARCH_GALGAME_SORTS.some((option) => option.value === sort)
      ? sort
      : 'relevance',
    tag_ids: first(query.tag_ids)
      .split(',')
      .map((id) => Number(id))
      .filter((id) => Number.isInteger(id) && id > 0)
      .slice(0, SEARCH_GALGAME_TAG_MAX),
    company_id: Number.isInteger(companyId) && companyId > 0 ? companyId : 0,
    released_from: asYear(query.released_from),
    released_to: asYear(query.released_to)
  }
}

export const searchGalgameFilterQuery = (
  filter: SearchGalgameFilter
): Record<string, string> => {
  const query: Record<string, string> = {}
  if (filter.sort !== 'relevance') {
    query.sort = filter.sort
  }
  if (filter.tag_ids.length) {
    query.tag_ids = filter.tag_ids.join(',')
  }
  if (filter.company_id) {
    query.company_id = String(filter.company_id)
  }
  if (filter.released_from) {
    query.released_from = filter.released_from
  }
  if (filter.released_to) {
    query.released_to = filter.released_to
  }
  return query
}

export const isSearchResourceScope = (
  value: unknown
): value is SearchResourceScope => value === 'model'

export const SEARCH_GALGAME_FILTER_KEYS = [
  'sort',
  'tag_ids',
  'company_id',
  'released_from',
  'released_to'
]

// Every filter this page writes into the URL. They belong to one lane each, so
// switching lanes or keywords clears all of them rather than silently narrowing
// the next search.
export const SEARCH_FILTER_QUERY_KEYS = [
  'family',
  'scope',
  ...SEARCH_GALGAME_FILTER_KEYS
]

// The one thing every query this page writes has to drop, on top of whatever
// keys it is replacing: `page` is where the reader's position lives now, and a
// page number carried into a different keyword, lane or filter asks for page 7
// of a result set that has three.
export const searchQueryWithout = (
  query: LocationQuery,
  ...drop: string[]
): LocationQuery =>
  Object.fromEntries(
    Object.entries(query).filter(
      ([key]) => key !== 'page' && !drop.includes(key)
    )
  )
