<script setup lang="ts">
// Settings shell: a left tab nav (underline indicator) over the account /
// system sub-pages, which render through <NuxtPage/>. Nested route parent of
// pages/settings/{user,system,index}.vue.
const route = useRoute()

const tabs = [
  { to: '/settings/user', label: '账户设置', icon: 'lucide:user-cog' },
  { to: '/settings/system', label: '系统设置', icon: 'lucide:settings-2' }
]

const isActive = (to: string) => route.path === to
</script>

<template>
  <div class="container mx-auto my-4 px-4">
    <div class="flex flex-col gap-6 md:flex-row md:gap-8">
      <!-- Left tab nav. Underline indicator (border-b-2): on desktop the items
           stack vertically and only the active one shows the underline; on
           mobile it collapses to a horizontal scrollable tab bar. -->
      <nav
        class="border-default/15 flex shrink-0 gap-1 overflow-x-auto border-b md:w-48 md:flex-col md:gap-0 md:overflow-visible md:border-b-0"
      >
        <NuxtLink
          v-for="t in tabs"
          :key="t.to"
          :to="t.to"
          :class="
            cn(
              'flex items-center gap-2 whitespace-nowrap border-b-2 px-3 py-2.5 text-sm font-medium transition-colors',
              isActive(t.to)
                ? 'border-primary text-primary'
                : 'text-default-500 hover:text-default-800 border-transparent'
            )
          "
        >
          <KunIcon :name="t.icon" class="size-4" />
          {{ t.label }}
        </NuxtLink>
      </nav>

      <div class="min-w-0 flex-1">
        <NuxtPage />
      </div>
    </div>
  </div>
</template>
