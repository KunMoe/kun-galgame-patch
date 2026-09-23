import type { KunAvatarDecoration } from '@kungal/ui-core'

export const toKunUser = <T extends { cosmetics?: UserCosmetics | null }>(
  user: T | null | undefined
): (T & { avatarDecoration?: KunAvatarDecoration }) | null | undefined => {
  const frame = user?.cosmetics?.avatar_frame
  if (!user || !frame) return user
  return {
    ...user,
    avatarDecoration: { src: frame.static_url, animatedSrc: frame.animated_url }
  }
}
