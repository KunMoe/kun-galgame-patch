<script setup lang="ts">
// Password recovery is handled entirely by the KUN OAuth server (passwords
// are not stored on this site after Phase 4 of the OAuth migration). This
// page just bounces the user to OAuth's reset-password flow.
useKunSeoMeta({
  title: '忘记密码',
  description: '通过 KUN 账号中心重置密码'
})

const config = useRuntimeConfig()
const oauthOrigin = computed(
  () => (config.public.oauthServerUrl || '').replace(/\/api\/v\d+\/?$/, '')
)
const forgotUrl = computed(() => `${oauthOrigin.value}/forgot`)
</script>

<template>
  <div class="m-auto">
    <KunCard class-name="w-80" :bordered="false">
      <template #header>
        <div class="flex flex-col gap-2 p-6">
          <div class="bg-primary/10 mx-auto rounded-full p-3">
            <KunIcon name="lucide:lock-keyhole" class="text-primary size-6" />
          </div>
          <h1 class="text-center text-2xl font-bold">重置密码</h1>
          <p class="text-default-500 text-center text-sm">
            密码由 KUN 账号中心统一管理，请前往 KUN 重置
          </p>
        </div>
      </template>

      <div class="space-y-3 p-6 pt-0">
        <a :href="forgotUrl" target="_blank" rel="noopener noreferrer">
          <KunButton color="primary" full-width>
            <KunIcon name="lucide:external-link" class="size-4" />
            前往 KUN 账号中心重置密码
          </KunButton>
        </a>
        <NuxtLink to="/login">
          <KunButton variant="bordered" full-width>返回登录</KunButton>
        </NuxtLink>
      </div>
    </KunCard>
  </div>
</template>
