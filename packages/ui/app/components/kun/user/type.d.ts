import type { KunAvatarSize } from '../avatar/type'

export interface KunUserProps {
  // Nullable: post user-migration, name/avatar live on OAuth and are
  // resolved via /users/batch. An unresolved id yields no brief — degrade
  // gracefully, never crash the page. Matches KunAvatarProps.user.
  user: KunUser | null | undefined
  size?: KunAvatarSize
  description?: string
  className?: string
  disableFloating?: boolean
  floatingPosition?: 'top' | 'bottom' | 'left' | 'right'
}
