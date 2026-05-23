import type { KunUIRounded } from '../ui/type'

export type KunDrawerPlacement = 'left' | 'right' | 'top' | 'bottom'

// Size buckets — interpreted as width when placement is left/right and
// as height when placement is top/bottom. `full` fills the long axis,
// useful for mobile breakpoints or full-screen detail panels.
export type KunDrawerSize = 'sm' | 'md' | 'lg' | 'xl' | 'full'

export interface KunDrawerProps {
  // Which viewport edge the panel slides in from.
  //
  // Pairs with `responsive`: when `responsive: true` (default) AND the
  // viewport is below the `md` Tailwind breakpoint (< 768px), the
  // effective placement is forced to `'bottom'` regardless of this
  // prop — that's the "phones get a bottom sheet, desktop gets a side
  // drawer" pattern that almost every consumer wants by default.
  //
  // Set `responsive: false` to keep this placement locked across all
  // viewport sizes.
  placement?: KunDrawerPlacement
  // Auto-switch to bottom placement on mobile (viewport < md = 768px).
  // Default true — this is the "right on desktop, bottom on mobile"
  // out-of-the-box behavior.
  responsive?: boolean
  // Cross-axis size of the panel. When `responsive` is active and
  // placement is forced to 'bottom' on mobile, this prop still
  // controls the height bucket (sm/md/lg/xl) of the bottom sheet.
  size?: KunDrawerSize
  // Optional title rendered in the auto-built header. If absent, use
  // the `header` slot for fully custom header content. Both can be
  // used together — title renders inside header alongside slot content.
  title?: string
  // Click on backdrop closes the drawer + Escape closes it.
  isDismissable?: boolean
  // Show the X close button in the corner of the header.
  isShowCloseButton?: boolean
  // Wrap the default slot in a padded scrollable container. Set false
  // for fully custom layouts (e.g. media-heavy detail panels that own
  // their own padding / scroll).
  withContainer?: boolean
  // Corner rounding for the drawer panel. Only the corners on the
  // INNER side of the placement edge are rounded; the edge anchored
  // to the viewport stays square. Defaults to 'lg' (matches Modal).
  rounded?: KunUIRounded
  className?: string
  innerClassName?: string
}
