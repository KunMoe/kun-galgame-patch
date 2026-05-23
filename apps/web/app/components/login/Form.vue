<script setup lang="ts">
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

// Use oauthWebUrl (frontend, dev :9420 / prod oauth.kungal.com), not oauthServerUrl
// (API, dev :9277). In prod they're same-origin, but in dev they're split ports.
const oauthWebUrl = computed(() => (config.public.oauthWebUrl as string) || '')
const registerUrl = computed(() => `${oauthWebUrl.value}/register`)
// Local /auth/forgot is just a redirect page; we link directly to OAuth's
// reset flow to save the extra hop.
const forgotUrl = computed(() => `${oauthWebUrl.value}/forgot`)
</script>

<template>
  <div class="flex w-72 flex-col gap-4">
    <p class="text-default-500 text-center text-sm">
      使用 鲲 Galgame OAuth 登录以继续
    </p>

    <KunButton
      color="primary"
      size="lg"
      full-width
      :loading="loading"
      :disabled="loading"
      @click="handleOAuthLogin"
    >
      <KunIcon v-if="!loading" name="lucide:log-in" class="size-5" />
      鲲 Galgame OAuth 登录
    </KunButton>

    <KunTextDivider text="或" />

    <a :href="forgotUrl" target="_blank" rel="noopener noreferrer">
      <KunButton color="primary" variant="bordered" full-width>
        <KunIcon name="lucide:external-link" class="size-4" />
        忘记密码
      </KunButton>
    </a>

    <div class="flex items-center justify-center">
      <span class="mr-2 text-sm">没有 鲲 Galgame OAuth 账号?</span>
      <a
        :href="registerUrl"
        target="_blank"
        rel="noopener noreferrer"
        class="text-primary text-sm hover:underline"
      >
        前往注册
      </a>
    </div>
  </div>
</template>
