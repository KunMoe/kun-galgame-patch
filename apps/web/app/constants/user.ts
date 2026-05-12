// Maps OAuth role strings (returned by /oauth/userinfo and /users/batch) to
// the Chinese labels rendered in role badges. Mirrors
// docs/user-migration/02-data-mapping.md §7:
//
//   "admin"     -> moyu 老 super-admin / OAuth global admin
//   "moderator" -> moyu / kungal 老 admin
//   "user"      -> 普通用户 (默认)
//
// Use `pickRoleLabel(roles)` to render the single badge for a user that may
// hold multiple OAuth roles (e.g. ["admin", "user"]).
export const USER_ROLE_MAP: Record<string, string> = {
  admin: '管理员',
  moderator: '版主',
  user: '用户'
}

// Highest-priority first; the first role in this list that the user holds
// wins, so an "admin + user" carrier shows up as 管理员.
const ROLE_PRIORITY: readonly string[] = ['admin', 'moderator', 'user']

export const pickRoleLabel = (roles: string[] | null | undefined): string => {
  if (!roles || roles.length === 0) return USER_ROLE_MAP.user
  for (const role of ROLE_PRIORITY) {
    if (roles.includes(role)) return USER_ROLE_MAP[role] ?? role
  }
  // Unknown role string -> render verbatim rather than mask the data.
  return roles[0] ?? USER_ROLE_MAP.user
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
