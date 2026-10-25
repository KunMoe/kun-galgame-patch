export interface ArchiveEntry {
  path: string
  size: number
  files?: number
  items?: number
}

export type ArchiveState = 'ready' | 'pending' | 'unsupported' | 'failed'

export interface ArchiveListing {
  state: ArchiveState
  file_count: number
  dir_count: number
  total_bytes: number
  entries: ArchiveEntry[]
  truncated: boolean
}

export interface ArchiveNode {
  name: string
  path: string
  parent: string
  dir: boolean
  size: number
  files: number
  items: number
  children: ArchiveNode[]
}

const collator = new Intl.Collator('zh-CN', {
  numeric: true,
  sensitivity: 'base'
})

const compareNodes = (a: ArchiveNode, b: ArchiveNode) =>
  a.dir === b.dir ? collator.compare(a.name, b.name) : a.dir ? -1 : 1

const settle = (node: ArchiveNode) => {
  for (const child of node.children) {
    if (child.dir) settle(child)
    node.size += child.size
    node.files += child.files
  }
  node.items = Math.max(node.items, node.children.length)
  node.children.sort(compareNodes)
}

export const isUnloaded = (node: ArchiveNode) =>
  node.items > node.children.length

export const buildArchiveTree = (entries: ArchiveEntry[]): ArchiveNode => {
  const root: ArchiveNode = {
    name: '',
    path: '',
    parent: '',
    dir: true,
    size: 0,
    files: 0,
    items: 0,
    children: []
  }
  const dirs = new Map<string, ArchiveNode>([['', root]])

  const ensureDir = (path: string): ArchiveNode => {
    const found = dirs.get(path)
    if (found) return found
    const cut = path.lastIndexOf('/')
    const parent = ensureDir(cut < 0 ? '' : path.slice(0, cut))
    const node: ArchiveNode = {
      name: path.slice(cut + 1),
      path,
      parent: parent.path,
      dir: true,
      size: 0,
      files: 0,
      items: 0,
      children: []
    }
    parent.children.push(node)
    dirs.set(path, node)
    return node
  }

  for (const entry of entries) {
    if (entry.path.endsWith('/')) {
      const dir = ensureDir(entry.path.slice(0, -1))
      if (entry.items) {
        dir.size = entry.size
        dir.files = entry.files ?? 0
        dir.items = entry.items
      }
      continue
    }
    const cut = entry.path.lastIndexOf('/')
    const parent = ensureDir(cut < 0 ? '' : entry.path.slice(0, cut))
    parent.children.push({
      name: entry.path.slice(cut + 1),
      path: entry.path,
      parent: parent.path,
      dir: false,
      size: entry.size,
      files: 1,
      items: 0,
      children: []
    })
  }

  settle(root)
  return root
}

export const findArchiveNode = (
  root: ArchiveNode,
  path: string
): ArchiveNode | null => {
  let node: ArchiveNode | undefined = root
  for (const name of path ? path.split('/') : []) {
    node = node.children.find((c) => c.name === name)
    if (!node) return null
  }
  return node
}

export const splitExtension = (name: string): [string, string] => {
  const dot = name.lastIndexOf('.')
  return dot > 0 ? [name.slice(0, dot), name.slice(dot)] : [name, '']
}

type FileKindKey =
  | 'app'
  | 'library'
  | 'config'
  | 'document'
  | 'image'
  | 'audio'
  | 'video'
  | 'package'
  | 'script'
  | 'font'

interface FileKind {
  icon: string
  tone: string
  label: string
}

const FILE_KINDS: Record<FileKindKey, FileKind> = {
  app: { icon: 'lucide:app-window', tone: 'text-success', label: '应用程序' },
  library: {
    icon: 'lucide:file-cog',
    tone: 'text-default-500',
    label: '系统组件'
  },
  config: {
    icon: 'lucide:file-sliders',
    tone: 'text-default-500',
    label: '配置文件'
  },
  document: {
    icon: 'lucide:file-text',
    tone: 'text-default-500',
    label: '文档'
  },
  image: { icon: 'lucide:file-image', tone: 'text-secondary', label: '图片' },
  audio: { icon: 'lucide:file-music', tone: 'text-info', label: '音频' },
  video: {
    icon: 'lucide:file-video-camera',
    tone: 'text-danger',
    label: '视频'
  },
  package: {
    icon: 'lucide:file-archive',
    tone: 'text-warning',
    label: '资源封包'
  },
  script: { icon: 'lucide:file-code', tone: 'text-info', label: '脚本' },
  font: { icon: 'lucide:file-type', tone: 'text-default-500', label: '字体' }
}

const EXTENSIONS: Record<FileKindKey, string[]> = {
  app: ['exe', 'msi', 'bat', 'cmd', 'com', 'apk', 'app', 'sh', 'command'],
  library: ['dll', 'sys', 'so', 'dylib', 'ocx', 'drv', 'ax', 'node'],
  config: ['ini', 'cfg', 'conf', 'json', 'xml', 'yaml', 'yml', 'toml', 'inf'],
  document: ['txt', 'md', 'rtf', 'doc', 'docx', 'pdf', 'htm', 'html', 'log'],
  image: [
    'png',
    'jpg',
    'jpeg',
    'gif',
    'bmp',
    'webp',
    'tga',
    'ico',
    'psd',
    'dds',
    'tif',
    'tiff',
    'svg',
    'avif'
  ],
  audio: ['ogg', 'mp3', 'wav', 'flac', 'm4a', 'opus', 'wma', 'mid', 'aac'],
  video: ['mp4', 'webm', 'mkv', 'avi', 'wmv', 'mpg', 'mpeg', 'mov', 'ogv'],
  package: [
    'zip',
    'rar',
    '7z',
    'xp3',
    'pak',
    'arc',
    'dat',
    'npa',
    'pfs',
    'cpk',
    'rpa',
    'rgss3a',
    'rgss2a',
    'rgssad',
    'ypf',
    'pac',
    'bin',
    'cab',
    'iso',
    'unity3d',
    'assets',
    'bundle'
  ],
  script: ['js', 'ks', 'tjs', 'rb', 'py', 'lua', 'rpy', 'rpyc', 'cs', 'ps1'],
  font: ['ttf', 'otf', 'ttc', 'woff', 'woff2', 'fnt']
}

const KIND_BY_EXTENSION = new Map<string, FileKind>(
  (Object.keys(EXTENSIONS) as FileKindKey[]).flatMap((key) =>
    EXTENSIONS[key].map((ext): [string, FileKind] => [ext, FILE_KINDS[key]])
  )
)

export const fileKindOf = (name: string): FileKind => {
  const ext = splitExtension(name)[1].slice(1).toLowerCase()
  return (
    KIND_BY_EXTENSION.get(ext) ?? {
      icon: 'lucide:file',
      tone: 'text-default-400',
      label: ext ? `${ext.toUpperCase()} 文件` : '文件'
    }
  )
}

const BYTE_UNITS = ['KB', 'MB', 'GB', 'TB']

export const formatBytes = (bytes: number): string => {
  if (bytes < 1024) return `${bytes} B`
  let value = bytes / 1024
  let unit = 0
  while (value >= 1024 && unit < BYTE_UNITS.length - 1) {
    value /= 1024
    unit++
  }
  const text = value >= 100 ? Math.round(value).toString() : value.toFixed(1)
  return `${text.replace(/\.0$/, '')} ${BYTE_UNITS[unit]}`
}
