<script setup lang="ts">
import { kunMoyuMoe } from '~/config/moyu-moe'

const { data: posts } = await useAsyncData<HomeCarouselMetadata[]>(
  'home-carousel',
  () => $fetch('/api/home/carousel'),
  { default: () => [] }
)
</script>

<template>
  <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
    <div class="pointer-events-none hidden select-none md:block">
      <!-- aspect-ratio reserves the box pre-load so the sibling carousel
           column (grid stretch + md:h-full) doesn't collapse on slow loads.
           Asset is 1920×1080. -->
      <KunImage
        src="/kungalgame-trans.webp"
        :alt="kunMoyuMoe.titleShort"
        aspect-ratio="16 / 9"
        class-name="rounded-2xl"
      />
    </div>

    <HomeCarousel :posts="posts ?? []" />
  </div>
</template>
