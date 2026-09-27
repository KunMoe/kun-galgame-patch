<script setup lang="ts">
const props = defineProps<{ item: FollowingActivityItem }>()

const external = computed(() => props.item.path === null)
const to = computed(() => props.item.path ?? props.item.url)
const target = computed(() => (external.value ? '_blank' : undefined))
const cover = computed(() => imageServiceUrl(props.item.cover_image_hash))
const coverFailed = ref(false)
</script>

<template>
  <div class="flex gap-3">
    <KunNsfwMask
      v-if="cover && !coverFailed"
      :nsfw="item.content_limit === 'nsfw'"
      rounded="rounded-md"
      class="w-28 shrink-0"
    >
      <NuxtLink :to="to" :target="target" class="block">
        <KunImage
          :src="cover"
          :alt="item.title"
          aspect-ratio="16 / 9"
          class-name="rounded-md"
          @error="coverFailed = true"
        />
      </NuxtLink>
    </KunNsfwMask>

    <div class="min-w-0 flex-1 space-y-1">
      <div class="flex items-center gap-1.5">
        <NuxtLink
          :to="to"
          :target="target"
          class="text-default-800 hover:text-primary min-w-0 truncate text-sm font-medium"
        >
          {{ item.title }}
        </NuxtLink>
        <KunIcon
          v-if="external"
          name="lucide:external-link"
          class="text-default-400 size-3.5 shrink-0"
        />
        <KunChip
          v-if="item.content_limit === 'nsfw'"
          size="sm"
          color="danger"
          variant="flat"
        >
          NSFW
        </KunChip>
      </div>
      <p
        v-if="item.excerpt"
        class="text-default-500 line-clamp-2 text-sm break-words"
      >
        {{ item.excerpt }}
      </p>
      <p class="text-default-400 text-xs">
        {{ formatDistanceToNow(item.occurred_at) }}
      </p>
    </div>
  </div>
</template>
