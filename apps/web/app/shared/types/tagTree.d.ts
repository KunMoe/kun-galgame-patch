// Matches apps/api/internal/galgame/client/catalog_tag_tree.go.

interface TagRef {
  id: number
  name: string
  is_sexual: boolean
}

interface TagTreeNode extends TagRef {
  parent_ids: number[]
}

interface TagSubtag extends TagRef {
  work_count: number
  descendant_count: number
}
