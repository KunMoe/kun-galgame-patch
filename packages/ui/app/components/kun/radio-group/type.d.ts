import type { KunUIColor, KunUIRounded, KunUISize } from '../ui/type'

export type KunRadioValue = string | number

export type KunRadioVariant = 'classic' | 'card'

export type KunRadioOrientation = 'vertical' | 'horizontal'

export interface KunRadioOption<T extends KunRadioValue = KunRadioValue> {
  value: T
  label: string
  // Optional sub-text shown beneath the label. Mostly useful with the
  // `card` variant; classic variant renders it as muted text inline.
  description?: string
  disabled?: boolean
}

export interface KunRadioGroupProps<
  T extends KunRadioValue = KunRadioValue,
> {
  options: readonly KunRadioOption<T>[]
  // Accessibility label for the radiogroup container. Used when there
  // is no visible `label`. Strongly recommended either way for screen
  // readers.
  ariaLabel?: string
  // Visible label rendered above the group. When set, the radiogroup
  // is wired up via aria-labelledby automatically.
  label?: string
  variant?: KunRadioVariant
  orientation?: KunRadioOrientation
  color?: KunUIColor
  size?: KunUISize
  // Only applies to `card` variant. `classic` variant always uses a
  // perfect circle for the indicator and ignores this prop.
  rounded?: KunUIRounded
  // Disables the whole group. Per-option `disabled` still works for
  // partial disables.
  disabled?: boolean
  error?: string
  className?: string
}
