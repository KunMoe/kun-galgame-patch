export interface KunSelectOption {
  value: string | number
  label: string
}

// modelValue is intentionally not on this interface — Select.vue uses
// defineModel<string | number>() to expose the v-model binding.
export interface KunSelectProps {
  options: KunSelectOption[]
  label?: string
  placeholder?: string
  error?: string
  disabled?: boolean
  darkBorder?: boolean
  ariaLabel?: string
  className?: string
}
