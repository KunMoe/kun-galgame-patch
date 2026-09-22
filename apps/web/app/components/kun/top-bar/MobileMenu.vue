<script setup lang="ts">
import { kunMoyuMoe } from '~/config/moyu-moe'
import {
  kunMobileAdminItem,
  kunMobileNavItem,
  KUN_CONTENT_LIMIT_RADIO_OPTIONS,
  KUN_THEME_OPTIONS
} from '~/constants/top-bar'
import { useBodyScrollLock } from '@kungal/ui-vue'

interface Props {
  isOpen: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{ 'update:isOpen': [value: boolean] }>()
const closeMenu = () => emit('update:isOpen', false)

const userStore = useUserStore()

const { theme, contentStance } = useKunDisplayPreference()

const ICON_BY_HREF: Record<string, string> = {
  '/galgame': 'lucide:gamepad-2',
  '/gallib': 'lucide:library-big',
  '/calendar': 'lucide:calendar-days',
  '/edit/create': 'lucide:plus-circle',
  '/ranking/user': 'lucide:chart-column-big',
  '/doc': 'lucide:book-open',
  '/comment': 'lucide:message-square',
  '/resource': 'lucide:puzzle',
  '/doc/notice/feedback': 'lucide:mail',
  '/admin': 'lucide:shield-check'
}
const iconFor = (href: string) => ICON_BY_HREF[href] ?? 'lucide:chevron-right'

const primaryItems = computed(() => kunMobileNavItem.slice(0, 4))
const utilityItems = computed(() => kunMobileNavItem.slice(4))
const adminItems = computed(() => (userStore.isAdmin ? kunMobileAdminItem : []))

const { open: openAuthModal } = useAuthModal()
const handleLoginClick = () => {
  closeMenu()
  openAuthModal()
}

const { openLogoutModal } = useLogoutModal()
const openLogout = () => {
  closeMenu()
  openLogoutModal()
}

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
  <Teleport to="body">
    <Transition
      enter-active-class="transition-all duration-200 ease-out"
      leave-active-class="transition-all duration-150 ease-in"
      enter-from-class="opacity-0 -translate-y-2"
      leave-to-class="opacity-0 -translate-y-2"
    >
      <div
        v-if="props.isOpen"
        class="bg-background/70 fixed inset-x-0 top-16 bottom-0 z-30 h-[calc(100dvh-4rem)] overflow-y-auto backdrop-blur-2xl backdrop-saturate-150 md:hidden"
        :style="{ paddingBottom: 'env(safe-area-inset-bottom)' }"
        @click.self="closeMenu"
      >
        <div class="mx-auto flex max-w-md flex-col gap-4 px-4 pt-5 pb-6">
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
              @click="openLogout"
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

          <NuxtLink
            to="/settings/user"
            class="hover:bg-default-100 active:bg-default-200 border-default/20 bg-default-50/40 flex items-center gap-3 rounded-2xl border px-3 py-3.5 transition-colors"
            @click="closeMenu"
          >
            <KunIcon
              name="lucide:settings"
              class="text-default-500 size-5 shrink-0"
            />
            <span class="text-sm font-medium">系统和用户设置</span>
            <KunIcon
              name="lucide:chevron-right"
              class="text-default-300 ml-auto size-4"
            />
          </NuxtLink>

          <section class="space-y-1">
            <p
              class="text-default-400 px-3 text-xs font-semibold tracking-wider uppercase"
            >
              主菜单
            </p>
            <nav class="flex flex-col gap-0.5">
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

          <section class="space-y-1">
            <p
              class="text-default-400 px-3 text-xs font-semibold tracking-wider uppercase"
            >
              外观与内容
            </p>
            <div
              class="flex flex-wrap items-center justify-between gap-2 px-3 py-2"
            >
              <p class="text-sm font-medium">主题</p>
              <KunRadioGroup
                v-model="theme"
                :options="KUN_THEME_OPTIONS"
                variant="pill"
                orientation="horizontal"
                size="sm"
                aria-label="主题"
                class-name="w-auto"
              />
            </div>
            <div
              class="flex flex-wrap items-center justify-between gap-2 px-3 py-2"
            >
              <p class="text-sm font-medium">内容显示</p>
              <KunRadioGroup
                v-model="contentStance"
                :options="KUN_CONTENT_LIMIT_RADIO_OPTIONS"
                variant="pill"
                orientation="horizontal"
                size="sm"
                aria-label="内容显示"
                class-name="w-auto"
              />
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
