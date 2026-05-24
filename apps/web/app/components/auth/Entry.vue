<script setup lang="ts">
// Shared modal body for the top-bar (desktop User.vue + mobile MobileMenu.vue)
// "登录" entry point. Two buttons — login and register — both bounce to OAuth
// web; we no longer host a /login or /register page locally.
//
// startOAuthLogin / startOAuthRegister are fire-and-forget — they set
// window.location.href, so we keep loading state on so the button shows the
// spinner until the navigation commits.
const loadingLogin = ref(false)
const loadingRegister = ref(false)

const handleLogin = async () => {
  loadingLogin.value = true
  try {
    await startOAuthLogin()
  } catch {
    loadingLogin.value = false
  }
}

const handleRegister = async () => {
  loadingRegister.value = true
  try {
    await startOAuthRegister()
  } catch {
    loadingRegister.value = false
  }
}
</script>

<template>
  <div class="flex w-72 flex-col gap-4">
    <p class="text-default-500 text-center text-sm">
      本站账号由 鲲 Galgame OAuth 统一管理
    </p>

    <KunButton
      color="primary"
      size="lg"
      full-width
      :loading="loadingLogin"
      :disabled="loadingLogin || loadingRegister"
      @click="handleLogin"
    >
      <KunIcon v-if="!loadingLogin" name="lucide:log-in" class="size-5" />
      使用 鲲 Galgame OAuth 登录
    </KunButton>

    <KunButton
      color="primary"
      variant="bordered"
      size="lg"
      full-width
      :loading="loadingRegister"
      :disabled="loadingLogin || loadingRegister"
      @click="handleRegister"
    >
      <KunIcon v-if="!loadingRegister" name="lucide:user-plus" class="size-5" />
      注册 鲲 Galgame OAuth 账户
    </KunButton>
  </div>
</template>
