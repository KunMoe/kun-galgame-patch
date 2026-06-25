<script setup lang="ts">
// Logout-scope chooser shared by the top-bar dropdown + mobile menu.
//
// Two session layers exist: moyu's own BFF session (moyu_session cookie + Redis)
// and the central OAuth/SSO session on the OP. moyu is cross-TLD from the OP, so
// it has exactly two real logout primitives (docs/oauth/07 + 09 §3.4):
//   1. POST /auth/logout (moyu)      → clears moyu's BFF session only.
//   2. RP-initiated logout (top-nav) → also revokes the CURRENT OAuth account at
//      the OP (single active session) + wipes the OP's persisted login.
// It deliberately does NOT expose a "退出全部账号" button: the OP's per-account /
// logout-all bag APIs are same-TLD account-center features (09 §3.4) that a
// cross-TLD downstream can't call. So we offer per-account logout (the everyday
// action) + a local-only option, and let the user opt into forgetting this
// device's remembered-account roster for shared machines.
const { open } = useLogoutModal()
const userStore = useUserStore()
const api = useApi()
const { accounts, clearAll: clearKnownAccounts } = useKnownAccounts()

const pending = ref<'current' | 'local' | null>(null)
// Opt-in (default off): also drop this browser's remembered-account list. Logging
// out the current account must NOT silently forget your OTHER saved accounts —
// you want them in the switcher next time. Turn this on only on shared devices.
const forgetDevice = ref(false)

// Clear moyu's own session: the server session cookie + the client store.
// Best-effort server call — a failure must not block the local clear.
const clearLocal = async () => {
  try {
    await api.post('/auth/logout')
  } finally {
    userStore.logout()
  }
}

const maybeForget = () => {
  if (forgetDevice.value) clearKnownAccounts()
}

// 退出当前账号: clear moyu + top-level redirect to the OP logout entry, which
// revokes the current OAuth account and clears the OP's persisted login. Other
// sites holding this account's token drop out on their next token refresh.
// (startOAuthLogout navigates away, so no need to reset pending / close.)
const chooseCurrent = async () => {
  if (pending.value) return
  pending.value = 'current'
  await clearLocal()
  maybeForget()
  startOAuthLogout()
}

// 仅退出本站: end only moyu's session. The OAuth account and every other site
// stay signed in, and re-entering moyu logs straight back in (auto-consent).
const chooseLocal = async () => {
  if (pending.value) return
  pending.value = 'local'
  await clearLocal()
  maybeForget()
  open.value = false
  pending.value = null
}
</script>

<template>
  <KunModal v-model="open" inner-class-name="max-w-lg">
    <div class="space-y-4">
      <div class="space-y-3">
        <h3 class="text-foreground text-lg font-semibold">退出登录</h3>
        <!-- Make "current account" concrete: per-account logout acts on THIS
             identity, so show who it is. -->
        <div class="bg-default-100 flex items-center gap-3 rounded-xl px-3 py-2">
          <KunAvatar :user="userStore.user" :is-navigation="false" size="sm" />
          <div class="min-w-0">
            <p class="text-foreground truncate text-sm font-medium">
              {{ userStore.user.name }}
            </p>
            <p class="text-default-400 text-xs">当前账号</p>
          </div>
        </div>
      </div>

      <button
        type="button"
        :disabled="!!pending"
        class="border-primary-200 bg-primary-50/50 hover:bg-primary-100/60 w-full rounded-xl border p-4 text-left transition-colors disabled:opacity-60"
        @click="chooseCurrent"
      >
        <div class="flex items-start gap-3">
          <KunIcon
            :name="pending === 'current' ? 'lucide:loader-circle' : 'lucide:log-out'"
            :class="`text-primary mt-0.5 size-5 shrink-0 ${pending === 'current' ? 'animate-spin' : ''}`"
          />
          <div class="space-y-1">
            <div class="text-foreground flex items-center gap-2 font-medium">
              退出当前账号
              <span class="bg-primary-100 text-primary-700 rounded px-1.5 py-0.5 text-xs">推荐</span>
            </div>
            <p class="text-default-500 text-xs leading-relaxed">
              退出本站，同时退出你的账号。其它用同一个账号登录的网站（论坛、Wiki 等）稍后也会一起退出。下次进入需要重新登录。和别人共用的电脑建议选这个。
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
              只退出本站，你的账号仍是登录状态。其它网站不受影响，下次再来本站会自动登录、不用重新输入。自己的电脑选这个就行。
            </p>
          </div>
        </div>
      </button>

      <!-- Shared-device escape hatch: forget the remembered-account roster too.
           Off by default so a normal logout keeps your other accounts handy. -->
      <label
        v-if="accounts.length"
        class="border-default-200 flex cursor-pointer items-center justify-between gap-3 rounded-xl border border-dashed px-3 py-2"
      >
        <div class="min-w-0">
          <p class="text-foreground text-sm">清除本设备记住的账号</p>
          <p class="text-default-400 text-xs leading-relaxed">
            为了方便切换，本站会在这台电脑上记住你登录过的 {{ accounts.length }} 个账号。打开后会清空这份记录，切换菜单里就不再显示它们，下次要用得重新登录添加。和别人共用的电脑建议打开。
          </p>
        </div>
        <KunSwitch v-model="forgetDevice" :disabled="!!pending" />
      </label>

      <p class="text-default-400 text-xs">退出后不会丢失你正在编辑的草稿（Galgame、回复等）。</p>

      <div class="flex justify-end pt-1">
        <KunButton variant="light" :disabled="!!pending" @click="open = false">
          取消
        </KunButton>
      </div>
    </div>
  </KunModal>
</template>
