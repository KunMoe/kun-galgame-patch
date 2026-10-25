import { describe, expect, it } from 'vitest'
import { decodeText, detectEncoding, looksBinary } from './textEncoding'

const hex = (s: string) =>
  Uint8Array.from(s.match(/../g)!.map((b) => parseInt(b, 16)))

const SJIS = hex(
  '8b4e93ae82b582c882a28fea8d8782cd82b182bf82e782f082a893c782dd82ad82be82b382a28142'
)
const GBK = hex(
  'bde2d1b9b5bdd3cecfb7c4bfc2bcb8b2b8c7bcb4bfc9a3acc8e7d3f6c2d2c2ebc7ebcab9d3c3d7aac7f8b9a4bedfa1a3'
)
const BIG5 = hex(
  'b14ec0c9aed7b8d1c0a3c159a8ecb943c0b8b8eaaec6a7a8a141c2d0bb5cadeca96cc0c9aed7abe1b0f5a6e6b943c0b8a143'
)

const utf16le = (text: string) => {
  const out = new Uint8Array(text.length * 2)
  for (let i = 0; i < text.length; i++) {
    out[i * 2] = text.charCodeAt(i) & 0xff
    out[i * 2 + 1] = text.charCodeAt(i) >> 8
  }
  return out
}

describe('detectEncoding', () => {
  it('reads the legacy CJK encodings a game readme ships in', async () => {
    expect(await detectEncoding(SJIS)).toBe('shift_jis')
    expect(decodeText(SJIS, 'shift_jis')).toBe(
      '起動しない場合はこちらをお読みください。'
    )
    expect(await detectEncoding(GBK)).toBe('gb18030')
    expect(decodeText(GBK, 'gb18030')).toBe(
      '解压到游戏目录覆盖即可，如遇乱码请使用转区工具。'
    )
    expect(await detectEncoding(BIG5)).toBe('big5')
  })

  it('prefers UTF-8, including ASCII and a multibyte char cut at the end', async () => {
    const utf8 = new TextEncoder().encode('[Config]\r\nTitle=魔法少女')
    expect(await detectEncoding(utf8)).toBe('utf-8')
    expect(await detectEncoding(utf8.subarray(0, utf8.length - 1))).toBe(
      'utf-8'
    )
    expect(
      await detectEncoding(new TextEncoder().encode('Width=1280\r\n'))
    ).toBe('utf-8')
  })

  it('finds UTF-16 by its BOM or by where the zero bytes sit', async () => {
    const body = utf16le('[System]\r\nVersion=1.02\r\n')
    expect(await detectEncoding(body)).toBe('utf-16le')
    const bom = new Uint8Array([0xff, 0xfe, ...body])
    expect(await detectEncoding(bom)).toBe('utf-16le')
    expect(decodeText(bom, 'utf-16le')).toBe('[System]\r\nVersion=1.02\r\n')
  })
})

describe('looksBinary', () => {
  it('flags a NUL outside UTF-16', () => {
    expect(looksBinary(new Uint8Array([0x4d, 0x5a, 0x90, 0x00]), 'utf-8')).toBe(
      true
    )
    expect(looksBinary(utf16le('abc'), 'utf-16le')).toBe(false)
    expect(looksBinary(SJIS, 'shift_jis')).toBe(false)
  })
})
