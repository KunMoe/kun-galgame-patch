import type { KunTagBlockState } from '~/stores/tagBlockStore'

export const emptyTagBlocks = (): KunTagBlockSet => ({ tree: [], exact: [] })

export const resolveTagBlocks = (
  state: KunTagBlockState,
  uid: number
): KunTagBlockSet => {
  if (uid <= 0) return state.anonymous
  return state.accountUid === uid ? state.account : emptyTagBlocks()
}

const ascending = (ids: number[]) => [...ids].sort((a, b) => a - b).join(',')

export const tagBlockQuery = (set: KunTagBlockSet): string =>
  [
    set.tree.length ? `exclude_tag_trees=${ascending(set.tree)}` : '',
    set.exact.length ? `exclude_tag_ids=${ascending(set.exact)}` : ''
  ]
    .filter(Boolean)
    .join('&')

export const withTagBlockMode = (
  set: KunTagBlockSet,
  id: number,
  mode: KunTagBlockMode | null
): KunTagBlockSet => {
  const next = {
    tree: set.tree.filter((x) => x !== id),
    exact: set.exact.filter((x) => x !== id)
  }
  if (mode) next[mode] = [...next[mode], id]
  return next
}
