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

const oauthOrigin = computed(
  () => (config.public.oauthServerUrl || '').replace(/\/api\/v\d+\/?$/, '')
)
const registerUrl = computed(() => `${oauthOrigin.value}/register`)
// Local /auth/forgot is just a redirect page; we link directly to OAuth's
// reset flow to save the extra hop.
const forgotUrl = computed(() => `${oauthOrigin.value}/forgot`)
</script>

<template>
  <div class="flex w-72 flex-col gap-4">
    <p class="text-default-500 text-center text-sm">
      使用 KUN 账号登录以继续
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
      KUN 账号登录
    </KunButton>

    <KunTextDivider text="或" />

    <a :href="forgotUrl" target="_blank" rel="noopener noreferrer">
      <KunButton color="primary" variant="bordered" full-width>
        <KunIcon name="lucide:external-link" class="size-4" />
        忘记密码
      </KunButton>
    </a>

    <div class="flex items-center justify-center">
      <span class="mr-2 text-sm">没有 KUN 账号?</span>
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
