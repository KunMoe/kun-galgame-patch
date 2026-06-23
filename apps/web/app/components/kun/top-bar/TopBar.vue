<script setup lang="ts">
import { kunNavItemDesktop, kunTopBarCategories } from '~/constants/top-bar'

const route = useRoute()
const isMenuOpen = ref(false)
const galgamePopover = ref<{ close: () => void } | null>(null)

watch(
  () => route.path,
  () => {
    isMenuOpen.value = false
    // Close the hover menu after a category click. NuxtLink navigates
    // client-side (no reload), so without this the menu would linger open
    // on the destination page until the pointer left it.
    galgamePopover.value?.close()
  }
)
</script>

<template>
  <nav
    class="bg-background/80 sticky top-0 z-40 flex h-16 w-full items-center px-3 backdrop-blur"
  >
    <div class="mx-auto flex w-full max-w-7xl items-center gap-3">
      <KunButton
        variant="light"
        color="default"
        size="sm"
        is-icon-only
        class-name="md:hidden"
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
        <!-- "下载补丁" hover menu — KunPopover trigger="hover" (kun-ui 2.1):
             coordinate safe-triangle (no pt-2 bridge hack), no focus steal on
             hover, touch→click. Both the trigger and the entries stay real
             <NuxtLink>s (middle-click / new-tab / SEO). Closed on route change
             via the watch above so a category click doesn't leave it lingering. -->
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
          </nav>
        </KunPopover>

        <NuxtLink
          v-for="item in kunNavItemDesktop"
          :key="item.href"
          :to="item.href"
          :class="
            cn(
              'shrink-0 text-base',
              route.path === item.href ? 'text-primary' : 'text-foreground'
            )
          "
        >
          {{ item.name }}
        </NuxtLink>

      </div>

      <!-- AIEro ad button. Pulled OUT of the `hidden md:flex` nav group above
           so it stays visible on mobile too (phones previously had no ad icon).
           The brand is itself `hidden md:flex`, so on mobile this sits next to
           the hamburger with the left side otherwise empty; on desktop it still
           trails the nav links. Non-moderators only (gated inside the
           component). -->
      <KunAdAIEroNav />

      <!-- KunTopBarUser already groups NSFW switcher + search + random +
           theme + bell + avatar (see User.vue). Don't add NSFWSwitcher here
           in parallel — that would render two copies on desktop. -->
      <KunTopBarUser />
    </div>

    <KunTopBarMobileMenu v-model:is-open="isMenuOpen" />
  </nav>
</template>
