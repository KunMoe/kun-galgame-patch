// KunUser is the minimal brief required to render an avatar / user
// chip / dropdown.
//
// `id` matches the DB-truth chain: Prisma user.id column → Go DTO
// `json:"id"` (apps/api/.../oauth_dto.go documents this as the FK
// invariant across kungal/moyu/wiki) → nitro-server response types.
// JWT claim `id` and URL param `[id]` are auth/transport labels
// for the same integer — those names live in their own layer and
// don't propagate into UI props.
//
// Components that consume KunUser (KunAvatar, KunUser display chip)
// must guard for null — upstream user hydration (OAuth /users/batch)
// can return a missing brief; see KunAvatarProps for the nullable
// type on the prop boundary.
interface KunUser {
  id: number
  name: string
  avatar: string
}
