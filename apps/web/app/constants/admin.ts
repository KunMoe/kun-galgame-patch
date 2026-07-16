export const APPLICANT_STATUS_MAP: Record<number, string> = {
  0: '待处理',
  1: '已处理',
  2: '已同意',
  3: '已拒绝'
}

export const APPLICANT_STATUS_COLOR_MAP: Record<
  number,
  'default' | 'primary' | 'success' | 'danger'
> = {
  0: 'primary',
  1: 'default',
  2: 'success',
  3: 'danger'
}

export const ADMIN_LOG_TYPE_MAP: Record<string, string> = {
  // Legacy types (historical rows written by the retired Next.js admin —
  // their content is full Chinese prose, so these still read fine).
  create: '创建',
  delete: '删除',
  approve: '同意',
  decline: '拒绝',
  update: '更改',
  // Current types (written by the Go admin's CreateLog — type + JSON content).
  deleteResource: '删除补丁资源',
  updateResource: '更改补丁资源',
  deleteComment: '删除评论',
  updateComment: '更改评论',
  purgeUser: '清除用户'
}

export const ADMIN_STATS_SUM_MAP: Record<string, string> = {
  user_count: '用户总数',
  galgame_count: 'Galgame 总数',
  resource_count: 'Galgame 补丁总数',
  comment_count: '评论总数'
}

export const ADMIN_STATS_MAP: Record<string, string> = {
  new_user: '新注册用户',
  new_active_user: '新活跃用户',
  new_galgame: '新发布 Galgame',
  new_resource: '新发布补丁',
  new_comment: '新发布评论'
}

// User management (/admin/user) and creator-application approvals
// (/admin/creator) were removed when identity moved to OAuth and the creator
// role was retired. User bans / role grants now happen on the OAuth admin
// console.
// `adminOnly` entries are visible only to OAuth "admin" (legacy super-admin,
// role 4); everything else is open to "moderator" + "admin" (role > 2). The
// flagged entries' backend endpoints are admin-gated (adminAuth), so showing
// them to a moderator would just yield 403s — hide them instead.
export interface KunAdminMenuItem {
  name: string
  href: string
  icon: string
  adminOnly?: boolean
}

export const ADMIN_MENU: KunAdminMenuItem[] = [
  { name: '数据概览', href: '/admin', icon: 'lucide:chart-column-big' },
  { name: 'Galgame 列表', href: '/admin/galgame', icon: 'lucide:gamepad-2' },
  { name: '补丁资源管理', href: '/admin/resource', icon: 'lucide:puzzle' },
  { name: '孤儿补丁', href: '/admin/orphans', icon: 'lucide:unlink' },
  { name: '评论管理', href: '/admin/comment', icon: 'lucide:message-square' },
  { name: '内容审核', href: '/admin/moderation', icon: 'lucide:shield-alert' },
  { name: '文档管理', href: '/admin/doc', icon: 'lucide:notebook-pen' },
  { name: '用户清除', href: '/admin/user-purge', icon: 'lucide:user-x', adminOnly: true },
  { name: '管理日志', href: '/admin/log', icon: 'lucide:file-clock' },
  // 网站设置 GET is moderator-readable (only its PUT toggles are admin-gated
  // server-side), so it stays visible to moderators — not adminOnly.
  { name: '网站设置', href: '/admin/setting', icon: 'lucide:settings' }
]
