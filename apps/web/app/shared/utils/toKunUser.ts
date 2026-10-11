import type { KunAvatarDecoration } from '@kungal/ui-core'

interface AvatarOwner {
  avatar?: string | null
  avatar_image_hash?: string | null
  cosmetics?: UserCosmetics | null
}

// KunAvatar draws `avatar` verbatim, and an OAuth upload writes only
// avatar_image_hash. Passing the raw field showed 131 users the picture they had
// replaced and 5,394 a stock sticker, while every other site drew the new one.
export const toKunUser = <T extends AvatarOwner>(
  user: T | null | undefined
):
  | (T & { avatar: string; avatarDecoration?: KunAvatarDecoration })
  | null
  | undefined => {
  if (!user) return user
  const frame = user.cosmetics?.avatar_frame
  return {
    ...user,
    avatar: resolveAvatarUrl(user),
    ...(frame && {
      avatarDecoration: {
        src: frame.static_url,
        animatedSrc: frame.animated_url
      }
    })
  }
}
