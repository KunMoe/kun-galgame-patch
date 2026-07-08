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

export interface RoleBadge {
  label: string
  // true → the winning role is moyu site-scoped (rendered with a 本站 prefix and
  // a distinct chip color); false → a global role (or the 普通用户 fallback).
  site: boolean
}

// pickRoleBadge picks the single most-significant role badge across a user's
// global `roles` and moyu site-scoped `site_roles` (docs/oauth/12-site-roles.md).
// Both sets are ranked together by ROLE_PRIORITY; the winner is prefixed 本站
// when it is site-scoped (a moyu-only 版主 → "本站版主"). The implicit `user` and
// unknown/custom site names never win; the fallback is 普通用户.
export const pickRoleBadge = (
  roles?: string[] | null,
  siteRoles?: string[] | null
): RoleBadge => {
  const candidates = [
    ...(roles ?? []).map((role) => ({ role, site: false })),
    ...(siteRoles ?? []).map((role) => ({ role, site: true }))
  ]
  let best: { role: string; site: boolean } | null = null
  let bestRank = ROLE_PRIORITY.length
  for (const cand of candidates) {
    if (cand.role === 'user') continue
    const rank = ROLE_PRIORITY.indexOf(cand.role)
    if (rank === -1) continue
    // Lower index = higher priority; on a tie prefer the global one (no 本站).
    if (rank < bestRank || (rank === bestRank && best?.site && !cand.site)) {
      best = cand
      bestRank = rank
    }
  }
  if (!best) return { label: USER_ROLE_MAP.user ?? '普通用户', site: false }
  const label = USER_ROLE_MAP[best.role] ?? best.role
  return { label: best.site ? `本站${label}` : label, site: best.site }
}

// pickRoleLabel returns just the badge text for a global role set (back-compat).
export const pickRoleLabel = (roles: string[] | null | undefined): string =>
  pickRoleBadge(roles).label

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
