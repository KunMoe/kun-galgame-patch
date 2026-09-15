<script setup lang="ts">
const userStore = useUserStore()
const { refreshMe } = useRefreshMe()
const { refresh: refreshUnread } = useUnreadCounts()

const { open: openAuthModal } = useAuthModal()

onMounted(async () => {
  if (userStore.user.id) {
    await Promise.all([refreshMe(), refreshUnread()])
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
