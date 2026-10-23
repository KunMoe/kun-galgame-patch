// 'unknown' is a 7z: its header, the only place that says whether it is
// encrypted, sits at the end of the file.
export type ArchiveEncryption = 'encrypted' | 'plain' | 'unknown'

export const ARCHIVE_SNIFF_BYTES = 64 * 1024

const startsWith = (b: Uint8Array, sig: number[]) =>
  b.length >= sig.length && sig.every((v, i) => b[i] === v)

const ZIP_LOCAL = [0x50, 0x4b, 0x03, 0x04]
const RAR4 = [0x52, 0x61, 0x72, 0x21, 0x1a, 0x07, 0x00]
const RAR5 = [0x52, 0x61, 0x72, 0x21, 0x1a, 0x07, 0x01, 0x00]
const SEVEN_ZIP = [0x37, 0x7a, 0xbc, 0xaf, 0x27, 0x1c]

const zipEncryption = (b: Uint8Array): ArchiveEncryption => {
  const v = new DataView(b.buffer, b.byteOffset, b.byteLength)
  let pos = 0
  while (pos + 30 <= b.length && v.getUint32(pos, true) === 0x04034b50) {
    const flags = v.getUint16(pos + 6, true)
    if (flags & 0x1) return 'encrypted'
    const packed = v.getUint32(pos + 18, true)
    // A data descriptor or ZIP64 entry hides its size here, so the next
    // header cannot be found.
    if (flags & 0x8 || packed === 0xffffffff) break
    pos +=
      30 + v.getUint16(pos + 26, true) + v.getUint16(pos + 28, true) + packed
  }
  return 'plain'
}

const rar4Encryption = (b: Uint8Array): ArchiveEncryption => {
  const v = new DataView(b.buffer, b.byteOffset, b.byteLength)
  let pos = RAR4.length
  while (pos + 7 <= b.length) {
    const type = b[pos + 2]
    const flags = v.getUint16(pos + 3, true)
    const size = v.getUint16(pos + 5, true)
    if (type === 0x73 && flags & 0x0080) return 'encrypted'
    if (type === 0x74) return flags & 0x0004 ? 'encrypted' : 'plain'
    if (size < 7) break
    const add =
      flags & 0x8000 && pos + 11 <= b.length ? v.getUint32(pos + 7, true) : 0
    pos += size + add
  }
  return 'plain'
}

const vint = (b: Uint8Array, pos: number): [number, number] => {
  let value = 0
  let mul = 1
  for (let i = pos; i < Math.min(b.length, pos + 10); i++) {
    const byte = b[i]!
    value += (byte & 0x7f) * mul
    mul *= 128
    if (!(byte & 0x80)) return [value, i + 1]
  }
  return [-1, b.length]
}

const rar5Encryption = (b: Uint8Array): ArchiveEncryption => {
  let pos = RAR5.length
  while (pos + 5 < b.length) {
    const [headSize, headStart] = vint(b, pos + 4)
    if (headSize <= 0) break
    const headEnd = headStart + headSize
    const [type, afterType] = vint(b, headStart)
    if (type === 4) return 'encrypted'
    const [flags, afterFlags] = vint(b, afterType)
    let p = afterFlags
    let extraSize = 0
    let dataSize = 0
    if (flags & 0x1) [extraSize, p] = vint(b, p)
    if (flags & 0x2) [dataSize, p] = vint(b, p)
    if (type === 2) {
      if (headEnd > b.length) break
      let r = headEnd - extraSize
      while (r < headEnd) {
        const [recSize, recStart] = vint(b, r)
        if (recSize <= 0) break
        if (vint(b, recStart)[0] === 1) return 'encrypted'
        r = recStart + recSize
      }
      return 'plain'
    }
    pos = headEnd + dataSize
  }
  return 'plain'
}

export const archiveEncryption = (head: Uint8Array): ArchiveEncryption => {
  if (startsWith(head, ZIP_LOCAL)) return zipEncryption(head)
  if (startsWith(head, RAR5)) return rar5Encryption(head)
  if (startsWith(head, RAR4)) return rar4Encryption(head)
  if (startsWith(head, SEVEN_ZIP)) return 'unknown'
  return 'plain'
}

export const sniffArchiveEncryption = async (
  file: Blob
): Promise<ArchiveEncryption> =>
  archiveEncryption(
    new Uint8Array(await file.slice(0, ARCHIVE_SNIFF_BYTES).arrayBuffer())
  )
