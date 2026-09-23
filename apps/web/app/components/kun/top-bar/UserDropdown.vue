<script setup lang="ts">
import type { KnownAccount } from '~/composables/useKnownAccounts'
import { kunMoyuMoe } from '~/config/moyu-moe'

const userStore = useUserStore()
const api = useApi()
const { openLogoutModal } = useLogoutModal()
const { accounts } = useKnownAccounts()

const checking = ref(false)
const logOpen = ref(false)
const creatorOpen = ref(false)

const isCreator = computed(() => userStore.user.roles?.includes('creator') ?? false)
const showCreatorApply = computed(
  () => !userStore.isModerator && !isCreator.value
)

const popover = ref<{ close: () => void } | null>(null)
const route = useRoute()
watch(
  () => route.fullPath,
  () => popover.value?.close()
)

const closeMenu = () => popover.value?.close()

const openModal = (target: 'log' | 'logout' | 'creator') => {
  popover.value?.close()
  if (target === 'log') logOpen.value = true
  else if (target === 'creator') creatorOpen.value = true
  else openLogoutModal()
}

const showAccountSwitch = ref(false)
const switchableAccounts = computed(() =>
  accounts.value.filter((a) => a.sub !== userStore.user.sub)
)

const returnAfterAuth = (): string | undefined => {
  const ownProfile = `/user/${userStore.user.id}`
  return route.path === ownProfile || route.path.startsWith(`${ownProfile}/`)
    ? undefined
    : route.fullPath
}

const onSwitchAccount = (account: KnownAccount) => {
  popover.value?.close()
  startOAuthSwitchAccount(account.sub, returnAfterAuth())
}

const onAddAccount = () => {
  popover.value?.close()
  startOAuthAddAccount(returnAfterAuth())
}

const needsReauth = (account: KnownAccount) =>
  (account.roles ?? []).includes('admin') ||
  (account.roles ?? []).includes('ren')

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
  <!-- Fixed width, not min-w: KunButton is inline-flex, so the auto-width panel's
       shrink-to-fit takes the SUM of the button rows, not the widest row. Two
       buttons summed under the 256px floor and hid it; the third pushed the menu
       to 437px. -->
  <KunPopover ref="popover" position="bottom-end" inner-class="w-64 p-2">
    <template #trigger>
      <button
        type="button"
        aria-label="账号菜单"
        class="flex cursor-pointer items-center justify-center rounded-full focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
      >
        <KunAvatar
          :user="toKunUser(userStore.user)"
          :is-navigation="false"
          size="md"
        />
      </button>
    </template>

    <div class="space-y-1">
      <div class="px-2 py-1">
        <p class="truncate font-semibold">{{ userStore.user.name }}</p>
      </div>
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
      <button
        type="button"
        class="hover:bg-default-100 flex w-full items-center gap-2 rounded px-2 py-2 text-sm"
        @click="showAccountSwitch = !showAccountSwitch"
      >
        <KunIcon name="lucide:users-round" class="size-4" />
        账号切换
        <KunIcon
          name="lucide:chevron-right"
          class="text-foreground/40 ml-auto size-4 transition-transform"
          :class="showAccountSwitch ? 'rotate-90' : ''"
        />
      </button>

      <div v-if="showAccountSwitch" class="space-y-1 pl-2">
        <button
          v-for="acc in switchableAccounts"
          :key="acc.sub"
          type="button"
          class="hover:bg-default-100 flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-sm"
          @click="onSwitchAccount(acc)"
        >
          <KunAvatar :user="toKunUser(acc)" :is-navigation="false" size="sm" />
          <span class="min-w-0 flex-1">
            <span class="block truncate">{{ acc.name }}</span>
            <span
              v-if="needsReauth(acc)"
              class="text-default-400 block text-xs"
            >
              管理员账号切换需重新登录
            </span>
          </span>
        </button>

        <button
          type="button"
          class="text-primary hover:bg-primary-50 flex w-full items-center gap-2 rounded px-2 py-1.5 text-sm font-medium"
          @click="onAddAccount"
        >
          <KunIcon name="lucide:user-plus" class="size-4" />
          添加新账号
        </button>
      </div>
      <NuxtLink
        to="/settings/user"
        class="hover:bg-default-100 flex items-center gap-2 rounded px-2 py-2 text-sm"
      >
        <KunIcon name="lucide:settings" class="size-4" />
        系统和用户设置
      </NuxtLink>
      <NuxtLink
        to="/doc/notice/feedback"
        class="hover:bg-default-100 flex items-center gap-2 rounded px-2 py-2 text-sm"
      >
        <KunIcon name="lucide:circle-help" class="size-4" />
        帮助与反馈
      </NuxtLink>
      <button
        v-if="showCreatorApply"
        type="button"
        class="text-primary hover:bg-primary-50 flex w-full items-center gap-2 rounded px-2 py-2 text-sm font-medium transition-colors"
        @click="openModal('creator')"
      >
        <KunIcon name="lucide:sparkles" class="size-4" />
        创作者申请
        <KunIcon
          name="lucide:chevron-right"
          class="text-primary/50 ml-auto size-4"
        />
      </button>
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

      <KunButton
        variant="light"
        color="primary"
        size="sm"
        full-width
        rounded="md"
        class-name="justify-between"
        :href="kunMoyuMoe.domain.kungal"
        target="_blank"
        @click="closeMenu"
      >
        <span class="flex items-center gap-2">
          <KunImage
            src="/favicon.webp"
            alt=""
            :skeleton="false"
            class-name="size-4 rounded-full"
          />
          鲲 Galgame 论坛
        </span>
        <KunIcon name="lucide:external-link" class="size-4" />
      </KunButton>
    </div>
  </KunPopover>

  <KunTopBarMoemoepointLog v-model="logOpen" />
  <KunTopBarCreatorApply v-model="creatorOpen" />
</template>
