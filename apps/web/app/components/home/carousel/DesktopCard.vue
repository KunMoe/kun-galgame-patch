<script setup lang="ts">
import { aboutDirectoryLabelMap } from '~/constants/about'

interface Props {
  posts: HomeCarouselMetadata[]
  currentSlide: number
}

const props = defineProps<Props>()

const post = computed(() => props.posts[props.currentSlide])
</script>

<template>
  <div v-if="post" class="group hidden h-full sm:block">
    <!-- `block` overrides KunImage's default inline-block wrapper so the
         carousel's h-full chain (parent h-[300px] / md:h-full → this h-full)
         can actually take effect. Without it the wrapper is inline-block
         0×0 pre-load and the whole carousel column collapses. -->
    <KunImage
      :src="post.banner"
      :alt="post.title"
      class-name="block h-full w-full rounded-2xl"
      image-class-name="brightness-75"
    />
    <div
      class="absolute inset-0 rounded-2xl bg-gradient-to-t from-black/30 via-black/10 to-transparent"
    />

    <HomeCarouselNavigationMenu />

    <div
      class="bg-background/80 absolute right-4 bottom-4 left-4 rounded-lg border-none p-4 backdrop-blur-md"
    >
      <div class="flex justify-between">
        <div>
          <div class="mb-2 flex items-center gap-3">
            <KunImage
              :src="post.authorAvatar"
              :alt="post.authorName"
              class-name="h-6 w-6 rounded-full"
            />
            <span class="text-foreground/80 text-sm">
              {{ post.authorName }}
            </span>
          </div>
          <KunLink
            color="default"
            underline="none"
            :to="post.link"
            class-name="hover:text-primary-500 mb-2 text-2xl font-bold line-clamp-1"
          >
            <h1>{{ post.title }}</h1>
          </KunLink>
        </div>
      </div>

      <p class="text-foreground/80 mb-2 text-sm line-clamp-1">
        {{ post.description }}
      </p>
      <div class="flex flex-wrap gap-2">
        <KunChip variant="flat" size="sm" color="primary">
          {{ aboutDirectoryLabelMap[post.directory] }}
        </KunChip>

        <KunChip variant="flat" size="sm">
          {{ formatDistanceToNow(post.date) }}
        </KunChip>
      </div>
    </div>
  </div>
</template>
