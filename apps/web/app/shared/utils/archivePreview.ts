import type { ArchiveNode } from './archiveTree'
import { splitExtension } from './archiveTree'

export type ArchivePreviewKind = 'text' | 'image' | 'audio' | 'save'

export interface ArchivePreviewRule {
  kind: ArchivePreviewKind
  max_bytes: number
  truncate: boolean
  extensions: string[]
}

export interface ArchivePreview {
  kind: ArchivePreviewKind
  truncated: boolean
  maxBytes: number
}

export type ArchivePreviewResolver = (
  node: ArchiveNode
) => ArchivePreview | null

export const createPreviewResolver = (
  rules: ArchivePreviewRule[]
): ArchivePreviewResolver => {
  const byExtension = new Map(
    rules.flatMap((rule) => rule.extensions.map((ext) => [ext, rule] as const))
  )
  return (node) => {
    if (node.dir) return null
    const ext = splitExtension(node.name)[1].slice(1).toLowerCase()
    const rule = byExtension.get(ext)
    if (!rule) return null
    const over = node.size > rule.max_bytes
    if (over && !rule.truncate) return null
    return { kind: rule.kind, truncated: over, maxBytes: rule.max_bytes }
  }
}

const FAILURE_BY_STATUS: Record<number, string> = {
  403: '请先确认显示 R18 内容',
  404: '文件不存在，可能已被替换',
  410: '资源已下架',
  429: '打开得太频繁了，请稍后再试'
}

export const readFailure = async (res: Response) => {
  const body = (await res.json().catch(() => null)) as {
    message?: string
  } | null
  if (body?.message && /[㐀-鿿]/.test(body.message)) {
    return body.message
  }
  return FAILURE_BY_STATUS[res.status] ?? `读取失败（${res.status}）`
}

export const archiveFileEndpoint = (resourceId: number, path: string) =>
  `/resource/${resourceId}/files/content?path=${encodeURIComponent(path)}`
