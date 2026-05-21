import type { KunUIColor, KunUISize } from '../ui/type'

export interface KunLinkProps {
  // NuxtLink auto-falls-through to `<a>` for external URLs, so no
  // separate `tag` / `as` prop is needed here.
  href?: string
  to?: string | Record<string, string>
  color?: KunUIColor
  underline?: 'none' | 'hover' | 'always'
  size?: KunUISize
  className?: string
  rel?: string
  target?: '_blank' | '_self' | '_parent' | '_top'
  isShowAnchorIcon?: boolean
}
