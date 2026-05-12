import type { KunUISize } from '~/components/kun/ui/type'

export interface KunAvatarProps {
  // Nullable: some response paths in the OAuth-migration transitional state
  // can pass a missing user (deleted sender, not-yet-enriched comment.user, ...).
  // The component handles undefined gracefully — see Avatar.vue safeUser.
  user: KunUser | null | undefined
  size?: KunUISize | 'original' | 'original-sm'
  isNavigation?: boolean
  className?: string
  imageClassName?: string
  disableFloating?: boolean
  floatingPosition?: 'top' | 'bottom' | 'left' | 'right'
}
