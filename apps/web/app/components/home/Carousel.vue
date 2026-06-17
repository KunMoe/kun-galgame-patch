<script setup lang="ts">
interface Props {
  posts: HomeCarouselMetadata[]
}

const props = defineProps<Props>()
</script>

<template>
  <!-- Migrated from a hand-rolled carousel to KunCarousel (kun-ui 1.6): it
       provides scroll-snap swipe, arrows, dot indicators and autoplay for free.
       We keep the per-post desktop/mobile cards as the slide content. Each slide
       owns its height (h-[300px] on mobile, 16/9 from md to match Hero's sibling
       brand image). KunCarousel renders every slide in the DOM (a horizontal
       track), so unlike the old one-at-a-time render we mark only the first
       banner eager to keep the home LCP unchanged. -->
  <KunCarousel
    :autoplay="5000"
    :slides-per-view="1"
    aria-label="置顶公告轮播"
  >
    <KunCarouselItem
      v-for="(post, index) in props.posts"
      :key="post.link ?? index"
      class-name="relative h-[300px] overflow-hidden rounded-2xl md:h-auto md:aspect-video"
    >
      <HomeCarouselDesktopCard :post="post" :eager="index === 0" />
      <HomeCarouselMobileCard :post="post" :eager="index === 0" />
    </KunCarouselItem>
  </KunCarousel>
</template>
