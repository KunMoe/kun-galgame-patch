<script setup lang="ts">
import { aboutDirectoryLabelMap } from '~/constants/about'

interface Props {
  post: KunPostMetadata
}

const props = defineProps<Props>()

// The API field is snake_case `text_count` (see shared/types/about.d.ts);
// reading `textCount` yielded undefined → NaN 分钟阅读.
const readMinutes = computed(() =>
  Math.max(1, Math.round((props.post.text_count ?? 0) / 500))
)
</script>

<template>
  <NuxtLink
    :to="`/about/${props.post.slug}`"
    class="bg-background border-default/20 hover:bg-default-100 block overflow-hidden rounded-lg border transition-colors"
  >
    <div
      v-if="props.post.banner"
      class="bg-default-100 aspect-video w-full overflow-hidden"
    >
      <KunImage
        :src="props.post.banner"
        :alt="props.post.title"
        class-name="h-full w-full object-cover"
      />
    </div>
    <div class="space-y-2 p-4">
      <div class="flex flex-wrap gap-2">
        <KunBadge
          v-if="aboutDirectoryLabelMap[props.post.directory]"
          variant="flat"
          color="primary"
          size="sm"
        >
          {{ aboutDirectoryLabelMap[props.post.directory] }}
        </KunBadge>
        <KunBadge variant="flat" size="sm">
          {{ readMinutes }} 分钟阅读
        </KunBadge>
      </div>
      <h2 class="text-lg font-semibold line-clamp-2">
        {{ props.post.title }}
      </h2>
      <p class="text-default-500 text-sm line-clamp-3">
        {{ props.post.description }}
      </p>
      <div class="text-default-400 text-xs">
        {{ props.post.date ? formatDistanceToNow(props.post.date) : '' }}
      </div>
    </div>
  </NuxtLink>
</template>
