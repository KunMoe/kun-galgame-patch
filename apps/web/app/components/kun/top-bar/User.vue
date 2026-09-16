<script setup lang="ts">
const userStore = useUserStore()
const messageStore = useMessageStore()
const api = useApi()
const { refreshMe } = useRefreshMe()

const { open: openAuthModal } = useAuthModal()

const fetchUnread = async () => {
  const res = await api.get<string[]>('/message/unread')
  if (res.code === 0) {
    messageStore.setUnread(res.data ?? [])
  }
}

onMounted(async () => {
  if (userStore.user.id) {
    await Promise.all([refreshMe(), fetchUnread()])
  }
})
</script>

<template>
  <div class="ml-auto flex items-center gap-2">
    <template v-if="!userStore.isLoggedIn">
      <KunButton
        size="sm"
        color="primary"
        variant="solid"
        @click="openAuthModal()"
      >
        登录
      </KunButton>
    </template>

    <KunTopBarNSFWSwitcher />

    <SearchPalette />

    <div class="hidden sm:flex">
      <KunTopBarRandomGalgameButton is-icon-only variant="light" size="sm" />
    </div>

    <template v-if="userStore.isLoggedIn">
      <KunTopBarUserMessageBell />
      <KunTopBarUserDropdown />
    </template>
  </div>
</template>
