import { describe, expect, it } from 'vitest'
import { buildArchiveTree, findArchiveNode } from './archiveTree'
import { archiveFileEndpoint, createPreviewResolver } from './archivePreview'

const rules = [
  {
    kind: 'text' as const,
    max_bytes: 100,
    truncate: true,
    extensions: ['txt', 'ini']
  },
  {
    kind: 'image' as const,
    max_bytes: 100,
    truncate: false,
    extensions: ['png']
  }
]

const root = buildArchiveTree([
  { path: 'Game/readme.TXT', size: 500 },
  { path: 'Game/cg/01.png', size: 10 },
  { path: 'Game/cg/02.png', size: 500 },
  { path: 'Game/Game.exe', size: 10 },
  { path: 'Game/img/a.png_', size: 10 }
])
const node = (path: string) => findArchiveNode(root, path)!

describe('createPreviewResolver', () => {
  const resolve = createPreviewResolver(rules)

  it('opens listed extensions in any case and truncates only text', () => {
    expect(resolve(node('Game/readme.TXT'))).toEqual({
      kind: 'text',
      truncated: true,
      maxBytes: 100
    })
    expect(resolve(node('Game/cg/01.png'))).toEqual({
      kind: 'image',
      truncated: false,
      maxBytes: 100
    })
    expect(resolve(node('Game/cg/02.png'))).toBeNull()
  })

  it('never opens folders, executables or encrypted engine assets', () => {
    expect(resolve(node('Game/cg'))).toBeNull()
    expect(resolve(node('Game/Game.exe'))).toBeNull()
    expect(resolve(node('Game/img/a.png_'))).toBeNull()
  })
})

describe('findArchiveNode', () => {
  it('walks to a node and misses cleanly', () => {
    expect(findArchiveNode(root, '')).toBe(root)
    expect(node('Game/cg').children.map((n) => n.name)).toEqual([
      '01.png',
      '02.png'
    ])
    expect(findArchiveNode(root, 'Game/nope/a.txt')).toBeNull()
  })
})

describe('archiveFileEndpoint', () => {
  it('escapes the whole path into one query value', () => {
    expect(archiveFileEndpoint(5, 'ゲーム/説明 #1&2.txt')).toBe(
      '/resource/5/files/content?path=%E3%82%B2%E3%83%BC%E3%83%A0%2F%E8%AA%AC%E6%98%8E%20%231%262.txt'
    )
  })
})
