<script setup lang="ts">
import type { KnownAccount } from '~/composables/useKnownAccounts'

const userStore = useUserStore()
const api = useApi()
const { openLogoutModal } = useLogoutModal()
const { accounts } = useKnownAccounts()

const checking = ref(false)
const logOpen = ref(false)
const creatorOpen = ref(false)

// "创作者申请" entry — only for regular users without the creator role:
// moderators / admins already publish Galgame directly, and existing creators
// don't need to apply, so both are excluded.
const isCreator = computed(() => userStore.user.roles?.includes('creator') ?? false)
const showCreatorApply = computed(
  () => !userStore.isModerator && !isCreator.value
)

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

const openModal = (target: 'log' | 'logout' | 'creator') => {
  popover.value?.close()
  if (target === 'log') logOpen.value = true
  else if (target === 'creator') creatorOpen.value = true
  else openLogoutModal()
}

// Account switching (docs/oauth/09-account-switching.md). Both actions are
// top-level authorize redirects — moyu is cross-TLD from the OP and can't read
// its session bag over fetch, so switching always bounces through /oauth/authorize.
// returnTo = the current path so the user lands back where they were.
const onSwitchAccount = (account: KnownAccount) => {
  if (account.id === userStore.user.id) return // already the active account
  popover.value?.close()
  startOAuthSwitchAccount(account.sub, route.fullPath)
}

const onAddAccount = () => {
  popover.value?.close()
  startOAuthAddAccount(route.fullPath)
}

// Switching INTO an admin account forces an OP re-login (step-up, §3.5) — flag
// it so the choice isn't surprising. The OP enforces it regardless; this is a hint.
const needsReauth = (account: KnownAccount) =>
  (account.roles ?? []).includes('admin')

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
        class="flex cursor-pointer items-center justify-center rounded-full focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
      >
        <KunAvatar :user="userStore.user" :is-navigation="false" size="md" />
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
      <!-- 账号切换 — nested submenu. trigger="hover" opens it on desktop hover
           and (kun-ui converts hover→tap on touch) on mobile, matching the
           other top-bar hover menus and the requested "手机端 hover 变点击".
           The list is the local known-accounts cache; clicking an account or
           "添加新账号" is a top-level authorize redirect (moyu is cross-TLD from
           the OP, so it can't read the OP session bag directly).
           See docs/oauth/09-account-switching.md §3.6.
           KunPopover wraps the trigger in two inline-block <div>s (its root +
           the inner triggerRef wrapper), so the row shrank to content width.
           `w-full` falls through to the root and `[&>div:first-child]:w-full`
           hits the inner wrapper — inline-block honours an explicit width, so
           both fill the menu with no display override or scoped CSS (which would
           stamp this component's scope id onto the teleport-root KunModals it
           renders → Vue "extraneous attrs" warnings). No full-width-trigger
           prop exists on KunPopover. -->
      <KunPopover
        class="w-full [&>div:first-child]:w-full"
        trigger="hover"
        position="bottom-start"
        inner-class="min-w-60 p-1"
      >
        <template #trigger>
          <button
            type="button"
            class="hover:bg-default-100 flex w-full items-center gap-2 rounded px-2 py-2 text-sm"
          >
            <KunIcon name="lucide:users-round" class="size-4" />
            账号切换
            <KunIcon
              name="lucide:chevron-right"
              class="text-foreground/40 ml-auto size-4"
            />
          </button>
        </template>

        <div class="space-y-1">
          <p
            v-if="accounts.length"
            class="text-default-500 px-2 py-1 text-xs"
          >
            切换账号
          </p>
          <template v-for="acc in accounts" :key="acc.sub">
            <!-- The currently-active account: marked, not clickable. -->
            <div
              v-if="acc.id === userStore.user.id"
              class="bg-default-100 flex items-center gap-2 rounded px-2 py-1.5 text-sm"
            >
              <KunAvatar :user="acc" :is-navigation="false" size="sm" />
              <span class="min-w-0 flex-1 truncate">{{ acc.name }}</span>
              <KunIcon
                name="lucide:check"
                class="text-primary size-4 shrink-0"
              />
            </div>
            <button
              v-else
              type="button"
              class="hover:bg-default-100 flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-sm"
              @click="onSwitchAccount(acc)"
            >
              <KunAvatar :user="acc" :is-navigation="false" size="sm" />
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
          </template>

          <div
            v-if="accounts.length"
            class="bg-default-200/60 my-1 h-px"
          />

          <button
            type="button"
            class="text-primary hover:bg-primary-50 flex w-full items-center gap-2 rounded px-2 py-2 text-sm font-medium"
            @click="onAddAccount"
          >
            <KunIcon name="lucide:user-plus" class="size-4" />
            添加新账号
          </button>
        </div>
      </KunPopover>
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
      <!-- 创作者申请 — accent-styled so it stands out as an aspirational action.
           Opens the root-sibling modal below; the popover closes first so it
           doesn't linger behind. -->
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
  <KunTopBarCreatorApply v-model="creatorOpen" />
</template>
