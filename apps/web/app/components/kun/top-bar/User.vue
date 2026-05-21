<script setup lang="ts">
// The user store is persisted in a cookie (see apps/web/app/stores/userStore.ts
// `piniaPluginPersistedstate.cookies()`), so SSR already has the logged-in
// user available on the initial render -- no hydration guard / skeleton
// needed; the template reads userStore directly.
//
// onMounted still fires /auth/me + /message/unread to refresh the cookie
// snapshot (it can be up to 7 days stale) and pull live unread counts.
import type { UserState } from '~/stores/userStore'

const userStore = useUserStore()
const api = useApi()
const config = useRuntimeConfig()

// 注册 points at the OAuth server's register page; the local backend has no
// /auth/register endpoint (identity is owned by OAuth).
const oauthRegisterUrl = computed(
  () =>
    (config.public.oauthServerUrl || '').replace(/\/api\/v\d+\/?$/, '') +
    '/register'
)

const unreadMessageTypes = ref<string[]>([])

const fetchUserStatus = async () => {
  const res = await api.get<UserState>('/auth/me')
  if (res.code === 0) {
    // setUser merges into existing state and preserves muted_message_types.
    userStore.setUser(res.data)
  } else {
    userStore.logout()
  }
}

const fetchUnread = async () => {
  const res = await api.get<string[]>('/message/unread')
  if (res.code === 0) {
    unreadMessageTypes.value = res.data ?? []
  }
}

onMounted(async () => {
  if (userStore.user.id) {
    await Promise.all([fetchUserStatus(), fetchUnread()])
  }
})
</script>

<template>
  <div class="ml-auto flex items-center gap-2">
    <template v-if="!userStore.isLoggedIn">
      <KunButton size="sm" color="primary" variant="flat" href="/login">
        登录
      </KunButton>
      <a
        :href="oauthRegisterUrl"
        target="_blank"
        rel="noopener noreferrer"
        class="hidden lg:inline-flex"
      >
        <KunButton size="sm" color="primary">注册</KunButton>
      </a>
    </template>

    <KunTopBarNSFWSwitcher />

    <KunTopBarSearch />

    <div class="hidden sm:flex">
      <KunTopBarRandomGalgameButton is-icon-only variant="light" size="sm" />
    </div>

    <div class="hidden sm:flex">
      <KunTopBarThemeSwitcher />
    </div>

    <template v-if="userStore.isLoggedIn">
      <KunTopBarUserMessageBell
        :unread-message-types="unreadMessageTypes"
        @read-messages="unreadMessageTypes = []"
      />
      <KunTopBarUserDropdown />
    </template>
  </div>
</template>
