// Maps OAuth role strings (returned by /oauth/userinfo and /users/batch) to
// the Chinese labels rendered in role badges. Mirrors
// docs/user-migration/02-data-mapping.md §7:
//
//   "admin"     -> 超级管理员 (moyu 老 super-admin / OAuth global admin)
//   "moderator" -> 管理员     (moyu / kungal 老 admin)
//   "user"      -> 用户       (普通用户, 默认)
//
// The display labels are deliberately one tier "up" from the OAuth role name
// (moderator→管理员, admin→超级管理员): the OAuth migration kept the legacy role
// *strings*, but the product names the tiers 用户 / 管理员 / 超级管理员.
//
// Use `pickRoleLabel(roles)` to render the single badge for a user that may
// hold multiple OAuth roles (e.g. ["admin", "user"]).
export const USER_ROLE_MAP: Record<string, string> = {
  // `ren` (莲) is the DB-preset super-admin; it ranks with admin and shares the
  // 超级管理员 badge (see userStore.isAdmin / middleware.SuperAdminRoles).
  ren: '超级管理员',
  admin: '超级管理员',
  moderator: '管理员',
  creator: '创作者',
  user: '用户'
}

// Highest-priority first; the first role in this list that the user holds
// wins, so an "admin + user" carrier shows up as 超级管理员. `creator` sits
// between moderator and user — a trusted-publisher tier, not a moderation one.
const ROLE_PRIORITY: readonly string[] = [
  'ren',
  'admin',
  'moderator',
  'creator',
  'user'
]

export const pickRoleLabel = (roles: string[] | null | undefined): string => {
  if (!roles || roles.length === 0) return USER_ROLE_MAP.user ?? '用户'
  for (const role of ROLE_PRIORITY) {
    if (roles.includes(role)) return USER_ROLE_MAP[role] ?? role
  }
  // Unknown role string -> render verbatim rather than mask the data.
  return roles[0] ?? USER_ROLE_MAP.user ?? '用户'
}

export const USER_STATUS_MAP: Record<number, string> = {
  0: '正常',
  1: '限制（正在开发中）',
  2: '封禁'
}

export const USER_STATUS_COLOR_MAP: Record<
  number,
  'success' | 'warning' | 'danger'
> = {
  0: 'success',
  1: 'warning',
  2: 'danger'
}
