<script setup lang="ts">
interface Props {
  className?: string
}
const props = withDefaults(defineProps<Props>(), { className: '' })

const userStore = useUserStore()
const aifyUrl = useRuntimeConfig().public.umamiAifyUrl
</script>

<template>
  <div
    v-if="!userStore.isAifyFree && aifyUrl"
    :class="cn('overflow-hidden rounded-2xl shadow-xl', props.className)"
  >
    <a
      :href="aifyUrl"
      target="_blank"
      rel="noopener noreferrer"
      class="block h-full w-full"
    >
      <KunImage
        src="/a/moyumoe1.avif"
        alt=""
        provider="none"
        aspect-ratio="1920 / 300"
        class-name="w-full"
        image-class-name="pointer-events-none select-none"
      />
    </a>
  </div>
</template>
