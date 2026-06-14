<script setup lang="ts">
const userStore = useUserStore()
const api = useApi()
const { openLogoutModal } = useLogoutModal()

const checking = ref(false)
const logOpen = ref(false)

// KunPopover only closes on outside-click / Escape — a click on an inner item
// is @click.stop'd, so navigating via a menu link leaves it open. Drive it shut
// ourselves: a route watcher covers every NuxtLink + programmatic navigateTo,
// and openModal() closes it before surfacing a modal so nothing lingers behind.
const popover = ref<{ close: () => void } | null>(null)
const route = useRoute()
watch(
  () => route.fullPath,
  () => popover.value?.close()
)

const openModal = (target: 'log' | 'logout') => {
  popover.value?.close()
  if (target === 'log') logOpen.value = true
  else openLogoutModal()
}

const handleCheckIn = async () => {
  if (checking.value || userStore.user.daily_check_in) return
  checking.value = true
  try {
    const res = await api.post<{ moemoepoint: number }>('/user/check-in')
    if (res.code === 0) {
      const gained = res.data.moemoepoint
      useKunMessage(
        gained > 0
          ? `签到成功! 您今天获得了 ${gained} 萌萌点`
          : '您的运气不好...今天没有获得萌萌点...',
        gained > 0 ? 'success' : 'info'
      )
      userStore.setUser({
        daily_check_in: 1,
        moemoepoint: userStore.user.moemoepoint + gained
      })
    } else {
      useKunMessage(res.message || '签到失败', 'error')
    }
  } finally {
    checking.value = false
  }
}
</script>

<template>
  <KunPopover ref="popover" position="bottom-end" inner-class="p-2 min-w-64">
    <template #trigger>
      <!-- Bare <button> (no KunButton ring/border — that felt foreign next to
           the rest of the top bar) purely so the trigger is keyboard-focusable
           and Enter/Space-activatable: since KunUI 0.15.0 the KunPopover wrapper
           is no longer a role="button"/focusable element, so the slotted trigger
           must carry its own focusability. KunAvatar renders a plain (non-
           focusable) image; a native button's activation click bubbles to the
           popover's own @click. -->
      <button
        type="button"
        aria-label="账号菜单"
        class="block cursor-pointer rounded-full focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
      >
        <KunAvatar :user="userStore.user" :is-navigation="false" size="sm" />
      </button>
    </template>

    <div class="space-y-1">
      <div class="px-2 py-1">
        <p class="font-semibold">{{ userStore.user.name }}</p>
      </div>
      <!-- 萌萌点 row doubles as the entry to the records modal — clicking opens
           the full ledger (OAuth is the source of truth). -->
      <button
        type="button"
        class="text-foreground/80 hover:bg-default-100 flex w-full items-center justify-between rounded px-2 py-1 text-sm"
        @click="openModal('log')"
      >
        <span class="flex items-center gap-2">
          <KunIcon name="lucide:lollipop" class="size-4" />
          萌萌点
        </span>
        <span class="flex items-center gap-1">
          {{ userStore.user.moemoepoint }}
          <KunIcon name="lucide:chevron-right" class="text-foreground/40 size-4" />
        </span>
      </button>
      <NuxtLink
        :to="`/user/${userStore.user.id}/resource`"
        class="hover:bg-default-100 flex items-center gap-2 rounded px-2 py-2 text-sm"
      >
        <KunIcon name="lucide:user-round" class="size-4" />
        用户主页
      </NuxtLink>
      <NuxtLink
        to="/settings/user"
        class="hover:bg-default-100 flex items-center gap-2 rounded px-2 py-2 text-sm"
      >
        <KunIcon name="lucide:settings" class="size-4" />
        信息设置
      </NuxtLink>
      <NuxtLink
        to="/doc/notice/feedback"
        class="hover:bg-default-100 flex items-center gap-2 rounded px-2 py-2 text-sm"
      >
        <KunIcon name="lucide:circle-help" class="size-4" />
        帮助与反馈
      </NuxtLink>
      <!-- Admin panel entry — only moderators / admins (OAuth role
           "moderator" or "admin", i.e. legacy role > 2) can reach /admin;
           isModerator covers both. The /admin route group is moderator-gated
           server-side too, so this is a visibility convenience, not the gate. -->
      <NuxtLink
        v-if="userStore.isModerator"
        to="/admin"
        class="hover:bg-default-100 flex items-center gap-2 rounded px-2 py-2 text-sm"
      >
        <KunIcon name="lucide:shield-check" class="size-4" />
        管理面板
      </NuxtLink>
      <KunButton
        variant="light"
        color="danger"
        size="sm"
        full-width
        rounded="md"
        class-name="justify-start"
        @click="openModal('logout')"
      >
        <KunIcon name="lucide:log-out" class="size-4" />
        退出登录
      </KunButton>

      <KunButton
        variant="light"
        color="secondary"
        size="sm"
        full-width
        rounded="md"
        class-name="justify-between"
        :disabled="!!userStore.user.daily_check_in || checking"
        @click="handleCheckIn"
      >
        <span class="flex items-center gap-2">
          <KunIcon name="lucide:calendar-check" class="size-4" />
          今日签到
        </span>
        <span v-if="userStore.user.daily_check_in" class="text-xs">
          签到过啦
        </span>
        <KunIcon
          v-else
          name="lucide:sparkles"
          class="text-secondary-500 size-5"
        />
      </KunButton>
    </div>
  </KunPopover>

  <KunTopBarMoemoepointLog v-model="logOpen" />
</template>
