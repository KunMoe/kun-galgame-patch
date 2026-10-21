import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

// The catalog is 鲲 Galgame 论坛数据 to readers; 资料库 named it before.
const RETIRED_SOURCE_NAMES = ['鲲 Galgame 目录', 'NextMoe 目录', '资料库']

const appDir = join(process.cwd(), 'app')

const sourceFiles = () =>
  readdirSync(appDir, { recursive: true, encoding: 'utf8' }).filter(
    (f) => /\.(vue|ts)$/.test(f) && !f.endsWith('.spec.ts')
  )

describe('game information source copy', () => {
  it('credits the forum, never a retired catalog name', () => {
    const offenders = sourceFiles().flatMap((f) => {
      const text = readFileSync(join(appDir, f), 'utf8')
      return RETIRED_SOURCE_NAMES.filter((n) => text.includes(n)).map(
        (n) => `${f}: ${n}`
      )
    })
    expect(offenders).toEqual([])
  })
})
