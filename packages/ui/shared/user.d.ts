// KunUser is the minimal brief required to render an avatar / user
// chip / dropdown. `uid` matches the rest of the stack (OAuth JWT
// `uid` claim, Prisma user PK, nitro-server route `/user/[uid]/...`).
//
// Components that consume KunUser (KunAvatar, KunUser display chip)
// must guard for null — upstream user hydration (OAuth /users/batch)
// can return a missing brief; see KunAvatarProps for the nullable
// type on the prop boundary.
interface KunUser {
  uid: number
  name: string
  avatar: string
}
