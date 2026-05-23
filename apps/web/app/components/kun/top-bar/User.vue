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

// Top-bar surfaces a single solid "登录" button (registration is offered
// inside the LoginForm via a link, so an extra top-bar entry would be
// redundant). Clicking the button pops the existing LoginForm in a modal —
// no need to navigate away from the current page just to log in.
const loginOpen = ref(false)

const unreadMessageTypes = ref<string[]>([])

const fetchUserStatus = async () => {
  const res = await api.get<UserState>('/auth/me')
  if (res.code === 0) {
    // setUser merges into existing state and preserves muted_message_types.
    userStore.setUser(res.data)
    return
  }
  // Only wipe the pinia store on signals the server-side session is truly
  // dead — auth-expired (40101) means the middleware refresh permanently
  // failed and the cookie was cleared; unauthorized (40100) means no cookie
  // was sent. Any OTHER non-zero code (5xx, network blip, OAuth slow during
  // background refresh, transient transport error → middleware returns 401
  // but KEEPS the cookie for the next-request retry) must NOT wipe the
  // store, or a single bad request silently logs the user out while the
  // server still considers them authenticated. Previous behavior (logout
  // on ANY non-zero) was the "登录之后过一会自动退出" bug.
  if (res.code === 40100 || res.code === 40101) {
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
      <KunButton
        size="sm"
        color="primary"
        variant="solid"
        @click="loginOpen = true"
      >
        登录
      </KunButton>
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

  <KunModal v-model="loginOpen" inner-class-name="max-w-sm">
    <div class="flex flex-col items-center gap-6 py-4">
      <h2 class="text-2xl font-bold">登录</h2>
      <LoginForm />
    </div>
  </KunModal>
</template>
