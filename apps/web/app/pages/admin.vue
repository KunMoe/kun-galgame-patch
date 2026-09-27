<script setup lang="ts">
import { ADMIN_MENU } from '~/constants/admin'

useKunDisableSeo('管理面板')

const route = useRoute()
const userStore = useUserStore()

if (!userStore.isModerator) {
  await navigateTo('/')
}

const visibleMenu = computed(() =>
  ADMIN_MENU.filter((item) => !item.adminOnly || userStore.isAdmin)
)
</script>

<template>
  <div class="container mx-auto my-4">
    <div class="grid gap-4 lg:grid-cols-5">
      <aside class="lg:col-span-1">
        <KunCard :bordered="true">
          <NuxtLink
            to="/admin"
            class="hover:text-primary mb-2 block text-xl font-bold"
          >
            管理面板
          </NuxtLink>
          <nav class="flex flex-col gap-1">
            <KunNavItem
              v-for="item in visibleMenu"
              :key="item.href"
              :href="item.href"
              :label="item.name"
              :icon="item.icon"
              :current="route.path === item.href"
            />
          </nav>
        </KunCard>
      </aside>

      <div class="min-w-0 lg:col-span-4">
        <NuxtPage />
      </div>
    </div>
  </div>
</template>
