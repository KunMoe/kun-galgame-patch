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
  purgeUser: '清除用户',
  reorderResources: '调整资源展示顺序'
}

export const ADMIN_STATS_SUM_MAP: Record<string, string> = {
  user_count: '用户总数',
  galgame_count: 'Galgame 总数',
  resource_count: 'Galgame 补丁总数'
}

export const ADMIN_STATS_MAP: Record<string, string> = {
  new_user: '新注册用户',
  new_active_user: '新活跃用户',
  new_galgame: '新发布 Galgame',
  new_resource: '新发布补丁'
}

// User management (/admin/user) and creator-application approvals
// (/admin/creator) were removed when identity moved to OAuth and the creator
// role was retired. User bans / role grants now happen on the OAuth admin
// console.
export interface KunAdminMenuItem {
  name: string
  href: string
  icon: string
  permission: string
}

export const ADMIN_MENU: KunAdminMenuItem[] = [
  {
    name: '数据概览',
    href: '/admin',
    icon: 'lucide:chart-column-big',
    permission: 'admin.access'
  },
  {
    name: 'Galgame 列表',
    href: '/admin/galgame',
    icon: 'lucide:gamepad-2',
    permission: 'admin.access'
  },
  {
    name: '补丁资源管理',
    href: '/admin/resource',
    icon: 'lucide:puzzle',
    permission: 'admin.access'
  },
  {
    name: '孤儿补丁',
    href: '/admin/orphans',
    icon: 'lucide:unlink',
    permission: 'admin.access'
  },
  {
    name: '合并记录',
    href: '/admin/merges',
    icon: 'lucide:history',
    permission: 'admin.access'
  },
  {
    name: '评论管理',
    href: '/admin/comment',
    icon: 'lucide:message-square',
    permission: 'admin.access'
  },
  {
    name: '内容审核',
    href: '/admin/moderation',
    icon: 'lucide:shield-alert',
    permission: 'trust.review'
  },
  {
    name: '文档管理',
    href: '/admin/doc',
    icon: 'lucide:notebook-pen',
    permission: 'doc.manage'
  },
  {
    name: '用户清除',
    href: '/admin/user-purge',
    icon: 'lucide:user-x',
    permission: 'user.purge'
  },
  {
    name: '权限管理',
    href: '/admin/permission',
    icon: 'lucide:key-round',
    permission: 'permission.manage'
  },
  {
    name: '管理日志',
    href: '/admin/log',
    icon: 'lucide:file-clock',
    permission: 'admin.access'
  },
  {
    name: '网站设置',
    href: '/admin/setting',
    icon: 'lucide:settings',
    permission: 'admin.access'
  }
]

export const ADMIN_PANEL_PERMISSIONS = [
  ...new Set(ADMIN_MENU.map((item) => item.permission))
]
