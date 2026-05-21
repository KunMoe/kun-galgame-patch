import type { KunUIRounded } from '../ui/type'

// Generic value type lets `as const` literals narrow to the union of
// their values instead of widening to `string | number`. Constrained to
// the primitive value types Select actually supports.
export type KunSelectValue = string | number

export interface KunSelectOption<T extends KunSelectValue = KunSelectValue> {
  value: T
  label: string
}

// `options` is `readonly` so callers can pass `as const`-frozen arrays
// without TS variance errors. modelValue is intentionally not on this
// interface — Select.vue uses defineModel<T>() to expose the v-model
// binding.
export interface KunSelectProps<T extends KunSelectValue = KunSelectValue> {
  options: readonly KunSelectOption<T>[]
  label?: string
  placeholder?: string
  error?: string
  disabled?: boolean
  darkBorder?: boolean
  ariaLabel?: string
  className?: string
  rounded?: KunUIRounded
}
