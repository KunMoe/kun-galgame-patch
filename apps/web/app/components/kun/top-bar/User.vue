<script setup lang="ts">
// The user store is persisted in a cookie (see apps/web/app/stores/userStore.ts
// `piniaPluginPersistedstate.cookies()`), so SSR already has the logged-in
// user available on the initial render -- no hydration guard / skeleton
// needed; the template reads userStore directly.
//
// onMounted fires the shared refreshMe (/auth/me → setUser) to revalidate the
// cookie snapshot (it can be up to 7 days stale) on every full page load, plus
// /message/unread for live counts. Background revalidation on tab focus /
// reconnect is handled by plugins/revalidate-me.client.ts; both go through
// useRefreshMe so the staleTime dedup and auth-expiry handling stay in one place.
const userStore = useUserStore()
const messageStore = useMessageStore()
const api = useApi()
const { refreshMe } = useRefreshMe()

// Top-bar surfaces a single solid "登录" button. Clicking opens the app-wide
// login modal (AuthEntry: 登录 / 注册, both bounce to OAuth web) — the same modal
// every login-required action/page opens via useAuthModal(), mounted once in the
// default layout. The local /login + /register pages were deleted (L1 unified
// registration), so the modal is the only auth entry point.
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

    <KunTopBarSearch />

    <div class="hidden sm:flex">
      <KunTopBarRandomGalgameButton is-icon-only variant="light" size="sm" />
    </div>

    <template v-if="userStore.isLoggedIn">
      <KunTopBarUserMessageBell />
      <KunTopBarUserDropdown />
    </template>
  </div>
</template>
