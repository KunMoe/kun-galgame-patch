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
      <!-- About post banners are pre-optimized AVIF authored at build time
           (/posts/notice/*/banner.avif). `provider="none"` returns the URL
           untouched — skips the IPX → sharp roundtrip + 5-min FS cache miss
           latency that would otherwise hit on every cold load. Explicit
           width/height + lazy also eliminate the layout shift / reflow churn
           previously seen with 4 banner cards loading in parallel. -->
      <KunImage
        :src="props.post.banner"
        :alt="props.post.title"
        provider="none"
        loading="lazy"
        :width="512"
        :height="288"
        class-name="h-full w-full object-cover"
      />
    </div>
    <div class="space-y-2 p-4">
      <div class="flex flex-wrap gap-2">
        <KunChip
          v-if="aboutDirectoryLabelMap[props.post.directory]"
          variant="flat"
          color="primary"
          size="sm"
        >
          {{ aboutDirectoryLabelMap[props.post.directory] }}
        </KunChip>
        <KunChip variant="flat" size="sm">
          {{ readMinutes }} 分钟阅读
        </KunChip>
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
