import type { KunUISize } from '../ui/type'

export interface KunAvatarProps {
  // Nullable because upstream user hydration (OAuth /users/batch) can
  // return a missing brief — Avatar.vue handles the null branch with
  // a placeholder sticker rather than crashing.
  user: KunUser | null | undefined
  size?: KunUISize | 'original' | 'original-sm'
  isNavigation?: boolean
  className?: string
  imageClassName?: string
  // `disableFloating` / `floatingPosition` are accepted but unused.
  // Kept on the interface so existing callers don't TS-error.
  disableFloating?: boolean
  floatingPosition?: 'top' | 'bottom' | 'left' | 'right'
}
