import type { KunUIColor } from '../ui/type'

export interface KunContextMenuItem {
  key: string
  label: string
  icon?: string
  color?: KunUIColor
  disabled?: boolean
}
