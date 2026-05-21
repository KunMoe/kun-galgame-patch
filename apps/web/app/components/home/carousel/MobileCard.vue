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
  <div
    v-if="post"
    class="h-full border-none bg-transparent shadow-none sm:hidden"
  >
    <div class="relative h-1/2">
      <img
        :alt="post.title"
        class="h-full w-full rounded-2xl object-cover"
        :src="post.banner"
      />
    </div>
    <div class="h-1/2 py-3">
      <div class="mb-2 flex items-center justify-between">
        <div class="flex items-center gap-2">
          <img
            :src="post.authorAvatar"
            :alt="post.authorName"
            class="h-8 w-8 rounded-full"
          />
          <span class="text-foreground/80 text-sm">
            {{ post.authorName }}
          </span>
        </div>
      </div>

      <KunLink
        color="default"
        underline="none"
        :to="post.link"
        class-name="hover:text-primary-500 text-lg font-bold line-clamp-2"
      >
        <h1>{{ post.title }}</h1>
      </KunLink>

      <p class="text-foreground/80 mb-2 text-xs line-clamp-2">
        {{ post.description }}
      </p>
      <div class="flex flex-wrap gap-1">
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
