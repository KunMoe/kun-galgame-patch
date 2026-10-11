// Matches apps/api/pkg/catalogv2/tag_blocks.go.

type KunTagBlockMode = 'tree' | 'exact'

interface KunTagBlockSet {
  tree: number[]
  exact: number[]
}

interface TagBlock {
  tag_id: number
  mode: KunTagBlockMode
  created_at: string
  updated_at: string
}
