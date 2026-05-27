<script setup lang="ts">
import { kunNavItemDesktop, kunTopBarCategories } from '~/constants/top-bar'

const route = useRoute()
const isMenuOpen = ref(false)

watch(
  () => route.path,
  () => {
    isMenuOpen.value = false
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
        <KunTooltip position="bottom">
          <template #content>
            <nav class="space-y-1 p-2">
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
          </template>

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
        </KunTooltip>

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
