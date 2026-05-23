<script setup lang="ts">
// Account settings — full proxy mode (docs/oauth/02-user-profile.md "代理
// 模式"). The user can edit name / bio / avatar / password / email inline
// without ever leaving moyu; each form posts to a moyu backend route that
// forwards the body + access_token to OAuth and re-emits OAuth's response
// verbatim. Identity remains OAuth-owned (moyu does no local validation).
//
// Endpoints used:
//   - PATCH /auth/me                 (name / bio)
//   - POST  /auth/me/avatar          (multipart file → OAuth handles
//                                    image_service upload internally)
//   - PUT   /auth/password           ({old_password, new_password})
//   - POST  /auth/email/send-code    (triggers verification mail)
//   - PUT   /auth/email              ({email, code})
//
// After any change that touches displayed fields (name / avatar / bio),
// refetch /auth/me so the top-bar avatar / pinia store reflect immediately.

import { MESSAGE_TYPE, MESSAGE_TYPE_MAP } from '~/constants/message'
import type { UserState } from '~/stores/userStore'

useKunSeoMeta({
  title: '账户设置',
  description: '管理您的账户'
})

const userStore = useUserStore()
const api = useApi()

// ─── /auth/me snapshot (read-only fields like email) ─────────────────
// userStore only carries the slim Me payload; the OAuth-extended fields
// (email, full bio if not yet on userStore) come from refetching /auth/me
// directly. Cached locally so the UI can show "current email" without
// re-querying on every keystroke.
interface MeFull {
  id: number
  sub: string
  name: string
  email?: string
  avatar: string
  avatar_image_hash: string
  bio: string
  moemoepoint: number
}
const me = ref<MeFull | null>(null)
const refreshMe = async () => {
  const res = await api.get<MeFull>('/auth/me')
  if (res.code === 0 && res.data) {
    me.value = res.data
    // Mirror display fields into pinia so top-bar / dropdown re-render.
    userStore.setUser(res.data as Partial<UserState>)
  }
}
onMounted(refreshMe)

// ─── 基本资料 (name / bio) ─────────────────────────────────────────────
const profileForm = reactive({ name: '', bio: '' })
const profileSaving = ref(false)
watch(
  me,
  (m) => {
    if (m) {
      profileForm.name = m.name
      profileForm.bio = m.bio
    }
  },
  { immediate: true }
)
const profileDirty = computed(
  () =>
    !!me.value &&
    (profileForm.name !== me.value.name || profileForm.bio !== me.value.bio)
)
const saveProfile = async () => {
  if (!profileDirty.value) return
  profileSaving.value = true
  try {
    // Per docs §"字段都用指针类型语义" — send only changed fields so
    // unrelated ones stay untouched (omit = 不动).
    const body: Record<string, unknown> = {}
    if (me.value && profileForm.name !== me.value.name)
      body.name = profileForm.name
    if (me.value && profileForm.bio !== me.value.bio) body.bio = profileForm.bio
    const res = await api.patch<MeFull>('/auth/me', body)
    if (res.code === 0) {
      useKunMessage('已保存', 'success')
      await refreshMe()
    } else {
      useKunMessage(res.message || '保存失败', 'error')
    }
  } finally {
    profileSaving.value = false
  }
}

// ─── 头像 (multipart upload) ──────────────────────────────────────────
const avatarFile = ref<File | null>(null)
const avatarPreview = computed(() =>
  avatarFile.value ? URL.createObjectURL(avatarFile.value) : null
)
const avatarUploading = ref(false)
const uploadAvatar = async () => {
  if (!avatarFile.value) return
  avatarUploading.value = true
  try {
    const fd = new FormData()
    fd.append('file', avatarFile.value, avatarFile.value.name)
    const config = useRuntimeConfig()
    const base = config.public.apiBase || ''
    // $fetch doesn't expose a clean way to send multipart + read JSON
    // envelope with our useApi wrapper, so we drop down to raw fetch
    // here and inspect _data.
    const r = await $fetch
      .raw<{ code: number; message: string; data: unknown }>(
        `${base}/auth/me/avatar`,
        { method: 'POST', body: fd, credentials: 'include' }
      )
      .catch((e) => e?.response)
    const env = r?._data ?? { code: -1, message: '上传失败', data: null }
    if (env.code === 0) {
      useKunMessage('头像已更新', 'success')
      avatarFile.value = null
      await refreshMe()
    } else {
      useKunMessage(env.message || '上传失败', 'error')
    }
  } finally {
    avatarUploading.value = false
  }
}

// ─── 身份层操作 (jump to OAuth profile) ───────────────────────────────
// Per docs/oauth/02-user-profile.md §身份操作 vs 展示操作 (policy effective
// 2026-05-23): 改密码 / 改邮箱 / 注销账号 / 2FA 等"身份层"操作必须由
// OAuth 自己的前端承担。moyu 不再代理这些端点（之前实现的 PUT
// /auth/password、POST /auth/email/send-code、PUT /auth/email 已经从
// router 移除）。这里只放一个跳转按钮，带 return 参数让用户改完跳回。
const config = useRuntimeConfig()
const oauthServerUrl =
  (config.public.oauthServerUrl as string) ??
  'https://oauth.kungal.com/api/v1'
const oauthOrigin = oauthServerUrl.replace(/\/api\/v\d+\/?$/, '')
const oauthProfileUrl = computed(() => {
  if (!import.meta.client) return `${oauthOrigin}/profile`
  return `${oauthOrigin}/profile?return=${encodeURIComponent(window.location.href)}`
})

// ─── 消息通知设置 (frontend-only, localStorage) ───────────────────────
const visibleTypes = MESSAGE_TYPE.filter((t) => t) as string[]
const isEnabled = (type: string) =>
  !(userStore.user.muted_message_types ?? []).includes(type)
const toggleType = (type: string) => {
  if (type === 'system') return
  userStore.toggleMutedMessageType(type)
}

// ─── 清除网站数据 ─────────────────────────────────────────────────────
const resetOpen = ref(false)
const resetting = ref(false)
const handleReset = async () => {
  resetting.value = true
  try {
    await api.post('/auth/logout').catch(() => {})
    userStore.logout()
    if (typeof localStorage !== 'undefined') localStorage.clear()
    resetOpen.value = false
    useKunMessage('您已成功清除网站所有数据, 请重新登录', 'success')
    setTimeout(() => {
      window.location.href = '/login'
    }, 1500)
  } finally {
    resetting.value = false
  }
}

// ─── Resolved avatar URL (current) ────────────────────────────────────
// Mirror resolveAvatarUrl's preference: image_service hash → 256-px webp
// thumb, fall back to legacy `avatar` URL when no hash is present.
const imageBed =
  (config.public as { imageBed?: string }).imageBed ?? ''
const currentAvatarUrl = computed(() => {
  if (!me.value) return ''
  if (me.value.avatar_image_hash && imageBed) {
    const h = me.value.avatar_image_hash
    return `${imageBed.replace(/\/$/, '')}/img/${h.slice(0, 2)}/${h.slice(2, 4)}/${h}-256.webp`
  }
  return me.value.avatar || ''
})
</script>

<template>
  <div class="my-4 w-full">
    <KunHeader name="账户设置" description="您可以在此处管理您的账户与本地偏好" />

    <div class="mx-auto my-4 max-w-3xl space-y-6">
      <!-- 1. 基本资料 (name / bio) -->
      <!-- Wrapped in <form> so the browser treats this as a discrete
           credential form (silences Chrome's "[DOM] Password field is not
           contained in a form" warning for the password card below — that
           heuristic is global per page, not per card, so EVERY edit area
           on this page needs to be inside its own <form>). `autocomplete=
           "username"` on the name input lets password managers associate
           the right user with the password form. -->
      <KunCard :bordered="true">
        <template #header>
          <h2 class="px-1 pt-1 text-xl font-medium">基本资料</h2>
        </template>
        <form class="space-y-4" @submit.prevent="saveProfile">
          <KunInput
            v-model="profileForm.name"
            label="用户名"
            placeholder="2-17 个字符，全局唯一"
            helper-text="改名后老的 @用户名 不会自动重定向。"
            autocomplete="username"
            name="username"
          />
          <KunTextarea
            v-model="profileForm.bio"
            label="个人简介"
            placeholder="≤ 107 个字符"
            :rows="3"
            :maxlength="107"
            show-char-count
          />
          <div class="flex justify-end">
            <KunButton
              type="submit"
              color="primary"
              :loading="profileSaving"
              :disabled="!profileDirty || profileSaving"
            >
              保存
            </KunButton>
          </div>
        </form>
      </KunCard>

      <!-- 2. 头像 -->
      <KunCard :bordered="true">
        <template #header>
          <h2 class="px-1 pt-1 text-xl font-medium">头像</h2>
        </template>
        <div class="flex flex-col gap-4 sm:flex-row sm:items-start">
          <div class="flex shrink-0 flex-col items-center gap-2">
            <img
              :src="
                avatarPreview ||
                currentAvatarUrl ||
                '/kungalgame-trans.webp'
              "
              alt="当前头像"
              class="bg-default-100 size-24 rounded-full object-cover"
            />
            <span v-if="avatarPreview" class="text-default-500 text-xs">
              预览
            </span>
          </div>
          <div class="flex-1 space-y-3">
            <p class="text-default-500 text-xs">
              图片由 OAuth 服务转发到 image_service，建议 ≤ 4 MiB。
              头像更新后，本站会立刻看到新头像（可能需要刷新顶栏）。
            </p>
            <KunFileInput
              v-model="avatarFile"
              accept="image/*"
              :max-size="4 * 1024 * 1024"
              hint="JPEG / PNG / WebP，≤ 4 MiB"
              trigger-text="选择新头像"
              trigger-icon="lucide:image-plus"
              @error-pick="useKunMessage($event, 'error')"
            />
            <div class="flex justify-end">
              <KunButton
                color="primary"
                :loading="avatarUploading"
                :disabled="!avatarFile || avatarUploading"
                @click="uploadAvatar"
              >
                上传并应用
              </KunButton>
            </div>
          </div>
        </div>
      </KunCard>

      <!-- 3. 身份信息（跳转模式） -->
      <!-- 改邮箱 / 改密码 / 注销账号 / 2FA 等"身份层"操作由 OAuth profile
           独家承担（docs/oauth/02-user-profile.md §身份操作 vs 展示操作，
           2026-05-23 政策）。原因：安全审计单点、未来 2FA / 异地通知
           只需改一处、避免邮箱劫持攻击面跨多个站点放大。
           moyu 这里只提供跳转入口，URL 携带 `?return=<currentUrl>` 让
           OAuth 改完直接跳回原页。 -->
      <KunCard :bordered="true">
        <template #header>
          <h2 class="px-1 pt-1 text-xl font-medium">身份与安全</h2>
        </template>
        <div class="space-y-4">
          <p class="text-default-600 text-sm">
            修改邮箱、密码、注销账号等敏感操作需要在
            <strong>KUN 账号中心</strong>完成 ——
            这是为了让所有身份层操作集中在一个安全审计点。改完后页面会自动跳回。
          </p>
          <div
            class="border-default/20 bg-default-50/50 space-y-2 rounded-lg border p-3 text-sm"
          >
            <div class="flex items-center justify-between">
              <span class="text-default-500">当前邮箱</span>
              <span class="font-medium">{{ me?.email || '（未绑定）' }}</span>
            </div>
            <div class="flex items-center justify-between">
              <span class="text-default-500">用户 ID</span>
              <span class="font-mono text-xs">{{ me?.id ?? '—' }}</span>
            </div>
          </div>
          <div class="flex justify-end">
            <a
              :href="oauthProfileUrl"
              target="_blank"
              rel="noopener noreferrer"
            >
              <KunButton color="primary" variant="flat">
                <KunIcon name="lucide:external-link" class="size-4" />
                前往 KUN 账号中心
              </KunButton>
            </a>
          </div>
        </div>
      </KunCard>

      <!-- 5. 消息通知设置 (frontend-only) -->
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

      <!-- 6. 清除网站数据 -->
      <KunCard :bordered="true">
        <template #header>
          <h2 class="px-1 pt-1 text-xl font-medium">清除网站数据</h2>
        </template>
        <div class="space-y-3 text-sm">
          <p class="text-default-600">
            如果您遇到任何报错，可以尝试清除网站所有数据。
            清除将会丢失所有 Galgame 发布草稿，并且需要重新登录。
            <strong>不会</strong>影响您的账户信息。
          </p>
          <div class="flex items-center justify-between gap-2 pt-1">
            <p class="text-danger-500 text-xs">注意，清除操作无法撤销</p>
            <KunButton
              color="danger"
              :disabled="resetting"
              @click="resetOpen = true"
            >
              清除
            </KunButton>
          </div>
        </div>
      </KunCard>
    </div>

    <KunModal v-model="resetOpen" inner-class-name="max-w-md">
      <div class="space-y-4">
        <h3 class="text-lg font-semibold">您确定要清除网站所有数据吗？</h3>
        <p class="text-foreground/80 text-sm">
          清除网站数据将丢失所有 Galgame 发布草稿、本地偏好、登录状态，并需要重新登录。
          清除操作<strong>不会</strong>影响您的账户信息。
        </p>
        <div class="flex justify-end gap-2">
          <KunButton
            variant="bordered"
            :disabled="resetting"
            @click="resetOpen = false"
          >
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
