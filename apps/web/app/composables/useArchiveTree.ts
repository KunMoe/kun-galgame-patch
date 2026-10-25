import { refDebounced } from '@vueuse/core'
import type { ArchiveNode } from '~/shared/utils/archiveTree'

export type ArchiveRow =
  | {
      type: 'node'
      key: string
      node: ArchiveNode
      depth: number
      open: boolean
    }
  | { type: 'more'; key: string; dir: ArchiveNode; depth: number; rest: number }
  | { type: 'loading'; key: string; dir: ArchiveNode; depth: number }

interface ArchiveFilter {
  query: string
  visible: Set<string>
  expand: Set<string>
  hits: number
}

const PAGE = 200
const ROW_IN_MS = 300

const indexFilter = (root: ArchiveNode, query: string): ArchiveFilter => {
  const visible = new Set<string>()
  const expand = new Set<string>()
  let hits = 0
  const walk = (node: ArchiveNode, inherited: boolean): boolean => {
    const hit = node.name.toLowerCase().includes(query)
    if (hit) hits++
    let below = false
    for (const child of node.children) {
      if (walk(child, inherited || hit)) below = true
    }
    if (below) expand.add(node.path)
    if (hit || below || inherited) visible.add(node.path)
    return hit || below
  }
  walk(root, false)
  return { query, visible, expand, hits }
}

export const useArchiveTree = (root: Ref<ArchiveNode>) => {
  const open = reactive(new Set<string>())
  const shown = reactive(new Map<string, number>())
  const justOpened = reactive(new Set<string>())
  const query = ref('')
  const debouncedQuery = refDebounced(query, 150)

  const replaceOpen = (paths: Iterable<string>) => {
    open.clear()
    for (const path of paths) open.add(path)
    shown.clear()
  }

  let beforeFilter: string[] = []
  const openWrapper = () => {
    const top = root.value.children
    beforeFilter = top.length === 1 && top[0]!.dir ? [top[0]!.path] : []
    replaceOpen(beforeFilter)
  }
  openWrapper()

  const filter = computed(() => {
    const q = debouncedQuery.value.trim().toLowerCase()
    return q ? indexFilter(root.value, q) : null
  })

  watch(filter, (next, prev) => {
    if (next && !prev) beforeFilter = [...open]
    replaceOpen(next ? next.expand : beforeFilter)
  })

  const rows = computed(() => {
    const out: ArchiveRow[] = []
    const f = filter.value
    const walk = (dir: ArchiveNode, depth: number) => {
      const kids = f
        ? dir.children.filter((c) => f.visible.has(c.path))
        : dir.children
      const limit = shown.get(dir.path) ?? PAGE
      for (const node of kids.slice(0, limit)) {
        const isOpen = node.dir && open.has(node.path)
        out.push({ type: 'node', key: node.path, node, depth, open: isOpen })
        if (isOpen && isUnloaded(node)) {
          out.push({
            type: 'loading',
            key: `${node.path}/:loading`,
            dir: node,
            depth: depth + 1
          })
        } else if (isOpen) walk(node, depth + 1)
      }
      if (kids.length > limit) {
        out.push({
          type: 'more',
          key: `${dir.path}/:more`,
          dir,
          depth,
          rest: kids.length - limit
        })
      }
    }
    walk(root.value, 0)
    return out
  })

  const toggle = (node: ArchiveNode) => {
    if (open.delete(node.path)) return
    open.add(node.path)
    justOpened.add(node.path)
    setTimeout(() => justOpened.delete(node.path), ROW_IN_MS)
  }

  const showMore = (dir: ArchiveNode) => {
    shown.set(dir.path, (shown.get(dir.path) ?? PAGE) + PAGE * 5)
  }

  const collapseAll = () => open.clear()

  return {
    query,
    filter,
    rows,
    isOpen: (path: string) => open.has(path),
    justOpened,
    toggle,
    showMore,
    collapseAll
  }
}
