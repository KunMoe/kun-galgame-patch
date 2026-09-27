<script setup lang="ts">
const chat = useChatStore()
const unread = computed(() => chat.hasUnread)
</script>

<template>
  <KunTooltip :text="unread ? '你有新私信' : '私信'" position="bottom">
    <KunButton
      is-icon-only
      variant="light"
      color="default"
      aria-label="私信"
      href="/messages"
    >
      <KunBadge
        variant="dot"
        color="danger"
        size="sm"
        placement="bottom-right"
        :show="unread"
        aria-label="有新私信"
      >
        <KunIcon
          name="lucide:mail"
          :class="unread ? 'text-primary size-6' : 'text-default-500 size-6'"
        />
      </KunBadge>
    </KunButton>
  </KunTooltip>
</template>
