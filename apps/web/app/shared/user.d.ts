// Augment @kun/ui's base KunUser (packages/ui/shared/user.d.ts: { id, name,
// avatar }) with moyu-specific optional fields. `id` is the DB-truth chain
// shared name (Prisma user.id → Go DTO `json:"id"`), so we don't redeclare
// the primary key here — let declaration merging add only what's new.
interface KunUser {
  // image_service hash for the avatar. When present, resolveAvatarUrl prefers
  // this over `avatar` (which is the legacy absolute URL kept for back-compat
  // while image_service is rolling out).
  avatar_image_hash?: string
  // OAuth role set (e.g. ["admin"], ["moderator"]). Surfaced here so the UI
  // can render an admin / mod badge without a second round-trip.
  roles?: string[]
}
