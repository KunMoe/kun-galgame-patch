<script setup lang="ts">
// Registration is handled entirely by the external 鲲 Galgame OAuth server — the
// backend exposes only OAuth callback, so the standalone register form was
// removed. This component now simply points users to the OAuth server's
// register page (same place LoginForm sends them to when they need to sign up)
// and offers the OAuth login button as a shortcut.
const config = useRuntimeConfig()
const loading = ref(false)

const handleOAuthLogin = async () => {
  loading.value = true
  try {
    await startOAuthLogin()
  } catch {
    loading.value = false
  }
}

const registerUrl = computed(
  () => `${(config.public.oauthWebUrl as string) || ''}/register`
)
</script>

<template>
  <div class="flex w-72 flex-col gap-4">
    <p class="text-default-500 text-center text-sm">
      本站账号由 鲲 Galgame OAuth 统一管理, 请前往注册
    </p>

    <KunButton
      color="primary"
      size="lg"
      full-width
      :href="registerUrl"
      target="_blank"
      rel="noopener noreferrer"
    >
      <KunIcon name="lucide:external-link" class="size-5" />
      前往 鲲 Galgame OAuth 注册
    </KunButton>

    <KunTextDivider text="或" />

    <KunButton
      color="primary"
      variant="bordered"
      full-width
      :loading="loading"
      :disabled="loading"
      @click="handleOAuthLogin"
    >
      <KunIcon v-if="!loading" name="lucide:log-in" class="size-5" />
      已有 鲲 Galgame OAuth 账号, 直接登录
    </KunButton>

    <div class="flex items-center justify-center">
      <span class="mr-2 text-sm">已经有账号了?</span>
      <NuxtLink to="/login" class="text-primary text-sm">登录账号</NuxtLink>
    </div>
  </div>
</template>
