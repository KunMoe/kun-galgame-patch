<script setup lang="ts">
import { kunNavItemDesktop, kunTopBarCategories } from '~/constants/top-bar'

const route = useRoute()
const isMenuOpen = ref(false)
const isGalgameMenuOpen = ref(false)

watch(
  () => route.path,
  () => {
    isMenuOpen.value = false
    // Close the hover menu after a category click. NuxtLink navigates
    // client-side (no reload), so without this the menu would linger open
    // on the destination page until the pointer left it.
    isGalgameMenuOpen.value = false
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
        <!-- Hover-revealed "下载补丁" nav menu. Inlined on purpose, not built
             on a KunUI component: KunDropdown is click-only by design (its
             changelog deliberately omits hover as non-WAI-ARIA) and renders
             items as action <button>s, but this menu must reveal on hover and
             keep its trigger + entries as real <NuxtLink>s (middle-click /
             new-tab / SEO). Open-state is JS-controlled (not CSS :hover) so a
             category click can close it immediately via the route watch above;
             the menu's pt-2 is a hover bridge across the trigger→menu gap so
             the pointer can cross without mouseleave firing. -->
        <div
          class="relative"
          @mouseenter="isGalgameMenuOpen = true"
          @mouseleave="isGalgameMenuOpen = false"
        >
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

          <Transition
            enter-active-class="transition duration-150 ease-out"
            enter-from-class="opacity-0 scale-95"
            enter-to-class="opacity-100 scale-100"
            leave-active-class="transition duration-100 ease-in"
            leave-from-class="opacity-100 scale-100"
            leave-to-class="opacity-0 scale-95"
          >
            <div
              v-if="isGalgameMenuOpen"
              class="absolute top-full left-0 z-kun-popover origin-top pt-2"
            >
              <nav
                class="border-default-200 bg-background/95 min-w-44 space-y-1 rounded-xl border p-2 shadow-lg backdrop-blur"
              >
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
            </div>
          </Transition>
        </div>

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

      <!-- KunTopBarUser already groups NSFW switcher + search + random +
           theme + bell + avatar (see User.vue). Don't add NSFWSwitcher here
           in parallel — that would render two copies on desktop. -->
      <KunTopBarUser />
    </div>

    <KunTopBarMobileMenu v-model:is-open="isMenuOpen" />
  </nav>
</template>
