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
    <div
      class="pointer-events-none relative hidden select-none md:block"
    >
      <KunChip
        size="lg"
        class-name="absolute top-0 left-0"
        variant="flat"
        color="secondary"
      >
        <div class="flex items-center gap-2">
          <KunIcon name="lucide:lollipop" class="h-5 w-5" />
          欢迎来到 {{ kunMoyuMoe.titleShort }}
        </div>
      </KunChip>
      <!-- aspect-ratio reserves the box pre-load so the absolute chip above
           still anchors to the right position, and the sibling carousel
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
