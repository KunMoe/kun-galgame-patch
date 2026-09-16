// Matches apps/api/internal/common/site_search.go.

type SearchType =
  | 'all'
  | 'galgame'
  | 'entity'
  | 'resource'
  | 'user'
  | 'comment'

// 全部 pages nothing, 资料库 pages one family at a time through its own endpoint,
// and 评论 is keyset — it answers a cursor and no total, so it cannot ride the
// {items, total} envelope the paged lanes share on GET /search.
type SearchPagedType = Exclude<SearchType, 'all' | 'entity' | 'comment'>

// No engine: catalog indexes one, moyu has no /galgame/engine/:id to open.
type SearchEntityFamily =
  | 'character'
  | 'company'
  | 'staff'
  | 'tag'
  | 'series'

// The 补丁资源 lane's match width. 'model' narrows it to the AI model the
// uploader recorded — the wide lane matches that field too, but alongside the
// game, the note and the group, so "claude" answers far more than 补丁 made
// with Claude.
type SearchResourceScope = 'all' | 'model'

// What the Galgame lane's 高级筛选 panel adds to the keyword. Years are strings
// because '' is the 不限 chip, not year zero. No rating: catalog's search index
// carries no rating attribute, so 按评分筛选 is an infra change.
interface SearchGalgameFilter {
  sort: string
  tag_ids: number[]
  company_id: number
  released_from: string
  released_to: string
}

interface SearchUser extends KunUser {
  bio: string
  moemoepoint: number
  patch_count: number
  resource_count: number
  comment_count: number
}

// Name arrives on all four slots rather than pre-rendered, so the reader's
// 标题语言 setting picks between them. Render with getPreferredLanguageText.
interface SearchEntityItem {
  id: number
  family: SearchEntityFamily
  name: KunLanguage
  // image_service content hash, not a URL: resolve it with imageServiceUrl.
  // A staff row never carries one — catalog holds no credit-name photos.
  image_hash?: string
  work_count: number
}

interface SearchEntityGroup {
  family: SearchEntityFamily
  total: number
  items: SearchEntityItem[]
}

interface SearchEntityResult {
  groups: SearchEntityGroup[]
}

interface SearchTotals {
  galgame: number
  entity: number
  resource: number
  user: number
}

// What GET /search/overview and GET /search/quick both answer: every lane at
// once, plus the totals the category rail counts with. The palette asks for no
// 资料库 rows, so entities is empty there.
interface SearchLanes {
  galgames: GalgameCard[]
  entities: SearchEntityGroup[]
  resources: PatchResource[]
  users: SearchUser[]
  totals: SearchTotals
}

type SearchResultItem = GalgameCard | PatchResource | SearchUser
