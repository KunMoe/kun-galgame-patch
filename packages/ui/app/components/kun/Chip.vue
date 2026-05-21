<script setup lang="ts">
import { kunVariantClasses } from './ui/variants'
import type { KunUIVariant, KunUIColor, KunUISize } from './ui/type'

// Small rounded "tag" piece — labels, status pills, taxonomy markers.
// For dot / count overlays use KunBadge instead.
interface Props {
  className?: string
  color?: KunUIColor
  size?: KunUISize
  variant?: KunUIVariant
}

const props = withDefaults(defineProps<Props>(), {
  color: 'default',
  className: '',
  size: 'sm',
  variant: 'flat'
})

const sizeClasses: Record<KunUISize, string> = {
  xs: 'px-2 py-0.5 text-xs',
  sm: 'px-2 py-1 text-xs',
  md: 'px-3 py-1.5 text-sm',
  lg: 'px-4 py-2 text-sm',
  xl: 'px-6 py-3 text-base'
}
</script>

<template>
  <span
    :class="
      cn(
        'inline-flex cursor-default items-center justify-center gap-1 rounded-full font-medium',
        sizeClasses[props.size],
        kunVariantClasses(props.variant, props.color),
        className
      )
    "
  >
    <slot />
  </span>
</template>
