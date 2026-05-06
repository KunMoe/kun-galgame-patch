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
  create: '创建',
  delete: '删除',
  approve: '同意',
  decline: '拒绝',
  update: '更改'
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

export const ADMIN_MENU = [
  { name: '数据概览', href: '/admin', icon: 'lucide:chart-column-big' },
  { name: '用户管理', href: '/admin/user', icon: 'lucide:users' },
  { name: '创作者管理', href: '/admin/creator', icon: 'lucide:badge-check' },
  { name: 'Galgame 列表', href: '/admin/galgame', icon: 'lucide:gamepad-2' },
  { name: '补丁资源管理', href: '/admin/resource', icon: 'lucide:puzzle' },
  { name: '孤儿补丁', href: '/admin/orphans', icon: 'lucide:unlink' },
  { name: '评论管理', href: '/admin/comment', icon: 'lucide:message-square' },
  { name: '管理日志', href: '/admin/log', icon: 'lucide:file-clock' },
  { name: '网站设置', href: '/admin/setting', icon: 'lucide:settings' }
]
