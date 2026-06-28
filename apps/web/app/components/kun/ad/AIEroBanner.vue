<script setup lang="ts">
import { kunMoyuMoe } from '~/config/moyu-moe'

// Wide ad banner (ported from the legacy next-web
// components/kun/ad/AIEroBanner.tsx). Shown to everyone EXCEPT ad-free roles
// (see AIEroNav for the role-gate rationale). The image is 1920x300; the
// aspect-ratio box reserves space so it doesn't shift layout while the bytes
// load. `className` lets call sites add responsive visibility (the resource
// detail page renders a desktop + a mobile instance).
interface Props {
  className?: string
}
const props = withDefaults(defineProps<Props>(), { className: '' })

const userStore = useUserStore()
</script>

<template>
  <div
    v-if="!userStore.isAdFree"
    :class="cn('overflow-hidden rounded-2xl shadow-xl', props.className)"
  >
    <a
      :href="kunMoyuMoe.ad[0]?.url"
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
