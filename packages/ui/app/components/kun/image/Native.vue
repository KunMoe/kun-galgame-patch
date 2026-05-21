<script setup lang="ts">
interface KunImageProps {
  src: string
  alt?: string
  ariaLabel?: string
  width?: string | number
  height?: string | number
  className?: string
}

withDefaults(defineProps<KunImageProps>(), {
  alt: 'image',
  className: undefined,
  ariaLabel: undefined,
  width: undefined,
  height: undefined
})
</script>

<template>
  <img
    :class="cn('', className)"
    :src="src"
    :alt="alt"
    :aria-label="ariaLabel"
    :width="width"
    :height="height"
  />
</template>
