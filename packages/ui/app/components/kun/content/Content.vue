<script setup lang="ts">
import { kunSanitize } from '../../../utils/sanitize'
import { useSpoilerContent } from '../../../composables/topic/useSpoilerContent'
import { useContentLightbox } from '../../../composables/topic/useContentLightbox'

withDefaults(
  defineProps<{
    content: string
    className?: string
  }>(),
  {
    className: ''
  }
)

const articleRef = ref<HTMLElement | null>(null)

useSpoilerContent(articleRef)

// Inline-image lightbox. Delegated click on the prose container → opens
// KunLightbox at the clicked image, cycling through this block's images.
// Works for images added after mount (pagination / async render) because
// the img set is scanned live on each click. Every <KunContent> gets this
// for free — no per-consumer wiring.
const { isLightboxOpen, images, currentImageIndex } =
  useContentLightbox(articleRef)

const sanitizeConfig = {
  ADD_TAGS: ['div', 'span', 'button'],
  ADD_ATTR: ['class', 'title', 'line']
}
</script>

<template>
  <div>
    <article
      ref="articleRef"
      :class="cn('kun-prose', className)"
      v-html="kunSanitize(content, sanitizeConfig)"
    />
    <KunLightbox
      v-model:is-open="isLightboxOpen"
      :images="images"
      :initial-index="currentImageIndex"
    />
  </div>
</template>

<style scoped>
.kun-prose {
  /* Inline images are clickable (open the lightbox) — hint it. Excludes
   * images inside a spoiler (those are gated by the spoiler reveal first). */
  & :deep(img) {
    cursor: zoom-in;
  }

  & :deep(.kun-spoiler) {
    position: relative;
    display: inline-block;
    border-radius: 0.5rem;
    overflow: hidden;
    vertical-align: middle;

    & .spoiler-frost {
      position: absolute;
      inset: 0;
      pointer-events: none;
      background: rgb(150, 150, 150);
      border-radius: inherit;
      transform: translateZ(0);
      z-index: 5;
    }

    & > *:not(.spoiler-canvas) {
      transition: filter 0.3s ease-in-out;
      filter: blur(0);
    }

    & .spoiler-canvas {
      position: absolute;
      inset: 0;
      z-index: 1;
      pointer-events: none;
      opacity: 1;
      transition: opacity 0.3s ease-in-out;
      background-color: rgba(150, 150, 150, 0.1);

      &.fade-out {
        opacity: 0;
      }
    }
  }

  & :deep(.kun-spoiler.kun-spoiler-hidden) {
    cursor: pointer;

    & > *:not(.spoiler-canvas) {
      filter: blur(52px);
      user-select: none;
    }
  }

  & :deep(div.kun-spoiler) {
    display: block;
    width: fit-content;
  }
}
</style>
