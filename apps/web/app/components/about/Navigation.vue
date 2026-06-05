<script setup lang="ts">
interface Props {
  prev: KunPostMetadata | null
  next: KunPostMetadata | null
}

const props = defineProps<Props>()
</script>

<template>
  <nav
    class="border-default/20 mt-12 flex flex-col gap-4 border-t pt-6 sm:flex-row sm:justify-between"
  >
    <NuxtLink
      v-if="props.prev"
      :to="`/doc/${props.prev.slug}`"
      class="hover:bg-default-100 flex-1 rounded-lg p-4 transition-colors"
    >
      <div class="text-default-500 mb-1 flex items-center gap-1 text-xs">
        <KunIcon name="lucide:arrow-left" class="size-3.5" />
        上一篇
      </div>
      <div class="font-medium line-clamp-1">{{ props.prev.title }}</div>
    </NuxtLink>
    <div v-else class="flex-1" />

    <NuxtLink
      v-if="props.next"
      :to="`/doc/${props.next.slug}`"
      class="hover:bg-default-100 flex-1 rounded-lg p-4 text-right transition-colors"
    >
      <div
        class="text-default-500 mb-1 flex items-center justify-end gap-1 text-xs"
      >
        下一篇
        <KunIcon name="lucide:arrow-right" class="size-3.5" />
      </div>
      <div class="font-medium line-clamp-1">{{ props.next.title }}</div>
    </NuxtLink>
    <div v-else class="flex-1" />
  </nav>
</template>
