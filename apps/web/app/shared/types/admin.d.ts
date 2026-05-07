// GET /api/v1/admin/stats/sum
interface SumData {
  user_count: number
  galgame_count: number
  resource_count: number
  comment_count: number
}

// GET /api/v1/admin/stats?days=N
interface OverviewData {
  new_user: number
  new_active_user: number
  new_galgame: number
  new_resource: number
  new_comment: number
}

// GET /api/v1/admin/user — per admin/repository AdminUserView.
interface AdminUser {
  id: number
  name: string
  email: string
  bio: string
  avatar: string
  role: number
  status: number
  moemoepoint: number
  daily_image_count: number
  daily_check_in: number
  daily_upload_size: number
  follower_count: number
  following_count: number
  patch_count: number
  resource_count: number
  created: Date | string
}

// GET /api/v1/admin/creator — per admin/repository CreatorApplicationItem.
interface AdminCreator {
  id: number
  content: string
  status: number
  sender: KunUser | null
  resource_count: number
  created: Date | string
}

// NOTE: AdminGalgame and the corresponding /admin/galgame page are deprecated per D12.
// Patch management is handled via /admin/orphans and /admin/resource.

interface AdminResourceItem {
  id: number
  name: string
  model_name: string
  size: string
  type: string[]
  language: string[]
  platform: string[]
  note: string
  galgame_id: number
  download: number
  like_count: number
  created: string
  user: KunUser
}

interface AdminLog {
  id: number
  type: string
  user_id: number
  user: KunUser | null
  content: string
  created: Date | string
}

// GET /api/v1/admin/patch/orphans — paginated with extra counts outside of `items`.
interface AdminOrphanPatchesResponse {
  items: GalgameCard[]
  total: number
  pending_count: number
  bad_vndb_count: number
}
