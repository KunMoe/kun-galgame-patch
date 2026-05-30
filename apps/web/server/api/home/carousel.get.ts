import fs from 'node:fs'
import path from 'node:path'

const POSTS_PATH = path.join(process.cwd(), 'posts')

// Local copy of the shape: the app-side `HomeCarouselMetadata` lives in
// app/shared/types/home.d.ts as a global ambient type, which the Nitro server
// tsconfig does not include — so this server route can't see it. Defining it
// here keeps the producer self-typed without exporting/modularizing the global.
interface HomeCarouselMetadata {
  title: string
  banner: string
  description: string
  date: string
  authorName: string
  authorAvatar: string
  pin: boolean
  directory: string
  link: string
}

const parseFrontmatter = (raw: string): Record<string, unknown> => {
  const match = raw.match(/^---\r?\n([\s\S]*?)\r?\n---/)
  if (!match) return {}

  const data: Record<string, unknown> = {}
  const body = match[1] ?? ''
  for (const line of body.split(/\r?\n/)) {
    const m = line.match(/^([A-Za-z0-9_-]+):\s*(.*)$/)
    if (!m) continue
    const key = m[1]!
    let value: unknown = (m[2] ?? '').trim()
    if (typeof value === 'string') {
      // strip surrounding quotes
      if (
        (value.startsWith('"') && value.endsWith('"')) ||
        (value.startsWith("'") && value.endsWith("'"))
      ) {
        value = value.slice(1, -1)
      } else if (value === 'true') value = true
      else if (value === 'false') value = false
      else if (value !== '' && !isNaN(Number(value))) value = Number(value)
    }
    data[key] = value
  }
  return data
}

const traverseDirectory = (
  currentPath: string,
  posts: HomeCarouselMetadata[]
) => {
  const files = fs.readdirSync(currentPath)

  for (const file of files) {
    const filePath = path.join(currentPath, file)
    const stat = fs.statSync(filePath)

    if (stat.isDirectory()) {
      traverseDirectory(filePath, posts)
    } else if (file.endsWith('.mdx')) {
      const raw = fs.readFileSync(filePath, 'utf8')
      const data = parseFrontmatter(raw) as {
        title?: string
        banner?: string
        description?: string
        date?: string
        authorName?: string
        authorAvatar?: string
        pin?: boolean
      }

      if (!data.pin) continue

      const parentDirectory = path.basename(path.dirname(filePath))
      const fileName = path.basename(file, '.mdx')

      posts.push({
        title: data.title ?? '',
        banner: data.banner ?? '',
        description: data.description ?? '',
        date: new Date(data.date ?? Date.now()).toISOString(),
        authorName: data.authorName ?? '',
        authorAvatar: data.authorAvatar ?? '',
        pin: !!data.pin,
        directory: parentDirectory,
        link: `/about/${parentDirectory}/${fileName}`
      })
    }
  }
}

export default defineEventHandler((): HomeCarouselMetadata[] => {
  if (!fs.existsSync(POSTS_PATH)) {
    return []
  }

  const posts: HomeCarouselMetadata[] = []
  traverseDirectory(POSTS_PATH, posts)

  return posts.sort(
    (a, b) => new Date(b.date).getTime() - new Date(a.date).getTime()
  )
})
