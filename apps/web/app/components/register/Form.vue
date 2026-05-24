<script setup lang="ts">
// Registration is handled entirely by the external 鲲 Galgame OAuth server.
// Per the unified-registration flow (docs/integration/oauth/05-registration.md),
// this component is a jump button that bounces to OAuth web's hosted
// register page with the full authorize URL stashed as ?redirect=. After
// registration OAuth web auto-logs-in and silently completes the
// authorization-code round-trip back to moyu (auto_consent=true skips
// the consent UI), landing the user back here logged in. Same in-window
// navigation as the login button — no target="_blank" / external link
// icon since users stay in the OAuth ecosystem the whole time.
const loadingRegister = ref(false)
const loadingLogin = ref(false)

const handleOAuthRegister = async () => {
  loadingRegister.value = true
  try {
    await startOAuthRegister()
  } catch {
    loadingRegister.value = false
  }
}

const handleOAuthLogin = async () => {
  loadingLogin.value = true
  try {
    await startOAuthLogin()
  } catch {
    loadingLogin.value = false
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
      :loading="loadingRegister"
      :disabled="loadingRegister"
      @click="handleOAuthRegister"
    >
      <KunIcon v-if="!loadingRegister" name="lucide:user-plus" class="size-5" />
      使用 鲲 Galgame OAuth 注册
    </KunButton>

    <KunTextDivider text="或" />

    <KunButton
      color="primary"
      variant="bordered"
      full-width
      :loading="loadingLogin"
      :disabled="loadingLogin"
      @click="handleOAuthLogin"
    >
      <KunIcon v-if="!loadingLogin" name="lucide:log-in" class="size-5" />
      已有账号, 直接登录
    </KunButton>

    <div class="flex items-center justify-center">
      <span class="mr-2 text-sm">已经有账号了?</span>
      <NuxtLink to="/login" class="text-primary text-sm">登录账号</NuxtLink>
    </div>
  </div>
</template>
