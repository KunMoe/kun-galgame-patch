// Matches apps/api/internal/blog/model. Banner is a derived image_service CDN
// URL; content_html is server-rendered markdown.

interface BlogUser {
  id: number
  name: string
  avatar: string
  avatar_image_hash: string
  roles: string[]
}

// One entry in the public/admin list (no body).
interface BlogCard {
  id: number
  title: string
  summary: string
  banner: string
  status: number // 0 = draft, 1 = published
  pin: boolean
  view: number
  user: BlogUser | null
  created: string
  updated: string
}

// Public GET /blog/:id (rendered HTML).
interface BlogDetail {
  id: number
  title: string
  summary: string
  content_html: string
  toc: { id: string; text: string; level: number }[]
  banner: string
  status: number
  pin: boolean
  view: number
  user: BlogUser | null
  created: string
  updated: string
}

// Admin GET /admin/blog/:id (raw markdown + hash for the editor).
interface BlogEdit {
  id: number
  title: string
  summary: string
  content: string
  banner_image_hash: string
  banner: string
  status: number
  pin: boolean
  view: number
  created: string
  updated: string
}
