// All keys are snake_case to match the Go API responses verbatim — see
// apps/api/internal/about/model.

interface KunPostMetadata {
  title: string
  banner: string
  date: string
  description: string
  text_count: number
  slug: string
  path: string
  directory: string
}

interface KunPostFrontmatter {
  title: string
  banner: string
  description: string
  date: string
  author_uid?: number
  author_name: string
  author_avatar: string
  author_homepage?: string
  pin?: boolean
}

// One heading from the rendered post — id matches the corresponding
// `<h1|h2|h3 id="...">` attribute, so the frontend just needs to scrollIntoView.
interface KunTOCItem {
  id: string
  text: string
  level: number
}

interface KunPostDetail {
  slug: string
  html: string
  toc: KunTOCItem[]
  frontmatter: KunPostFrontmatter
  prev: KunPostMetadata | null
  next: KunPostMetadata | null
}

interface KunTreeNode {
  name: string
  label: string
  path: string
  children?: KunTreeNode[]
  type: 'file' | 'directory'
}

// /about/posts response: flat list (for the index card grid) + tree (for the
// sidebar on the detail page) returned together.
interface KunPostsResponse {
  items: KunPostMetadata[]
  tree: KunTreeNode
}
