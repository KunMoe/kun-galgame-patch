export const tagPathsUnder = (rootId: number, nodes: TagTreeNode[]) => {
  const byId = new Map(nodes.map((node) => [node.id, node]))
  const paths = new Map<number, TagTreeNode[]>([[rootId, []]])
  for (const node of nodes) {
    let best: TagTreeNode[] | undefined
    for (const parentId of node.parent_ids) {
      const above = paths.get(parentId)
      if (!above) {
        continue
      }
      const parent = byId.get(parentId)
      const via = parent ? [...above, parent] : above
      if (!best || via.length < best.length) {
        best = via
      }
    }
    paths.set(node.id, best ?? [])
  }
  return paths
}

export const ancestorTrail = (parents: TagRef[], ancestors: TagTreeNode[]) => {
  const byId = new Map(ancestors.map((node) => [node.id, node]))
  const trail: TagTreeNode[] = []
  const seen = new Set<number>()
  let next = parents[0] && byId.get(parents[0].id)
  while (next && !seen.has(next.id)) {
    seen.add(next.id)
    trail.unshift(next)
    const up = next.parent_ids[0]
    next = up === undefined ? undefined : byId.get(up)
  }
  return trail
}
