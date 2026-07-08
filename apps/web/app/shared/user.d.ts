// Global KunUser brief used across moyu (avatars, user chips, and the
// message / chat / comment / patch / resource response shapes). The old in-repo
// @kun/ui layer declared the base { id, name, avatar } globally and moyu
// declaration-merged the extras; @kungal/ui-core now exports KunUser as a module
// type (NOT global), so moyu owns the full global shape here. @kungal components
// (KunAvatar / KunUserChip) take their own KunUser ({ id, name, avatar }) — this
// superset stays structurally assignable to it.
//
// `id` is the DB-truth chain shared name (user.id column → Go DTO `json:"id"`,
// the FK invariant across kungal / moyu / wiki). Components must guard for null —
// OAuth /users/batch can return a missing brief.
interface KunUser {
  id: number
  name: string
  avatar: string
  // image_service hash for the avatar. When present, resolveAvatarUrl prefers
  // this over `avatar` (which is the legacy absolute URL kept for back-compat
  // while image_service is rolling out).
  avatar_image_hash?: string
  // OAuth role set (e.g. ["admin"], ["moderator"]). Surfaced here so the UI
  // can render an admin / mod badge without a second round-trip.
  roles?: string[]
  // moyu site-scoped roles (never admin/ren) — pair with `roles` in
  // pickRoleBadge to render a "本站版主" badge. See docs/oauth/12-site-roles.md.
  site_roles?: string[]
}
