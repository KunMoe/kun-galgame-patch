interface KunUser {
  id: number
  name: string
  avatar: string
  // image_service hash for the avatar. When present, resolveAvatarUrl prefers
  // this over `avatar` (which is the legacy absolute URL kept for back-compat
  // while image_service is rolling out). May be empty for users who haven't
  // re-uploaded their avatar post-migration.
  avatar_image_hash?: string
  // OAuth role set (e.g. ["admin"], ["moderator"]). Surfaced here so the UI
  // can render an admin / mod badge next to a username on cards or comments
  // without a second round-trip. Empty / undefined for regular users.
  roles?: string[]
}
