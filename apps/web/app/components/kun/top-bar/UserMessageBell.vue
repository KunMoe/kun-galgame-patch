<script setup lang="ts">
const messageStore = useMessageStore()

const hasUnread = computed(() => messageStore.hasAnyUnread)
</script>

<template>
  <KunTooltip
    :text="hasUnread ? '您有新消息!' : '我的消息'"
    position="bottom"
  >
    <KunBadge
      variant="dot"
      color="danger"
      placement="top-right"
      :show="hasUnread"
    >
      <KunButton
        is-icon-only
        variant="light"
        color="default"
        aria-label="我的消息"
        href="/message/notice"
      >
        <KunIcon
          :name="hasUnread ? 'lucide:bell-ring' : 'lucide:bell'"
          :class="hasUnread ? 'text-primary size-6' : 'text-default-500 size-6'"
        />
      </KunButton>
    </KunBadge>
  </KunTooltip>
</template>
