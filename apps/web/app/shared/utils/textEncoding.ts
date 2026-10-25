export const TEXT_ENCODINGS = [
  { value: 'utf-8', label: 'UTF-8' },
  { value: 'shift_jis', label: 'Shift-JIS' },
  { value: 'gb18030', label: 'GBK' },
  { value: 'big5', label: 'Big5' },
  { value: 'euc-kr', label: 'EUC-KR' },
  { value: 'euc-jp', label: 'EUC-JP' },
  { value: 'utf-16le', label: 'UTF-16 LE' },
  { value: 'utf-16be', label: 'UTF-16 BE' }
] as const

export type TextEncoding = (typeof TEXT_ENCODINGS)[number]['value']

const SAMPLE_BYTES = 64 * 1024

const CHARDET_NAMES: Record<string, TextEncoding> = {
  Shift_JIS: 'shift_jis',
  GB18030: 'gb18030',
  Big5: 'big5',
  'EUC-KR': 'euc-kr',
  'EUC-JP': 'euc-jp',
  'UTF-16LE': 'utf-16le',
  'UTF-16BE': 'utf-16be'
}

const bomEncoding = (b: Uint8Array): TextEncoding | null => {
  if (b[0] === 0xef && b[1] === 0xbb && b[2] === 0xbf) return 'utf-8'
  if (b[0] === 0xff && b[1] === 0xfe) return 'utf-16le'
  if (b[0] === 0xfe && b[1] === 0xff) return 'utf-16be'
  return null
}

const utf16Encoding = (b: Uint8Array): TextEncoding | null => {
  const pairs = Math.floor(b.length / 2)
  if (pairs < 2) return null
  let evenZeros = 0
  let oddZeros = 0
  for (let i = 0; i + 1 < b.length; i += 2) {
    if (b[i] === 0) evenZeros++
    if (b[i + 1] === 0) oddZeros++
  }
  if (oddZeros > pairs * 0.3 && evenZeros < pairs * 0.05) return 'utf-16le'
  if (evenZeros > pairs * 0.3 && oddZeros < pairs * 0.05) return 'utf-16be'
  return null
}

const isUtf8 = (b: Uint8Array) => {
  try {
    new TextDecoder('utf-8', { fatal: true }).decode(b, { stream: true })
    return true
  } catch {
    return false
  }
}

const guessLegacy = async (b: Uint8Array): Promise<TextEncoding> => {
  const { analyse } = await import('chardet')
  for (const match of analyse(b)) {
    const known = CHARDET_NAMES[match.name]
    if (known) return known
  }
  return 'shift_jis'
}

export const detectEncoding = async (
  bytes: Uint8Array
): Promise<TextEncoding> => {
  const sample = bytes.subarray(0, SAMPLE_BYTES)
  return (
    bomEncoding(sample) ??
    utf16Encoding(sample) ??
    (isUtf8(sample) ? 'utf-8' : await guessLegacy(sample))
  )
}

export const decodeText = (bytes: Uint8Array, encoding: TextEncoding) =>
  new TextDecoder(encoding).decode(bytes)

export const looksBinary = (bytes: Uint8Array, encoding: TextEncoding) =>
  !encoding.startsWith('utf-16') && bytes.subarray(0, 8192).includes(0)
