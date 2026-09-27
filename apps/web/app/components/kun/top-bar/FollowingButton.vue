<script setup lang="ts">
const { unseen, check } = useFollowingUnseen()

onMounted(check)
</script>

<template>
  <KunTooltip
    :text="unseen > 0 ? '你关注的人有新动态' : '关注动态'"
    position="bottom"
  >
    <KunButton
      is-icon-only
      variant="light"
      color="default"
      aria-label="关注动态"
      href="/following"
      class-name="relative"
    >
      <KunIcon
        name="lucide:rss"
        :class="unseen > 0 ? 'text-primary size-6' : 'text-default-500 size-6'"
      />
      <span
        v-if="unseen > 0"
        class="bg-danger absolute right-1 bottom-1 size-2 rounded-full"
      />
    </KunButton>
  </KunTooltip>
</template>
