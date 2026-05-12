<script setup lang="ts">
// Landing page for OAuth code 10014 (HTTP 403). Per docs/oauth/api-reference.md
// the frontend MUST NOT redirect banned users back to /login — re-logging in
// hits the same 10014. They should see a static "联系管理员" message instead.

useKunSeoMeta({
  title: '账号已被封禁',
  description: '账号已被封禁，无法登录'
})

definePageMeta({
  ssr: false
})

const route = useRoute()
const reason = computed(() => (route.query.reason as string) || '')

const config = useRuntimeConfig()
const oauthOrigin = computed(() => {
  const u = (config.public.oauthServerUrl as string) || ''
  try {
    return new URL(u).origin
  } catch {
    return 'https://oauth.kungal.com'
  }
})
</script>

<template>
  <div class="container mx-auto flex min-h-[60vh] max-w-xl items-center justify-center">
    <KunCard class-name="w-full">
      <div class="space-y-4 p-6 text-center">
        <KunIcon
          name="lucide:shield-alert"
          class="text-danger mx-auto size-12"
        />
        <h1 class="text-2xl font-bold">账号已被封禁</h1>
        <p class="text-default-600">
          您的账号被管理员标记为封禁状态，无法登录本站。
        </p>
        <p v-if="reason" class="text-default-500 text-sm">
          原因：{{ reason }}
        </p>
        <div class="border-default/20 bg-default-50 rounded-lg border p-3 text-left text-sm">
          <p class="font-semibold">下一步可以：</p>
          <ul class="text-default-600 mt-2 list-disc space-y-1 pl-5">
            <li>
              在
              <a
                :href="oauthOrigin"
                target="_blank"
                rel="noopener noreferrer"
                class="text-primary hover:underline"
              >KUN OAuth</a>
              联系管理员了解封禁原因
            </li>
            <li>申诉成功后再尝试登录本站</li>
          </ul>
        </div>
        <div class="flex justify-center gap-2 pt-2">
          <KunButton variant="bordered" @click="navigateTo('/')">
            返回首页
          </KunButton>
        </div>
      </div>
    </KunCard>
  </div>
</template>
