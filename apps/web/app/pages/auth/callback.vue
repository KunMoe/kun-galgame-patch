<script setup lang="ts">
import type { UserState } from '~/stores/userStore'

useKunSeoMeta({
  title: 'OAuth 回调',
  description: '正在完成 OAuth 登录'
})

definePageMeta({
  ssr: false
})

const api = useApi()
const userStore = useUserStore()

const error = ref<string | null>(null)
const processed = ref(false)

onMounted(async () => {
  if (processed.value) return
  processed.value = true

  const params = verifyOAuthCallback()
  if (!params) {
    error.value = 'OAuth callback verification failed'
    setTimeout(() => navigateTo('/login'), 2000)
    return
  }

  const res = await api.post<UserState>('/auth/oauth/callback', {
    code: params.code,
    code_verifier: params.codeVerifier
  })

  if (res.code !== 0) {
    error.value = res.message
    // Per docs/oauth/api-reference.md, code 10014 (HTTP 403) means the
    // account is banned — re-logging in won't help, so go to the dedicated
    // banned-account page instead of looping back to /login.
    if (res.code === 10014) {
      await navigateTo({
        path: '/account-banned',
        query: res.message ? { reason: res.message } : {}
      })
      return
    }
    useKunMessage(res.message || '登录失败', 'error')
    setTimeout(() => navigateTo('/login'), 2000)
    return
  }

  userStore.setUser(res.data)
  useKunMessage('登录成功!', 'success')
  await navigateTo(`/user/${res.data.uid}/resource`)
})
</script>

<template>
  <div class="flex min-h-[50vh] items-center justify-center">
    <KunCard class-name="w-full max-w-sm" :bordered="false">
      <div class="flex flex-col items-center gap-4 py-8">
        <template v-if="error">
          <p class="text-danger text-center">{{ error }}</p>
          <p class="text-default-400 text-sm">Redirecting to login...</p>
        </template>
        <template v-else>
          <KunIcon
            name="svg-spinners:90-ring-with-bg"
            class="text-primary size-10"
          />
          <p class="text-default-500">正在完成登录...</p>
        </template>
      </div>
    </KunCard>
  </div>
</template>
