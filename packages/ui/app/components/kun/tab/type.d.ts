import type { KunUIColor } from '../ui/type'

export type KunTabItem = {
  value: string
  textValue?: string
  icon?: string
  disabled?: boolean
  href?: string
}

//   underlined — bottom 2px sliding indicator (default)
//   solid      — selected tab gets a filled chip
//   bordered   — outer frame + selected tab outline
//   light      — selected tab gets a soft tinted background
//   pills      — each tab is an independent pill, selected fills color
export type KunTabVariant = 'underlined' | 'solid' | 'bordered' | 'light' | 'pills'

export type KunTabColor = KunUIColor
export type KunTabSize = 'sm' | 'md' | 'lg'
export type KunTabOrientation = 'horizontal' | 'vertical'
