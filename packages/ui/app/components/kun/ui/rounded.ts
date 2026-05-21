import { computed, type ComputedRef } from 'vue'
import { useKunUIConfig } from '../../../composables/useKunUIConfig'
import type { KunUIRounded } from './type'

// Static class map for the 5-bucket Kun radius system. All keys are
// literal strings so the Tailwind JIT picks them up (the
// `rounded-kun-*` utilities come from --radius-kun-* tokens in
// app/styles/tailwindcss.css's @theme block).
export const kunRoundedClasses: Record<KunUIRounded, string> = {
  none: 'rounded-kun-none',
  sm: 'rounded-kun-sm',
  md: 'rounded-kun-md',
  lg: 'rounded-kun-lg',
  full: 'rounded-kun-full',
}

// Resolves a component's effective rounded bucket using the precedence
// chain (prop > provider > built-in). Pass the prop accessor as a
// function so reactivity tracking works correctly inside computed.
//
// Usage in a component:
//
//   const props = defineProps<{ rounded?: KunUIRounded }>()
//   const rounded = useResolvedRounded(() => props.rounded)
//   // then bind  :class="kunRoundedClasses[rounded.value]"
//
// `fallback` lets a component override the provider default (e.g.,
// Modal wants 'lg' by built-in default; Input wants 'md'). Pass
// undefined to defer to provider/built-in.
export const useResolvedRounded = (
  propValue: () => KunUIRounded | undefined,
  fallback?: KunUIRounded
): ComputedRef<KunUIRounded> => {
  const config = useKunUIConfig()
  return computed<KunUIRounded>(
    () => propValue() ?? fallback ?? config.rounded
  )
}
