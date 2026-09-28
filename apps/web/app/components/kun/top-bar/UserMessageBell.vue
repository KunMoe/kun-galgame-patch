<script setup lang="ts">
const userStore = useUserStore()
const messageStore = useMessageStore()
const chat = useChatStore()

const hasNotice = computed(() =>
  messageStore.unreadTypes.some(
    (type) => !userStore.user.muted_message_types?.includes(type)
  )
)
const hasUnread = computed(() => hasNotice.value || chat.hasUnread)

const href = computed(() =>
  chat.hasUnread && !hasNotice.value ? '/messages' : '/message/notice'
)
const tooltip = computed(() => {
  if (hasNotice.value) return '您有新消息!'
  return chat.hasUnread ? '您有新私信' : '我的消息'
})
</script>

<template>
  <KunTooltip :text="tooltip" position="bottom">
    <KunButton
      is-icon-only
      variant="light"
      color="default"
      aria-label="我的消息"
      :href="href"
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
