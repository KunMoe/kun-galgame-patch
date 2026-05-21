<script setup lang="ts">
import { ADMIN_MENU } from '~/constants/admin'

useKunSeoMeta({
  title: '管理面板',
  description: '鲲 Galgame 补丁 管理后台'
})

const route = useRoute()
// Writable computed for KunTab v-model — `set` is a no-op; KunTab.href
// already does the navigateTo() and the route change re-runs the getter.
const currentHref = computed({
  get: () => route.path,
  set: () => {}
})
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
            <NuxtLink
              v-for="item in ADMIN_MENU"
              :key="item.href"
              :to="item.href"
              :class="
                cn(
                  'flex items-center gap-2 rounded-lg px-3 py-2 text-sm transition-colors',
                  route.path === item.href
                    ? 'bg-primary text-primary-foreground'
                    : 'hover:bg-default-100'
                )
              "
            >
              <KunIcon :name="item.icon" class="size-4" />
              {{ item.name }}
            </NuxtLink>
          </nav>
        </KunCard>
      </aside>

      <div class="lg:col-span-4">
        <NuxtPage />
      </div>
    </div>
  </div>
</template>
