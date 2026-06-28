// Maps OAuth role strings (from the access-token `roles` claim / /users/batch)
// to the Chinese badge labels. Names follow the AUTHORITATIVE 5-role contract
// docs/oauth/11-roles.md §1 (Tier A) verbatim:
//
//   user      -> 普通用户   (implicit default; never appears in the claim)
//   creator   -> 创作者     (trusted publisher; orthogonal to moderation)
//   moderator -> 版主       (content moderation)
//   admin     -> 管理员     (site & user management)
//   ren (莲)  -> 莲         (super-admin above admin; DB-preset only)
//
// (Previously moyu used a "one tier up" product naming — moderator=管理员,
// admin=超级管理员 — now aligned to the contract's canonical names.)
//
// The `roles` claim is a SET with NO "user" string (普通用户 = empty array), so
// 普通用户 is the empty/fallback case. Use `pickRoleLabel(roles)` to render the
// single badge for a user that may hold several roles.
export const USER_ROLE_MAP: Record<string, string> = {
  ren: '莲',
  admin: '管理员',
  moderator: '版主',
  creator: '创作者',
  user: '普通用户'
}

// Highest-priority first; the first role the user holds wins, mirroring the
// contract's management axis (ren > admin > moderator) with `creator` slotted
// as the orthogonal trusted-publisher tier between moderator and user.
const ROLE_PRIORITY: readonly string[] = [
  'ren',
  'admin',
  'moderator',
  'creator',
  'user'
]

export const pickRoleLabel = (roles: string[] | null | undefined): string => {
  if (!roles || roles.length === 0) return USER_ROLE_MAP.user ?? '普通用户'
  for (const role of ROLE_PRIORITY) {
    if (roles.includes(role)) return USER_ROLE_MAP[role] ?? role
  }
  // Unknown role string -> render verbatim rather than mask the data.
  return roles[0] ?? USER_ROLE_MAP.user ?? '普通用户'
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
