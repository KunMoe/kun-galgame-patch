<script setup lang="ts">
const userStore = useUserStore()
const api = useApi()

const logoutOpen = ref(false)
const loggingOut = ref(false)
const checking = ref(false)

const handleLogout = async () => {
  loggingOut.value = true
  try {
    await api.post('/auth/logout')
  } finally {
    loggingOut.value = false
    logoutOpen.value = false
    userStore.logout()
    useKunMessage('您已经成功登出!', 'success')
    await navigateTo('/login')
  }
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
  <KunPopover position="bottom-end" inner-class="p-2 min-w-64">
    <template #trigger>
      <button
        type="button"
        class="border-secondary ring-secondary/20 shrink-0 rounded-full border-2 transition-transform hover:scale-105"
        aria-label="用户菜单"
      >
        <KunAvatar :user="userStore.user" :is-navigation="false" size="sm" />
      </button>
    </template>

    <div class="space-y-1">
      <div class="px-2 py-1">
        <p class="font-semibold">{{ userStore.user.name }}</p>
      </div>
      <div
        class="text-foreground/80 flex items-center justify-between rounded px-2 py-1 text-sm"
      >
        <span class="flex items-center gap-2">
          <KunIcon name="lucide:lollipop" class="size-4" />
          萌萌点
        </span>
        <span>{{ userStore.user.moemoepoint }}</span>
      </div>
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
        to="/about/notice/feedback"
        class="hover:bg-default-100 flex items-center gap-2 rounded px-2 py-2 text-sm"
      >
        <KunIcon name="lucide:circle-help" class="size-4" />
        帮助与反馈
      </NuxtLink>
      <button
        type="button"
        class="text-danger hover:bg-danger/10 flex w-full items-center gap-2 rounded px-2 py-2 text-left text-sm"
        @click="logoutOpen = true"
      >
        <KunIcon name="lucide:log-out" class="size-4" />
        退出登录
      </button>

      <button
        type="button"
        class="text-secondary hover:bg-secondary/10 flex w-full items-center justify-between rounded px-2 py-2 text-sm disabled:opacity-50"
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
      </button>
    </div>
  </KunPopover>

  <KunModal v-model="logoutOpen" inner-class-name="max-w-md">
    <div class="space-y-4">
      <h3 class="text-lg font-semibold">您确定要登出网站吗?</h3>
      <p class="text-foreground/80 text-sm">
        登出将会清除您的登录状态, 但是不会清除您的编辑草稿 (Galgame,
        回复等), 您可以稍后继续登录
      </p>
      <div class="flex justify-end gap-2">
        <KunButton
          color="danger"
          variant="light"
          :disabled="loggingOut"
          @click="logoutOpen = false"
        >
          关闭
        </KunButton>
        <KunButton
          color="primary"
          :loading="loggingOut"
          :disabled="loggingOut"
          @click="handleLogout"
        >
          确定
        </KunButton>
      </div>
    </div>
  </KunModal>
</template>
