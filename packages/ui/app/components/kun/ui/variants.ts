import type { KunUIVariant, KunUIColor } from './type'

// Single source of truth for the variant × color → Tailwind class table.
// Button / Badge / Chip / Tab / Info / Progress etc. consume this so the
// 7 × 7 table only lives in one place. Adding a new color or variant is
// a single edit here instead of 4+ identical edits across components.
//
// All keys must be static string literals so the Tailwind JIT picks them
// up — never construct class names with template strings at runtime.
const TABLE: Record<KunUIVariant, Record<KunUIColor, string>> = {
  solid: {
    default: 'bg-default text-white',
    primary: 'bg-primary text-white',
    secondary: 'bg-secondary text-white',
    success: 'bg-success-600 text-white dark:bg-success-300',
    warning: 'bg-warning text-white',
    danger: 'bg-danger text-white',
    info: 'bg-info-600 text-white',
  },
  bordered: {
    default: 'bg-transparent border-default',
    primary: 'bg-transparent border-primary text-primary',
    secondary: 'bg-transparent border-secondary text-secondary',
    success: 'bg-transparent border-success text-success',
    warning: 'bg-transparent border-warning text-warning',
    danger: 'bg-transparent border-danger text-danger',
    info: 'bg-transparent border-info text-info',
  },
  light: {
    default: 'bg-transparent hover:bg-default/20',
    primary: 'bg-transparent text-primary hover:bg-primary/20',
    secondary: 'bg-transparent text-secondary hover:bg-secondary/20',
    success: 'bg-transparent text-success hover:bg-success/20',
    warning: 'bg-transparent text-warning hover:bg-warning/20',
    danger: 'bg-transparent text-danger hover:bg-danger/20',
    info: 'bg-transparent text-info hover:bg-info/20',
  },
  flat: {
    default: 'bg-default/20 text-default-700 border-transparent',
    primary: 'bg-primary/20 text-primary-600 border-transparent',
    secondary: 'bg-secondary/20 text-secondary-600 border-transparent',
    success: 'bg-success/20 text-success-700 border-transparent dark:text-success',
    warning: 'bg-warning/20 text-warning-700 border-transparent dark:text-warning',
    danger: 'bg-danger/20 text-danger-600 border-transparent dark:text-danger-500',
    info: 'bg-info/20 text-info-700 border-transparent dark:text-info-500',
  },
  faded: {
    default: 'border-default bg-default-100',
    primary: 'border-default bg-primary-100 text-primary',
    secondary: 'border-default bg-secondary-100 text-secondary',
    success: 'border-default bg-success-100 text-success',
    warning: 'border-default bg-warning-100 text-warning',
    danger: 'border-default bg-danger-100 text-danger',
    info: 'border-default bg-info-100 text-info',
  },
  shadow: {
    default: 'shadow-default/40 bg-default text-white',
    primary: 'shadow-primary/40 bg-primary text-white',
    secondary: 'shadow-secondary/40 bg-secondary text-white',
    success: 'shadow-success/40 bg-success-600 text-white',
    warning: 'shadow-warning/40 bg-warning text-white',
    danger: 'shadow-danger/40 bg-danger text-white',
    info: 'shadow-info/40 bg-info-600 text-white',
  },
  ghost: {
    default: 'bg-transparent border-default hover:bg-default/10',
    primary: 'bg-transparent border-primary text-primary hover:bg-primary/10',
    secondary: 'bg-transparent border-secondary text-secondary hover:bg-secondary/10',
    success: 'bg-transparent border-success text-success hover:bg-success/10',
    warning: 'bg-transparent border-warning text-warning hover:bg-warning/10',
    danger: 'bg-transparent border-danger text-danger hover:bg-danger/10',
    info: 'bg-transparent border-info text-info hover:bg-info/10',
  },
}

export const kunVariantClasses = (
  variant: KunUIVariant,
  color: KunUIColor
): string => TABLE[variant][color]

// Static maps for narrower use cases (e.g. just the fill color, just the
// text color) — same JIT safety requirement: keys must be literals.

export const kunBgClasses: Record<KunUIColor, string> = {
  default: 'bg-default',
  primary: 'bg-primary',
  secondary: 'bg-secondary',
  success: 'bg-success',
  warning: 'bg-warning',
  danger: 'bg-danger',
  info: 'bg-info',
}

export const kunTextClasses: Record<KunUIColor, string> = {
  default: 'text-foreground',
  primary: 'text-primary',
  secondary: 'text-secondary',
  success: 'text-success',
  warning: 'text-warning',
  danger: 'text-danger',
  info: 'text-info',
}

export const kunBorderClasses: Record<KunUIColor, string> = {
  default: 'border-default',
  primary: 'border-primary',
  secondary: 'border-secondary',
  success: 'border-success',
  warning: 'border-warning',
  danger: 'border-danger',
  info: 'border-info',
}

export const kunRingClasses: Record<KunUIColor, string> = {
  default: 'focus-within:ring-default/40 focus:ring-default/40',
  primary: 'focus-within:ring-primary/40 focus:ring-primary/40',
  secondary: 'focus-within:ring-secondary/40 focus:ring-secondary/40',
  success: 'focus-within:ring-success/40 focus:ring-success/40',
  warning: 'focus-within:ring-warning/40 focus:ring-warning/40',
  danger: 'focus-within:ring-danger/40 focus:ring-danger/40',
  info: 'focus-within:ring-info/40 focus:ring-info/40',
}
