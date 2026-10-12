// @vitest-environment node
import { describe, expect, it } from 'vitest'
import { ancestorTrail, tagPathsUnder } from './tagTree'

const node = (id: number, parentIds: number[], isSexual = false) => ({
  id,
  name: `t${id}`,
  is_sexual: isSexual,
  parent_ids: parentIds
})

const ids = (nodes: TagTreeNode[] | undefined) => nodes?.map((n) => n.id)

describe('tagPathsUnder', () => {
  it('names the tags between the root and each node along a shortest path', () => {
    const paths = tagPathsUnder(1, [
      node(2, [1]),
      node(3, [1]),
      node(4, [2, 3]),
      node(5, [4, 1]),
      node(6, [4]),
      node(7, [6, 3])
    ])
    expect(ids(paths.get(2))).toEqual([])
    expect(ids(paths.get(4))).toEqual([2])
    expect(ids(paths.get(5))).toEqual([])
    expect(ids(paths.get(6))).toEqual([2, 4])
    expect(ids(paths.get(7))).toEqual([3])
  })

  it('leaves a node whose parents are all unknown at the root', () => {
    const paths = tagPathsUnder(1, [node(2, [9])])
    expect(ids(paths.get(2))).toEqual([])
  })
})

describe('ancestorTrail', () => {
  it('walks first parents up to a root, root first', () => {
    const trail = ancestorTrail(
      [node(3, [])],
      [node(3, [2, 9]), node(9, []), node(2, [1]), node(1, [])]
    )
    expect(ids(trail)).toEqual([1, 2, 3])
  })

  it('starts from the first parent the reader was given, adult ancestors kept', () => {
    const trail = ancestorTrail(
      [node(4, [])],
      [node(8, [1], true), node(4, [1]), node(1, [])]
    )
    expect(ids(trail)).toEqual([1, 4])
  })

  it('is empty for a root and stops on a loop', () => {
    expect(ancestorTrail([], [])).toEqual([])
    const trail = ancestorTrail([node(2, [])], [node(2, [3]), node(3, [2])])
    expect(ids(trail)).toEqual([3, 2])
  })
})
