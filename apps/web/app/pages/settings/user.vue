<script setup lang="ts">
// Account settings page.
//
// After Phase 4 of the OAuth migration, identity fields (username / bio /
// password / email / avatar) are owned by the OAuth server. The local
// PUT /user/{username,bio,password,email,avatar} endpoints have all been
// removed -- editing identity now happens on oauth.kungal.com. This page
// therefore:
//
//   1. Links users out to the OAuth profile page for identity edits
//   2. Offers a local 消息通知设置 panel (frontend-only, persisted in
//      localStorage via userStore.muted_message_types)
//   3. Offers 清除网站数据 (localStorage.clear + logout + re-login)
//
// (1) is just a link; (2) and (3) are the panels we re-implement here from
// the original next-web design.
import { MESSAGE_TYPE, MESSAGE_TYPE_MAP } from '~/constants/message'

useKunSeoMeta({
  title: '账户设置',
  description: '管理您的账户'
})

const userStore = useUserStore()
const api = useApi()

const config = useRuntimeConfig()
const oauthServerUrl = (config.public.oauthServerUrl as string) ?? 'https://oauth.kungal.com/api/v1'
// Strip the trailing /api/v1 to get the frontend origin.
const oauthOrigin = oauthServerUrl.replace(/\/api\/v\d+\/?$/, '')

// ─── 消息通知设置 ───────────────────────────────────
//
// MESSAGE_TYPE has an empty-string sentinel at the end; we filter it out.
// "system" stays in the grid but is rendered as a disabled checkbox so
// users know it cannot be silenced (mirrors the next-web original).
const visibleTypes = MESSAGE_TYPE.filter((t) => t) as string[]

const isEnabled = (type: string) =>
  !(userStore.user.muted_message_types ?? []).includes(type)

const toggleType = (type: string) => {
  if (type === 'system') return
  userStore.toggleMutedMessageType(type)
}

// ─── 清除网站数据 ──────────────────────────────────
const resetOpen = ref(false)
const resetting = ref(false)

const handleReset = async () => {
  resetting.value = true
  try {
    // Best-effort: kill the server-side session too so the cookie is no
    // longer valid. If the network is dead we still clear local state.
    await api.post('/auth/logout').catch(() => {})
    userStore.logout()
    if (typeof localStorage !== 'undefined') {
      localStorage.clear()
    }
    resetOpen.value = false
    useKunMessage('您已成功清除网站所有数据, 请重新登录', 'success')
    // Give the toast a moment, then hard-reload so any in-memory caches
    // (pinia / nuxt payload) are dropped along with localStorage.
    setTimeout(() => {
      window.location.href = '/login'
    }, 1500)
  } finally {
    resetting.value = false
  }
}
</script>

<template>
  <div class="my-4 w-full">
    <KunHeader name="账户设置" description="您可以在此处管理您的账户与本地偏好" />

    <div class="mx-auto my-4 max-w-3xl space-y-6">
      <!-- 1. 身份字段编辑跳转到 OAuth -->
      <KunCard :bordered="true">
        <template #header>
          <h2 class="px-1 pt-1 text-xl font-medium">身份信息</h2>
        </template>
        <div class="space-y-3 text-sm">
          <p class="text-default-600">
            用户名、密码、邮箱、头像、个人简介等身份信息现由
            <a
              :href="oauthOrigin"
              target="_blank"
              rel="noopener noreferrer"
              class="text-primary hover:underline"
            >
              KUN 账号中心
            </a>
            统一管理，请前往修改，修改后本站可能需要等待最多 10 分钟生效。
          </p>
          <div class="flex flex-wrap gap-3 pt-1">
            <a :href="`${oauthOrigin}/profile`" target="_blank" rel="noopener noreferrer">
              <KunButton color="primary" variant="flat">
                <KunIcon name="lucide:external-link" class="size-4" />
                前往修改身份信息
              </KunButton>
            </a>
          </div>
        </div>
      </KunCard>

      <!-- 2. 消息通知设置（前端本地，localStorage 持久化） -->
      <KunCard :bordered="true">
        <template #header>
          <h2 class="px-1 pt-1 text-xl font-medium">消息通知设置</h2>
        </template>
        <div class="space-y-4 text-sm">
          <p class="text-default-600">
            选择您想要接收通知的消息类型。如果下面的选项无法点击，请尝试清除网站数据。
          </p>
          <div class="grid grid-cols-2 gap-3">
            <KunCheckBox
              v-for="type in visibleTypes"
              :key="type"
              color="primary"
              :model-value="type === 'system' ? true : isEnabled(type)"
              :label="MESSAGE_TYPE_MAP[type] || type"
              :disabled="type === 'system'"
              @update:model-value="() => toggleType(type)"
            />
          </div>
          <p class="text-default-500 text-xs">
            消息通知设置保存在浏览器本地，不会同步到服务器，更换设备需要重新设置。
          </p>
        </div>
      </KunCard>

      <!-- 3. 清除网站数据 -->
      <KunCard :bordered="true">
        <template #header>
          <h2 class="px-1 pt-1 text-xl font-medium">清除网站数据</h2>
        </template>
        <div class="space-y-3 text-sm">
          <p class="text-default-600">
            如果您遇到任何报错（例如本页面中的消息通知无法点击），可以尝试清除网站所有数据。
            清除将会丢失所有 Galgame 发布草稿，并且需要重新登录。
            <strong>不会</strong>影响您的账户信息。
          </p>
          <div class="flex items-center justify-between gap-2 pt-1">
            <p class="text-danger-500 text-xs">注意，清除操作无法撤销</p>
            <KunButton color="danger" :disabled="resetting" @click="resetOpen = true">
              清除
            </KunButton>
          </div>
        </div>
      </KunCard>
    </div>

    <KunModal v-model:modal-value="resetOpen" inner-class-name="max-w-md">
      <div class="space-y-4">
        <h3 class="text-lg font-semibold">您确定要清除网站所有数据吗？</h3>
        <p class="text-foreground/80 text-sm">
          清除网站数据将丢失所有 Galgame 发布草稿、本地偏好、登录状态，并需要重新登录。
          清除操作<strong>不会</strong>影响您的账户信息。
        </p>
        <div class="flex justify-end gap-2">
          <KunButton variant="bordered" :disabled="resetting" @click="resetOpen = false">
            关闭
          </KunButton>
          <KunButton
            color="danger"
            :loading="resetting"
            :disabled="resetting"
            @click="handleReset"
          >
            确定清除
          </KunButton>
        </div>
      </div>
    </KunModal>
  </div>
</template>
