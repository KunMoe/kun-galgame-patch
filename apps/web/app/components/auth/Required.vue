<script setup lang="ts">
// Gate for login-required PAGES. Logged in → renders the page (default slot).
// Logged out → renders NOTHING and pops the global login modal (the same one the
// 登录 button shows), staying on the current URL. No on-page "需要登录" notice —
// the modal IS the prompt, so a placeholder would only duplicate it.
const userStore = useUserStore()
const { open } = useAuthModal()

// Client-only so the overlay pops after hydration (toggling a modal during SSR
// would only risk a hydration mismatch). A logout while sitting on the page
// re-pops it.
onMounted(() => {
  if (!userStore.isLoggedIn) {
    open()
  }
})
watch(
  () => userStore.isLoggedIn,
  (loggedIn) => {
    if (!loggedIn) {
      open()
    }
  }
)
</script>

<template>
  <template v-if="userStore.isLoggedIn">
    <slot />
  </template>
</template>
