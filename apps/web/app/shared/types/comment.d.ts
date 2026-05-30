// PatchSummary mirrors apps/api/internal/patch/model PatchSummary — a compact
// projection of the owning patch attached to comment / resource rows that come
// from the global lists.
interface PatchSummary {
  id: number
  vndb_id: string
  banner: string
  name: KunLanguage
}

// PatchComment is used for home/global comment summaries.
interface PatchComment {
  id: number
  user: KunUser
  content: string
  content_html: string
  galgame_id: number
  like_count: number
  created: Date | string
  // Populated only by /api/v1/comment and /api/v1/home; null on per-patch lists.
  patch?: PatchSummary | null
  // Moderation state (comment-verify): 0 = approved, 1 = pending. Surfaced in
  // the admin comment list so moderators can review/approve the queue.
  status?: number
}

// PatchPageComment is a top-level or reply comment returned from
// GET /api/v1/patch/:id/comment. is_liked is filled per-request from the
// current user's like relation (false for anonymous callers); content_html is
// the markdown-rendered content with @mention support.
interface PatchPageComment {
  id: number
  content: string
  content_html: string
  is_liked: boolean
  like_count: number
  parent_id: number | null
  user_id: number
  galgame_id: number
  created: string
  updated: string
  reply: PatchPageComment[]
  user: KunUser
  quoted_content?: string | null
  quoted_username?: string | null
  // Moderation state (comment-verify): 0 = approved/visible, 1 = pending review.
  // Public list endpoints only ever return status=0; the create response may be
  // 1 when the comment was held for admin approval.
  status?: number
}

type HomeComment = PatchComment
