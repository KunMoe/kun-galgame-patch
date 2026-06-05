// Admin /admin/doc shapes — matches apps/api/internal/doc/model AdminItem /
// AdminDetail. (Public /doc shapes reuse the KunPosts* types in about.d.ts.)

interface DocAdminItem {
  id: number
  category: string
  slug: string // full "<category>/<name>"
  name: string // within-category name
  title: string
  status: number // 0 = draft, 1 = published
  pin: boolean
  view: number
  date: string
  banner: string
}

interface DocAdminDetail {
  id: number
  category: string
  slug: string
  name: string
  title: string
  description: string
  content: string
  banner_image_hash: string
  banner: string
  date: string
  status: number
  pin: boolean
  view: number
}
