export interface KunNavItem {
  name: string
  href: string
}

// NOTE: /tag, /company, /character, /person, /release are deprecated per D8/D11/D12.
// Their metadata is owned by the Galgame Wiki Service (galgame.kungal.com).
export const kunNavItem: KunNavItem[] = [
  { name: '下载', href: '/galgame' },
  { name: '发布', href: '/edit/create' },
  { name: '排行', href: '/ranking/user' },
  { name: '关于', href: '/about' }
]

export const kunNavItemDesktop: KunNavItem[] = [
  { name: '发布补丁', href: '/edit/create' },
  { name: '关于我们', href: '/about' }
]

// Public mobile nav entries (visible to everyone). The admin entry is
// rendered separately, gated on userStore.isAdmin in MobileMenu.vue, so
// non-admins don't see (and can't 403 on) it.
export const kunMobileNavItem: KunNavItem[] = [
  ...kunNavItem,
  { name: '补丁评论列表', href: '/comment' },
  { name: '补丁资源列表', href: '/resource' },
  { name: '联系我们', href: '/about/notice/feedback' }
]

// Admin-only entries. Filter into the rendered list at the call site.
export const kunMobileAdminItem: KunNavItem[] = [
  { name: '管理系统', href: '/admin' }
]

export const KUN_CONTENT_LIMIT_MAP: Record<string, string> = {
  sfw: '仅显示 SFW (内容安全) 的内容',
  nsfw: '仅显示 NSFW (可能含有 R18) 的内容',
  all: '同时显示 SFW 和 NSFW 的内容'
}

export const KUN_CONTENT_LIMIT_LABEL: Record<string, string> = {
  '': '全年龄',
  sfw: '全年龄',
  nsfw: '涩涩模式',
  all: 'R18模式'
}

export interface KunTopBarCategoryItem {
  href: string
  label: string
  icon: string
}

export const kunTopBarCategories: KunTopBarCategoryItem[] = [
  { href: '/galgame', label: 'Galgame 列表', icon: 'lucide:gamepad-2' },
  { href: '/resource', label: '最新补丁列表', icon: 'lucide:puzzle' },
  { href: '/ranking', label: 'Galgame 排行', icon: 'lucide:chart-column-big' }
]
