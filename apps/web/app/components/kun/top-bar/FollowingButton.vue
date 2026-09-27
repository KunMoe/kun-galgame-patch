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
    >
      <KunBadge
        variant="dot"
        color="danger"
        size="sm"
        placement="bottom-right"
        :show="unseen > 0"
        aria-label="有新动态"
      >
        <KunIcon
          name="lucide:rss"
          :class="
            unseen > 0 ? 'text-primary size-6' : 'text-default-500 size-6'
          "
        />
      </KunBadge>
    </KunButton>
  </KunTooltip>
</template>
