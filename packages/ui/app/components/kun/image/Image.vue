<script setup lang="ts">
interface KunImageProps {
  src: string
  alt?: string
  loading?: 'lazy' | 'eager'
  placeholder?:
    | string
    | number
    | boolean
    | [w: number, h: number, q?: number | undefined, b?: number | undefined]
    | undefined
  className?: string
  ariaLabel?: string
  format?: string
  quality?: string | number
  width?: string | number
  height?: string | number
  preload?:
    | boolean
    | {
        fetchPriority: 'auto' | 'high' | 'low'
      }
}
withDefaults(defineProps<KunImageProps>(), {
  alt: 'image',
  loading: undefined,
  placeholder: undefined,
  className: undefined,
  ariaLabel: undefined,
  format: undefined,
  quality: undefined,
  width: undefined,
  height: undefined,
  preload: undefined
})
</script>

<template>
  <NuxtImg
    :class="cn('', className)"
    :src="src"
    :alt="alt"
    :loading="loading"
    :placeholder="placeholder"
    :aria-label="ariaLabel"
    :format="format"
    :quality="quality"
    :width="width"
    :height="height"
    :preload="preload"
  />
</template>
