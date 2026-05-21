<script setup lang="ts">
import { useIntervalFn } from '@vueuse/core'

interface Props {
  posts: HomeCarouselMetadata[]
}

const props = defineProps<Props>()

const currentSlide = ref(0)
const isHovered = ref(false)

const paginate = (direction: number) => {
  const total = props.posts.length
  if (total === 0) return
  currentSlide.value = (currentSlide.value + direction + total) % total
}

const goTo = (index: number) => {
  currentSlide.value = index
}

const { pause, resume } = useIntervalFn(() => {
  if (!isHovered.value) {
    paginate(1)
  }
}, 5000)

watch(isHovered, (v) => (v ? pause() : resume()))
</script>

<template>
  <div
    class="group relative h-[300px] touch-pan-y overflow-hidden md:h-full"
    @mouseenter="isHovered = true"
    @mouseleave="isHovered = false"
  >
    <Transition
      enter-active-class="transition-all duration-300 ease-out"
      leave-active-class="transition-all duration-300 ease-in absolute inset-0"
      enter-from-class="opacity-0"
      leave-to-class="opacity-0"
      mode="out-in"
    >
      <div
        :key="currentSlide"
        class="absolute h-full w-full cursor-grab active:cursor-grabbing"
      >
        <HomeCarouselDesktopCard
          :posts="props.posts"
          :current-slide="currentSlide"
        />
        <HomeCarouselMobileCard
          :posts="props.posts"
          :current-slide="currentSlide"
        />
      </div>
    </Transition>

    <KunButton
      variant="flat"
      color="default"
      size="sm"
      is-icon-only
      rounded="full"
      class-name="bg-background/20 hover:bg-background/40 touch:opacity-100 absolute top-1/2 left-2 z-10 -translate-y-1/2 opacity-0 backdrop-blur-sm group-hover:opacity-100"
      aria-label="previous slide"
      @click="paginate(-1)"
    >
      <KunIcon name="lucide:chevron-left" class="size-4" />
    </KunButton>

    <KunButton
      variant="flat"
      color="default"
      size="sm"
      is-icon-only
      rounded="full"
      class-name="bg-background/20 hover:bg-background/40 touch:opacity-100 absolute top-1/2 right-2 z-10 -translate-y-1/2 opacity-0 backdrop-blur-sm group-hover:opacity-100"
      aria-label="next slide"
      @click="paginate(1)"
    >
      <KunIcon name="lucide:chevron-right" class="size-4" />
    </KunButton>

    <div
      class="absolute bottom-1 left-1/2 z-10 flex -translate-x-1/2 gap-1"
    >
      <button
        v-for="(_, index) in props.posts"
        :key="index"
        type="button"
        :aria-label="`go to slide ${index + 1}`"
        :class="
          cn(
            'h-1.5 w-1.5 rounded-full transition-all',
            index === currentSlide
              ? 'bg-primary w-4'
              : 'bg-foreground/20 hover:bg-foreground/40'
          )
        "
        @click="goTo(index)"
      />
    </div>
  </div>
</template>
