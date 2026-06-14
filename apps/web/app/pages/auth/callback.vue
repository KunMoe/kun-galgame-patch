<script setup lang="ts">
import type { UserState } from '~/stores/userStore'

// OAuth callback is a pure transit page (exchanges the code for a session
// then redirects). Nothing to index — disable SEO so the URL doesn't show
// up in search results, and the noindex/nofollow stop crawlers from
// following the redirect chain.
useKunDisableSeo('OAuth 回调')

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
    error.value = '登录回调校验失败，请重新登录'
    setTimeout(() => navigateTo('/'), 2000)
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
    setTimeout(() => navigateTo('/'), 2000)
    return
  }

  userStore.setUser(res.data)
  useKunMessage('登录成功!', 'success')
  await navigateTo(`/user/${res.data.id}/resource`)
})
</script>

<template>
  <!-- w-full is required: the default layout wraps the page in a flex ROW, so
       without it this node shrinks to content width and pins left instead of
       centering. Card mirrors the sibling /account-banned status page. -->
  <div class="flex min-h-[60vh] w-full items-center justify-center px-4">
    <KunCard class-name="w-full max-w-sm">
      <div class="flex flex-col items-center gap-4 px-6 py-10 text-center">
        <template v-if="error">
          <KunIcon name="lucide:triangle-alert" class="text-danger size-12" />
          <div class="space-y-1">
            <h1 class="text-foreground text-lg font-bold">登录失败</h1>
            <p class="text-default-500 text-sm">{{ error }}</p>
          </div>
          <p class="text-default-400 text-xs">即将返回首页…</p>
        </template>
        <template v-else>
          <KunIcon
            name="svg-spinners:90-ring-with-bg"
            class="text-primary size-12"
          />
          <div class="space-y-1">
            <h1 class="text-foreground text-lg font-bold">正在完成登录</h1>
            <p class="text-default-500 text-sm">请稍候，正在验证您的身份…</p>
          </div>
        </template>
      </div>
    </KunCard>
  </div>
</template>
