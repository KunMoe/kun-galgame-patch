<script setup lang="ts">
import { ADMIN_MENU } from '~/constants/admin'

// Admin shell + every nested admin/* page disables SEO — backend route
// is moderator-gated, and indexing internal counts / audit logs would
// be both useless and a data leak.
useKunDisableSeo('管理面板')

const route = useRoute()
const userStore = useUserStore()

// Frontend access gate for the whole /admin subtree: only moderators / admins
// (OAuth role "moderator" or "admin", i.e. legacy role > 2) may load the panel.
// This shell mounts for every /admin/* child, so the check covers all pages.
// It's defense-in-depth / UX only — the REAL gate is the backend (every /admin
// API runs moderatorAuth, validating roles from the OAuth JWT in the Redis
// session, not from this client-persisted store). isModerator is reliable
// during SSR because the user store is cookie-persisted (see User.vue).
if (!userStore.isModerator) {
  await navigateTo('/')
}

// adminOnly menu entries (e.g. 用户清除) hit admin-gated endpoints, so hide
// them from moderators — otherwise they'd open a page that only 403s.
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
            <NuxtLink
              v-for="item in visibleMenu"
              :key="item.href"
              :to="item.href"
              :class="
                cn(
                  'flex items-center gap-2 rounded-lg px-3 py-2 text-sm transition-colors',
                  route.path === item.href
                    ? 'bg-primary text-white'
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

      <!-- min-w-0: without it this grid cell defaults to min-width:auto and a
           wide child (e.g. the resource table, long content) forces the whole
           page past the viewport instead of letting the child's own
           overflow-x-auto / wrapping contain it. The mobile-overflow fix. -->
      <div class="min-w-0 lg:col-span-4">
        <NuxtPage />
      </div>
    </div>
  </div>
</template>
