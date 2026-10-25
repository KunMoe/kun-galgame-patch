import { describe, expect, it } from 'vitest'
import {
  buildArchiveTree,
  fileKindOf,
  formatBytes,
  isUnloaded,
  splitExtension
} from './archiveTree'

describe('buildArchiveTree', () => {
  it('derives folders a zip never listed and sums them', () => {
    const root = buildArchiveTree([
      { path: 'Game/data/a.xp3', size: 100 },
      { path: 'Game/data/b.xp3', size: 50 },
      { path: 'Game/game.exe', size: 10 }
    ])
    const game = root.children[0]!
    expect(game.name).toBe('Game')
    expect(game.dir).toBe(true)
    expect(game.size).toBe(160)
    expect(game.files).toBe(3)
    const data = game.children[0]!
    expect(data.path).toBe('Game/data')
    expect(data.parent).toBe('Game')
    expect(data.size).toBe(150)
  })

  it('keeps an explicit empty folder and puts folders before files', () => {
    const root = buildArchiveTree([
      { path: 'w/', size: 0 },
      { path: 'w/readme.txt', size: 1 },
      { path: 'w/save/', size: 0 }
    ])
    expect(root.children.map((n) => n.path)).toEqual(['w'])
    expect(root.children[0]!.children.map((n) => n.name)).toEqual([
      'save',
      'readme.txt'
    ])
  })

  it('takes a folder cut by the outline at its shipped totals', () => {
    const root = buildArchiveTree([
      { path: 'w/', size: 0 },
      { path: 'w/game.exe', size: 10 },
      { path: 'w/cg/', size: 900, files: 300, items: 2 }
    ])
    const w = root.children[0]!
    const cg = w.children[0]!
    expect([cg.size, cg.files, cg.items]).toEqual([900, 300, 2])
    expect(isUnloaded(cg)).toBe(true)
    expect([w.size, w.files, w.items]).toEqual([910, 301, 2])
    expect(isUnloaded(w)).toBe(false)
  })

  it('sorts numbered names the way a file manager does', () => {
    const root = buildArchiveTree(
      ['bg10.png', 'bg2.png', 'bg1.png'].map((name) => ({
        path: name,
        size: 1
      }))
    )
    expect(root.children.map((n) => n.name)).toEqual([
      'bg1.png',
      'bg2.png',
      'bg10.png'
    ])
  })
})

describe('file helpers', () => {
  it('splits the extension but not a leading dot', () => {
    expect(splitExtension('data.xp3')).toEqual(['data', '.xp3'])
    expect(splitExtension('.gitignore')).toEqual(['.gitignore', ''])
    expect(splitExtension('README')).toEqual(['README', ''])
  })

  it('labels known and unknown extensions', () => {
    expect(fileKindOf('Game.EXE').label).toBe('应用程序')
    expect(fileKindOf('voice.ogg').icon).toBe('lucide:file-music')
    expect(fileKindOf('thing.xyz').label).toBe('XYZ 文件')
    expect(fileKindOf('LICENSE').label).toBe('文件')
  })

  it('formats sizes like a file manager', () => {
    expect(formatBytes(0)).toBe('0 B')
    expect(formatBytes(1023)).toBe('1023 B')
    expect(formatBytes(1024)).toBe('1 KB')
    expect(formatBytes(1834496)).toBe('1.7 MB')
    expect(formatBytes(4282403304)).toBe('4 GB')
    expect(formatBytes(150 * 1024 * 1024)).toBe('150 MB')
  })
})
