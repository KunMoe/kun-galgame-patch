import { describe, expect, it } from 'vitest'
import { archiveEncryption } from './archiveEncryption'

// The first 160 bytes of zips 7-Zip 26 wrote: plain, ZipCrypto, AES-256, and
// ZipCrypto behind an unencrypted directory entry.
const ZIP = {
  plain:
    'UEsDBBQAAAAIAHc6V13XK1AKJAAAAGAJAAAFAAAAYS50eHTtxrEJACAMBMBVMpyCxUPAxvUdIu1ddWcnXa9vVrm7u7v7/B9QSwMEFAAAAAAAdzpXXQAAAAAAAAAAAAAAAAQAAABkaXIvUEsDBAoAAAAAAHc6V10fCOpGAgAAAAIAAAAJAAAAZGlyL2IudHh0eApQSwECPwMUAAAACAB3Og==',
  zipcrypto:
    'UEsDBBQAAQAIAHc6V13XK1AKMAAAAGAJAAAFAAAAYS50eHRI/IJ0jGUJNNvRP5yJ8hHAhf4x5IlKX7wqg9S2HJrXROsTBwuhSlpTCbvrkqahn65QSwMEFAAAAAAAdzpXXQAAAAAAAAAAAAAAAAQAAABkaXIvUEsDBBQAAQAAAHc6V10fCOpGDgAAAAIAAAAJAAAAZGlyL2IudHh0bA4IOg==',
  aes: 'UEsDBDMAAQBjAHc6V10AAAAAQAAAAGAJAAAFAAsAYS50eHQBmQcAAgBBRQMIAKij8suHP5OGkT880Al7zt6fMUbTYG5yChPqszvlnzYA3z5zcWze2G8cwFOpIhZrpmKufR+S9StDPkxw4TP9e3ZQSwMEFAAAAAAAdzpXXQAAAAAAAAAAAAAAAAQAAABkaXIvUEsDBDMAAQBjAHc6V10AAA==',
  dirFirst:
    'UEsDBBQAAAAAAHc6V10AAAAAAAAAAAAAAAAEAAAAZGlyL1BLAwQUAAEAAAB3OlddHwjqRg4AAAACAAAACQAAAGRpci9iLnR4dPGGFFfBnEAO5ddyGiNiUEsBAj8DFAAAAAAAdzpXXQAAAAAAAAAAAAAAAAQAJAAAAAAAAAAQgO1BAAAAAGRpci8KACAAAAAAAAEAGABkbK1o4GLdAQAAAA=='
}

const b64 = (s: string) => Uint8Array.from(atob(s), (c) => c.charCodeAt(0))

const u16 = (n: number) => [n & 0xff, n >> 8]
const u32 = (n: number) => [
  n & 0xff,
  (n >> 8) & 0xff,
  (n >> 16) & 0xff,
  n >>> 24
]
const vint = (n: number) => {
  const out: number[] = []
  do {
    out.push((n & 0x7f) | (n > 0x7f ? 0x80 : 0))
    n = Math.floor(n / 128)
  } while (n > 0)
  return out
}

// No tool here writes RAR, so the headers are built by hand; the walk ignores
// CRCs, so they are zero.
const rar4Block = (type: number, flags: number, body: number[], add = 0) => {
  const fields = flags & 0x8000 ? [...u32(add), ...body] : body
  return [0, 0, type, ...u16(flags), ...u16(7 + fields.length), ...fields]
}
const rar4 = (...blocks: number[][]) =>
  new Uint8Array([0x52, 0x61, 0x72, 0x21, 0x1a, 0x07, 0x00, ...blocks.flat()])
const rar4File = (flags: number) =>
  rar4Block(0x74, 0x8000 | flags, [
    ...u32(5),
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0x14,
    0x30,
    ...u16(1),
    0,
    0,
    0,
    0,
    0x61
  ])

const rar5Block = (
  type: number,
  flags: number,
  body: number[],
  extra: number[] = [],
  data = 0
) => {
  const head = [
    ...vint(type),
    ...vint(flags),
    ...(flags & 0x1 ? vint(extra.length) : []),
    ...(flags & 0x2 ? vint(data) : []),
    ...body,
    ...extra
  ]
  return [0, 0, 0, 0, ...vint(head.length), ...head, ...new Array(data).fill(0)]
}
const rar5 = (...blocks: number[][]) =>
  new Uint8Array([
    0x52,
    0x61,
    0x72,
    0x21,
    0x1a,
    0x07,
    0x01,
    0x00,
    ...blocks.flat()
  ])
const rar5Main = rar5Block(1, 0, [0])
const rar5File = (extra: number[]) =>
  rar5Block(2, 0x1 | 0x2, [0, 5, 0x20, 0, 0, 1, 0x61], extra, 5)
const encryptionRecord = [...vint(3), 0x01, 0, 0]
const hashRecord = [...vint(3), 0x02, 0, 0]

describe('archiveEncryption', () => {
  it('reads the zip encryption bit of every entry it can reach', () => {
    expect(archiveEncryption(b64(ZIP.plain))).toBe('plain')
    expect(archiveEncryption(b64(ZIP.zipcrypto))).toBe('encrypted')
    expect(archiveEncryption(b64(ZIP.aes))).toBe('encrypted')
    expect(archiveEncryption(b64(ZIP.dirFirst))).toBe('encrypted')
  })

  it('reads RAR4 file and archive-wide encryption', () => {
    expect(
      archiveEncryption(
        rar4(rar4Block(0x73, 0, [0, 0, 0, 0, 0, 0]), rar4File(0))
      )
    ).toBe('plain')
    expect(
      archiveEncryption(
        rar4(rar4Block(0x73, 0, [0, 0, 0, 0, 0, 0]), rar4File(0x4))
      )
    ).toBe('encrypted')
    expect(
      archiveEncryption(rar4(rar4Block(0x73, 0x80, [0, 0, 0, 0, 0, 0])))
    ).toBe('encrypted')
  })

  it('reads RAR5 header encryption and file encryption records', () => {
    expect(archiveEncryption(rar5(rar5Main, rar5File(hashRecord)))).toBe(
      'plain'
    )
    expect(archiveEncryption(rar5(rar5Main, rar5File(encryptionRecord)))).toBe(
      'encrypted'
    )
    expect(
      archiveEncryption(
        rar5(rar5Main, rar5File([...hashRecord, ...encryptionRecord]))
      )
    ).toBe('encrypted')
    expect(archiveEncryption(rar5(rar5Block(4, 0, [0, 0, 15])))).toBe(
      'encrypted'
    )
    // An archive comment is a service header ahead of the first file.
    expect(
      archiveEncryption(
        rar5(
          rar5Main,
          rar5Block(3, 0x2, [0, 0, 0, 0, 3, 0x43, 0x4d, 0x54], [], 9),
          rar5File(encryptionRecord)
        )
      )
    ).toBe('encrypted')
  })

  it('cannot tell a 7z from its head', () => {
    expect(
      archiveEncryption(
        new Uint8Array([0x37, 0x7a, 0xbc, 0xaf, 0x27, 0x1c, 0, 4])
      )
    ).toBe('unknown')
  })

  it('passes anything else through', () => {
    expect(archiveEncryption(new Uint8Array([0x4d, 0x5a, 0x90, 0]))).toBe(
      'plain'
    )
    expect(archiveEncryption(new Uint8Array())).toBe('plain')
  })
})
