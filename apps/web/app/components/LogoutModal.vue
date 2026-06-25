<script setup lang="ts">
// Logout-scope chooser shared by the top-bar dropdown + mobile menu. Two session
// layers exist (moyu's own session, and the central OAuth/SSO session); let the
// user pick which to end and spell out the impact. See docs/oauth/07-logout.md.
const { open } = useLogoutModal()
const userStore = useUserStore()
const api = useApi()
const { clearAll: clearKnownAccounts } = useKnownAccounts()
const pending = ref<'local' | 'everywhere' | null>(null)

// Clear moyu's own session: the server session cookie + the client store.
// Best-effort server call — a failure must not block the local clear.
const clearLocal = async () => {
  try {
    await api.post('/auth/logout')
  } finally {
    userStore.logout()
  }
}

const chooseLocal = async () => {
  if (pending.value) return
  pending.value = 'local'
  await clearLocal()
  open.value = false
  pending.value = null
}

const chooseEverywhere = async () => {
  if (pending.value) return
  pending.value = 'everywhere'
  await clearLocal()
  // Leaving the whole SSO on a shared device → also forget the local account
  // roster (the "适合公共 / 共享设备" option). Local-only logout keeps it.
  clearKnownAccounts()
  startOAuthLogout() // top-level redirect to the OP; clears the SSO session
}
</script>

<template>
  <KunModal v-model="open" inner-class-name="max-w-lg">
    <div class="space-y-4">
      <div class="space-y-1">
        <h3 class="text-foreground text-lg font-semibold">退出登录</h3>
        <p class="text-default-500 text-sm">请选择退出范围：</p>
      </div>

      <button
        type="button"
        :disabled="!!pending"
        class="border-primary-200 bg-primary-50/50 hover:bg-primary-100/60 w-full rounded-xl border p-4 text-left transition-colors disabled:opacity-60"
        @click="chooseEverywhere"
      >
        <div class="flex items-start gap-3">
          <KunIcon
            :name="pending === 'everywhere' ? 'lucide:loader-circle' : 'lucide:log-out'"
            :class="`text-primary mt-0.5 size-5 shrink-0 ${pending === 'everywhere' ? 'animate-spin' : ''}`"
          />
          <div class="space-y-1">
            <div class="text-foreground flex items-center gap-2 font-medium">
              退出本站和 OAuth 账号
              <span class="bg-primary-100 text-primary-700 rounded px-1.5 py-0.5 text-xs">推荐</span>
            </div>
            <p class="text-default-500 text-xs leading-relaxed">
              本站与 OAuth 账号都会退出；其它已登录的站点会在下次刷新登录态时一并退出；再次登录需重新验证身份。适合公共 / 共享设备。
            </p>
          </div>
        </div>
      </button>

      <button
        type="button"
        :disabled="!!pending"
        class="border-default-200 hover:bg-default-100 w-full rounded-xl border p-4 text-left transition-colors disabled:opacity-60"
        @click="chooseLocal"
      >
        <div class="flex items-start gap-3">
          <KunIcon
            :name="pending === 'local' ? 'lucide:loader-circle' : 'lucide:monitor'"
            :class="`text-default-500 mt-0.5 size-5 shrink-0 ${pending === 'local' ? 'animate-spin' : ''}`"
          />
          <div class="space-y-1">
            <div class="text-foreground font-medium">仅退出本站</div>
            <p class="text-default-500 text-xs leading-relaxed">
              只退出本站；OAuth 账号与其它站点保持登录；再次登录本站可免密直接进入。适合自己的设备。
            </p>
          </div>
        </div>
      </button>

      <p class="text-default-400 text-xs">登出不会清除您的编辑草稿（Galgame、回复等）。</p>

      <div class="flex justify-end pt-1">
        <KunButton variant="light" :disabled="!!pending" @click="open = false">
          取消
        </KunButton>
      </div>
    </div>
  </KunModal>
</template>
