<script setup lang="ts">
import { kunMoyuMoe } from '~/config/moyu-moe'
import {
  kunMobileAdminItem,
  kunMobileNavItem,
  KUN_CONTENT_LIMIT_MAP
} from '~/constants/top-bar'
import type { KunNsfwPreference } from '~/stores/settingStore'
import { useBodyScrollLock } from '@kun/ui/app/composables/useBodyScrollLock'

interface Props {
  isOpen: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{ 'update:isOpen': [value: boolean] }>()
const closeMenu = () => emit('update:isOpen', false)

const userStore = useUserStore()
const api = useApi()

// Theme switcher (mirrored from the desktop KunTopBarThemeSwitcher popover —
// reproduced as a horizontal 3-segmented control here so phone users get the
// same control surface that's hidden on mobile in the top bar).
const colorMode = useColorMode()
const themes = [
  { key: 'light', label: '浅色', icon: 'lucide:sun' },
  { key: 'dark', label: '深色', icon: 'lucide:moon' },
  { key: 'system', label: '系统', icon: 'lucide:sun-moon' }
] as const
const setTheme = (key: 'light' | 'dark' | 'system') => {
  colorMode.preference = key
}

// NSFW content_limit picker (mobile mirror of KunTopBarNSFWSwitcher — that
// component is a popover that's awkward on touch; the inline 3-tile control
// matches the theme picker above). location.reload() on change for the same
// reason as the desktop switcher: useApi captures content_limit at setup
// time, so an in-place update only takes effect on the next navigation.
const settingStore = useSettingStore()
const nsfwOptions = [
  { key: 'sfw', icon: 'lucide:shield-check' },
  { key: 'all', icon: 'lucide:circle-slash' },
  { key: 'nsfw', icon: 'lucide:ban' }
] as const satisfies ReadonlyArray<{ key: KunNsfwPreference; icon: string }>
const setNsfw = (key: KunNsfwPreference) => {
  settingStore.setNsfwPreference(key)
  if (import.meta.client) location.reload()
}

// Icon map. Kept in this file (not in constants/top-bar.ts) so the icon
// asset list scanner picks them up via the literal KunIcon name="..." form
// at the call sites below; adding a generic `icon?: string` to the
// constants list would also work but requires re-running `npm run icons`.
const ICON_BY_HREF: Record<string, string> = {
  '/galgame': 'lucide:gamepad-2',
  '/edit/create': 'lucide:plus-circle',
  '/ranking/user': 'lucide:chart-column-big',
  '/doc': 'lucide:book-open',
  '/comment': 'lucide:message-square',
  '/resource': 'lucide:puzzle',
  '/doc/notice/feedback': 'lucide:mail',
  '/admin': 'lucide:shield-check'
}
const iconFor = (href: string) => ICON_BY_HREF[href] ?? 'lucide:chevron-right'

// Section grouping: first 4 items (kunNavItem) are the primary nav; the
// extras after that are utility links. Admin (if present) gets its own
// section. Keeps the menu scannable instead of one long flat list.
const primaryItems = computed(() => kunMobileNavItem.slice(0, 4))
const utilityItems = computed(() => kunMobileNavItem.slice(4))
const adminItems = computed(() => (userStore.isAdmin ? kunMobileAdminItem : []))

// "登录" button → open the app-wide login modal (useAuthModal, mounted in the
// default layout — same modal every login-required action/page opens). closeMenu
// first so the slide-out animation runs cleanly before the modal opens on top.
const { open: openAuthModal } = useAuthModal()
const handleLoginClick = () => {
  closeMenu()
  openAuthModal()
}

// Logout (mirrors UserDropdown.handleLogout but routed through the menu).
const loggingOut = ref(false)
const handleLogout = async () => {
  loggingOut.value = true
  try {
    await api.post('/auth/logout')
  } finally {
    loggingOut.value = false
    userStore.logout()
    useKunMessage('您已经成功登出!', 'success')
    closeMenu()
    await navigateTo('/')
  }
}

// Body scroll lock — use the shared singleton refcount from @kun/ui so
// opening the menu while a Modal is also up doesn't unlock the body when
// the menu closes. (Previously this file wrote document.body.style
// directly, which clobbered any concurrent overlay's lock.)
const { lock, unlock } = useBodyScrollLock()
let locked = false
watch(
  () => props.isOpen,
  (open) => {
    if (open && !locked) {
      lock()
      locked = true
    } else if (!open && locked) {
      unlock()
      locked = false
    }
  }
)
onUnmounted(() => {
  if (locked) {
    unlock()
    locked = false
  }
})

// Esc to close.
const onKey = (e: KeyboardEvent) => {
  if (e.key === 'Escape' && props.isOpen) closeMenu()
}
onMounted(() => {
  if (import.meta.client) document.addEventListener('keydown', onKey)
})
onUnmounted(() => {
  if (import.meta.client) document.removeEventListener('keydown', onKey)
})
</script>

<template>
  <!-- Teleport to <body> so the menu escapes the parent `<nav>`'s
       backdrop-filter stacking context. KunTopBar's <nav> has its own
       `backdrop-blur`, which per CSS spec creates a containing block for
       backdrop-filter — any descendant's backdrop-blur ends up blurring
       only the parent's already-rendered surface, not what's behind on
       the actual viewport, so the frosted-glass effect silently
       collapses to a flat tint. Teleporting moves the menu DOM out of
       that boundary while keeping it bound to this component's reactive
       state, so we get a real backdrop blur over the page content. -->
  <Teleport to="body">
    <Transition
      enter-active-class="transition-all duration-200 ease-out"
      leave-active-class="transition-all duration-150 ease-in"
      enter-from-class="opacity-0 -translate-y-2"
      leave-to-class="opacity-0 -translate-y-2"
    >
      <!-- Fixed positioning with `top-16 inset-x-0 bottom-0` + `h-[calc(100dvh-4rem)]`
         picks up iOS's dynamic viewport (avoids the URL-bar overlap that
         100vh produces). `pb-[env(safe-area-inset-bottom)]` reserves room
         for the iOS home indicator so the last list item is fully tappable.
         `bg-background/70 + backdrop-blur-2xl` gives a frosted-glass effect
         where the page underneath shows through softly — matches modern
         iOS / macOS sheet aesthetics. -->
      <div
        v-if="props.isOpen"
        class="bg-background/70 fixed inset-x-0 top-16 bottom-0 z-30 h-[calc(100dvh-4rem)] overflow-y-auto backdrop-blur-2xl backdrop-saturate-150 md:hidden"
        :style="{ paddingBottom: 'env(safe-area-inset-bottom)' }"
        @click.self="closeMenu"
      >
        <div class="mx-auto flex max-w-md flex-col gap-4 px-4 pt-5 pb-6">
          <!-- ── User card (top): logged-in state shows avatar + name + 萌萌点;
             logged-out state shows a single primary Login CTA. -->
          <section
            v-if="userStore.isLoggedIn"
            class="border-default/20 bg-default-50/40 flex items-center gap-3 rounded-2xl border p-3"
          >
            <KunAvatar
              :user="userStore.user"
              :is-navigation="false"
              size="md"
            />
            <div class="min-w-0 flex-1">
              <p class="truncate font-semibold">{{ userStore.user.name }}</p>
              <p
                class="text-default-500 mt-0.5 flex items-center gap-1 text-xs"
              >
                <KunIcon name="lucide:sparkles" class="size-3.5" />
                萌萌点 {{ userStore.user.moemoepoint }}
              </p>
            </div>
            <KunButton
              variant="light"
              color="default"
              size="sm"
              is-icon-only
              aria-label="退出登录"
              :loading="loggingOut"
              :disabled="loggingOut"
              @click="handleLogout"
            >
              <KunIcon name="lucide:log-out" class="size-4" />
            </KunButton>
          </section>

          <section
            v-else
            class="border-default/20 bg-default-50/40 flex items-center gap-3 rounded-2xl border p-3"
          >
            <NuxtLink
              class="flex shrink-0 items-center gap-2"
              to="/"
              @click="closeMenu"
            >
              <KunImage
                src="/favicon.webp"
                :alt="kunMoyuMoe.titleShort"
                :width="40"
                :height="40"
                class-name="rounded-xl"
              />
            </NuxtLink>
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm font-semibold">
                {{ kunMoyuMoe.creator.name }}
                <KunChip size="sm" variant="flat" color="primary" class="ml-1">
                  补丁
                </KunChip>
              </p>
              <p class="text-default-500 mt-0.5 text-xs">登录解锁完整功能</p>
            </div>
            <KunButton
              color="primary"
              variant="solid"
              size="sm"
              @click="handleLoginClick"
            >
              <KunIcon name="lucide:log-in" class="size-4" />
              登录
            </KunButton>
          </section>

          <!-- ── Primary nav ── -->
          <section class="space-y-1">
            <p
              class="text-default-400 px-3 text-xs font-semibold tracking-wider uppercase"
            >
              主菜单
            </p>
            <nav class="flex flex-col gap-0.5">
              <!-- 首页 — kunMobileNavItem starts at 下载, so the menu had no way
                   back to the homepage (the logged-in user card has no logo
                   link). Pin an explicit 首页 entry at the top. -->
              <NuxtLink
                to="/"
                class="hover:bg-default-100 active:bg-default-200 flex items-center gap-3 rounded-xl px-3 py-3.5 transition-colors"
                @click="closeMenu"
              >
                <KunIcon
                  name="lucide:house"
                  class="text-default-500 size-5 shrink-0"
                />
                <span class="text-sm font-medium">首页</span>
                <KunIcon
                  name="lucide:chevron-right"
                  class="text-default-300 ml-auto size-4"
                />
              </NuxtLink>
              <NuxtLink
                v-for="item in primaryItems"
                :key="item.href"
                :to="item.href"
                class="hover:bg-default-100 active:bg-default-200 flex items-center gap-3 rounded-xl px-3 py-3.5 transition-colors"
                @click="closeMenu"
              >
                <KunIcon
                  :name="iconFor(item.href)"
                  class="text-default-500 size-5 shrink-0"
                />
                <span class="text-sm font-medium">{{ item.name }}</span>
                <KunIcon
                  name="lucide:chevron-right"
                  class="text-default-300 ml-auto size-4"
                />
              </NuxtLink>
            </nav>
          </section>

          <!-- ── Utility links ── -->
          <section v-if="utilityItems.length" class="space-y-1">
            <p
              class="text-default-400 px-3 text-xs font-semibold tracking-wider uppercase"
            >
              浏览与反馈
            </p>
            <nav class="flex flex-col gap-0.5">
              <NuxtLink
                v-for="item in utilityItems"
                :key="item.href"
                :to="item.href"
                class="hover:bg-default-100 active:bg-default-200 flex items-center gap-3 rounded-xl px-3 py-3.5 transition-colors"
                @click="closeMenu"
              >
                <KunIcon
                  :name="iconFor(item.href)"
                  class="text-default-500 size-5 shrink-0"
                />
                <span class="text-sm font-medium">{{ item.name }}</span>
                <KunIcon
                  name="lucide:chevron-right"
                  class="text-default-300 ml-auto size-4"
                />
              </NuxtLink>
            </nav>
          </section>

          <!-- ── Admin (gated) ── -->
          <section v-if="adminItems.length" class="space-y-1">
            <p
              class="text-default-400 px-3 text-xs font-semibold tracking-wider uppercase"
            >
              管理
            </p>
            <nav class="flex flex-col gap-0.5">
              <NuxtLink
                v-for="item in adminItems"
                :key="item.href"
                :to="item.href"
                class="hover:bg-warning/10 active:bg-warning/15 text-warning flex items-center gap-3 rounded-xl px-3 py-3.5 transition-colors"
                @click="closeMenu"
              >
                <KunIcon :name="iconFor(item.href)" class="size-5 shrink-0" />
                <span class="text-sm font-medium">{{ item.name }}</span>
                <KunIcon
                  name="lucide:chevron-right"
                  class="ml-auto size-4 opacity-60"
                />
              </NuxtLink>
            </nav>
          </section>

          <!-- ── Theme switcher ── -->
          <section class="space-y-3">
            <p
              class="text-default-400 px-3 text-xs font-semibold tracking-wider uppercase"
            >
              外观
            </p>
            <!-- 3-segmented control: each tile equally sized so it reads as
               a single grouped picker rather than three loose buttons.
               No menu close on tap — users typically tweak theme then keep
               browsing other menu items. -->
            <div
              class="border-default/20 bg-default-50/40 grid grid-cols-3 gap-1 rounded-xl border p-1"
            >
              <KunButton
                v-for="t in themes"
                :key="t.key"
                :variant="colorMode.preference === t.key ? 'flat' : 'light'"
                :color="colorMode.preference === t.key ? 'primary' : 'default'"
                size="sm"
                full-width
                rounded="lg"
                class-name="flex-col gap-1 py-3"
                :aria-label="`切换到${t.label}主题`"
                @click="setTheme(t.key)"
              >
                <KunIcon :name="t.icon" class="size-5" />
                <span class="text-xs">{{ t.label }}</span>
              </KunButton>
            </div>
          </section>

          <!-- ── Content / NSFW switcher ── -->
          <section class="space-y-3">
            <p
              class="text-default-400 px-3 text-xs font-semibold tracking-wider uppercase"
            >
              内容显示
            </p>
            <div
              class="border-default/20 bg-default-50/40 grid grid-cols-3 gap-1 rounded-xl border p-1"
            >
              <KunButton
                v-for="opt in nsfwOptions"
                :key="opt.key"
                :variant="
                  settingStore.data.kunNsfwEnable === opt.key ? 'flat' : 'light'
                "
                :color="
                  settingStore.data.kunNsfwEnable === opt.key
                    ? 'primary'
                    : 'default'
                "
                size="sm"
                full-width
                rounded="lg"
                class-name="flex-col gap-1 py-3"
                :aria-label="`切换内容模式: ${KUN_CONTENT_LIMIT_MAP[opt.key]}`"
                @click="setNsfw(opt.key)"
              >
                <KunIcon :name="opt.icon" class="size-5" />
                <span class="text-xs">{{ KUN_CONTENT_LIMIT_MAP[opt.key] }}</span>
              </KunButton>
            </div>
          </section>

          <p class="text-default-300 mt-auto pt-4 text-center text-xs">
            {{ kunMoyuMoe.titleShort }}
          </p>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
