<script setup lang="ts">
import { useWindowScroll } from '@vueuse/core'
import { kunNavItemDesktop, kunTopBarCategories } from '~/constants/top-bar'

const route = useRoute()
const isMenuOpen = ref(false)
const galgamePopover = ref<{ close: () => void } | null>(null)

const { y } = useWindowScroll()
const scrolled = computed(() => y.value > 8)

const { hasReleaseToday } = useGalgameReleaseToday()

watch(
  () => route.path,
  () => {
    isMenuOpen.value = false
    galgamePopover.value?.close()
  }
)
</script>

<template>
  <nav
    :class="
      cn(
        'sticky top-0 z-40 flex h-16 w-full items-center border-b px-3 transition-[background-color,border-color,box-shadow] duration-200',
        scrolled
          ? 'bg-background/80 border-default/40 shadow-kun-sm backdrop-blur'
          : 'border-transparent bg-transparent'
      )
    "
  >
    <div class="mx-auto flex w-full max-w-7xl items-center gap-3">
      <KunButton
        variant="light"
        color="default"
        size="sm"
        is-icon-only
        class-name="shrink-0 md:hidden"
        aria-label="菜单"
        @click="isMenuOpen = !isMenuOpen"
      >
        <KunIcon
          :name="isMenuOpen ? 'lucide:x' : 'lucide:menu'"
          class="size-5"
        />
      </KunButton>

      <KunTopBarBrand />

      <div class="hidden items-center gap-6 md:flex">
        <KunPopover
          ref="galgamePopover"
          trigger="hover"
          position="bottom-start"
          inner-class="min-w-44 p-1"
        >
          <template #trigger>
            <NuxtLink
              to="/galgame"
              :class="
                cn(
                  'shrink-0 text-base',
                  route.path === '/galgame' ? 'text-primary' : 'text-foreground'
                )
              "
            >
              下载补丁
            </NuxtLink>
          </template>

          <nav class="space-y-1">
            <NuxtLink
              v-for="it in kunTopBarCategories"
              :key="it.href"
              :to="it.href"
              class="text-default-700 hover:bg-default-100 flex items-center gap-3 rounded-lg px-3 py-2 text-sm"
            >
              <KunIcon :name="it.icon" class="text-default-600 size-4" />
              <span class="truncate">{{ it.label }}</span>
            </NuxtLink>
            <KunTopBarRandomGalgameButton />
          </nav>
        </KunPopover>

        <NuxtLink
          v-for="item in kunNavItemDesktop"
          :key="item.href"
          :to="item.href"
          :class="
            cn(
              'shrink-0 text-base',
              route.path === item.href ||
                (item.href === '/calendar' && hasReleaseToday)
                ? 'text-primary'
                : 'text-foreground'
            )
          "
        >
          {{
            item.href === '/calendar' && hasReleaseToday
              ? '有新作发售'
              : item.name
          }}
        </NuxtLink>
      </div>

      <KunAdAIEroNav />

      <KunTopBarUser />
    </div>

    <KunTopBarMobileMenu v-model:is-open="isMenuOpen" />
  </nav>
</template>
