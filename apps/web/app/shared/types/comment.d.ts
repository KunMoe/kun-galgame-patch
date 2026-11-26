// PatchSummary mirrors apps/api/internal/patch/model PatchSummary — a compact
// projection of the owning patch attached to comment / resource rows that come
// from the global lists.
interface PatchSummary {
  id: number
  vndb_id: string
  // Legacy absolute banner URL — empty post wiki→catalog migration; kept as a
  // resolveBannerUrl fallback. effective_banner_hash is the current cover source
  // (image_service hash of the pinned cover) the resolver prefers.
  banner: string
  effective_banner_hash?: string
  name: KunLanguage
}

// PatchComment is one comment as the mixed feeds print it — home, the global
// feed, a profile, the search lane, the admin queue. `id` is a COMMUNITY POST
// id, and `link` is the permalink the server built from the post's anchor: it
// is the only thing that knows which of the two walls the comment is on, so
// never rebuild it from galgame_id / resource_id here.
interface PatchComment {
  id: number
  thread_id: number
  user: KunUser
  content: string
  content_html: string
  galgame_id: number
  resource_id?: number | null
  like_count: number
  created: string
  link: string
  patch?: PatchSummary | null
  status?: number
}

// A keyset page. There is no total and no page number: the upstream faces
// answer a cursor, so these lists load more rather than paginate.
interface PatchCommentFeed {
  items: PatchComment[]
  next_cursor: string
}

// Who removed a tombstone; absent on one older than community 1.4.0. Only
// moderator and review can be restored.
type CommentDeleteSource = 'author' | 'moderator' | 'review' | 'trust' | 'purge'

// PatchPageComment is one comment on a wall. Replies are NOT nested: a page
// lists top-level comments and their replies separately, each reply naming its
// root by id (see useCommentList's `groups`).
interface PatchPageComment {
  id: number
  thread_id: number
  // The keyset of a root's reply pages.
  post_number: number
  content: string
  content_html: string
  galgame_id: number
  resource_id?: number | null
  user: KunUser
  // The person being replied to, when the post is a reply.
  target_user?: KunUser | null
  parent_comment_id: number | null
  root_comment_id: number | null
  // Top-level comments only: visible replies, loaded or not.
  reply_count?: number
  // Set while this site has the comment pinned on its wall.
  pinned_at: string | null
  like_count: number
  is_liked: boolean
  created: string
  // RFC3339 when the post has been edited, null otherwise. Drives 已编辑.
  edited: string | null
  // The LATEST edit was a moderator's, not the author's — labelled distinctly.
  edited_by_moderator: boolean
  // 0=visible 1=held 2=deleted, as the community service numbers them.
  status: number
  // A tombstone: the post keeps its number so the wall's numbering never
  // collapses, and renders as a stub with no content.
  deleted: boolean
  delete_source?: CommentDeleteSource
  // Held for review by the newcomer sandbox. Only its own author ever sees it.
  held: boolean
}

interface PatchCommentPage {
  // 0 until somebody comments: the wall's thread is created by the first
  // comment, not by looking at the page.
  thread_id: number
  // First page only, newest pin first. The same comments also come back in
  // `posts` at their own place.
  pinned: PatchPageComment[]
  // Top-level comments.
  posts: PatchPageComment[]
  // The first replies of every listed top-level comment, pinned ones included.
  replies: PatchPageComment[]
  // Bound to the sort that minted it.
  next_cursor: string
  total: number
}

interface PatchCommentReplies {
  posts: PatchPageComment[]
  next_cursor: string
}

// 0=muted 1=normal 2=tracking 3=watching, 4=watching first post (anchor-level only).
type CommentNotificationLevel = 0 | 1 | 2 | 3 | 4

interface CommentWallState {
  thread_id: number
  subscribed: boolean
  notification_level: CommentNotificationLevel
  unread_count: number
}

interface CommentUnreadItem {
  thread_id: number
  link: string
  title: string
  label: string
  patch_id?: number
  resource_id?: number
  unread_count: number
  notification_level: CommentNotificationLevel
  last_posted_at: string
}

interface CommentUnreadResult {
  items: CommentUnreadItem[]
  next_cursor: string
}

type HomeComment = PatchComment
