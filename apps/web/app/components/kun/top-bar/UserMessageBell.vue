<script setup lang="ts">
const userStore = useUserStore()
const messageStore = useMessageStore()

const hasUnread = computed(() =>
  messageStore.unreadTypes.some(
    (type) => !userStore.user.muted_message_types?.includes(type)
  )
)
</script>

<template>
  <KunTooltip :text="hasUnread ? '您有新消息!' : '我的消息'" position="bottom">
    <KunButton
      is-icon-only
      variant="light"
      color="default"
      aria-label="我的消息"
      href="/message/notice"
      class-name="relative"
    >
      <KunIcon
        :name="hasUnread ? 'lucide:bell-ring' : 'lucide:bell'"
        :class="hasUnread ? 'text-primary size-6' : 'text-default-500 size-6'"
      />
      <span
        v-if="hasUnread"
        class="bg-danger absolute right-1 bottom-1 size-2 rounded-full"
      />
    </KunButton>
  </KunTooltip>
</template>
