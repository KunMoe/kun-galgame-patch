<script setup lang="ts">
// Settings shell: one page title, with the section Tab on its lower-left and the
// routed sub-page (账户 / 系统) on its lower-right; on mobile the Tab collapses
// to a horizontal bar above the content. Nested-route parent of
// pages/settings/{user,system,index}.vue.
//
// The Tab is a KunTab driven by the current route (not KunTabPanels) — the panel
// content is route-rendered via <NuxtPage/>. Two CSS-toggled instances
// (vertical desktop / horizontal mobile) instead of a JS media query, so SSR
// has no orientation flash.
const route = useRoute()

const tabs = [
  { value: 'user', textValue: '账户设置', icon: 'lucide:user-cog' },
  { value: 'system', textValue: '系统设置', icon: 'lucide:settings-2' }
]

const active = computed({
  get: () => (route.path === '/settings/system' ? 'system' : 'user'),
  set: (v: string) => {
    if (v && route.path !== `/settings/${v}`) navigateTo(`/settings/${v}`)
  }
})
</script>

<template>
  <div class="container mx-auto my-4 px-4">
    <KunHeader name="设置" description="管理您的账户与本地偏好" />

    <!-- Mobile: horizontal tabs above the content. -->
    <KunTab
      v-model="active"
      :items="tabs"
      variant="underlined"
      orientation="horizontal"
      name="settings-nav-mobile"
      class-name="mt-2 mb-4 md:hidden"
    />

    <div class="flex flex-col gap-6 md:flex-row md:gap-8">
      <!-- Desktop: vertical tabs on the lower-left. -->
      <KunTab
        v-model="active"
        :items="tabs"
        variant="underlined"
        orientation="vertical"
        name="settings-nav-desktop"
        class-name="hidden shrink-0 md:block md:w-48"
      />

      <div class="min-w-0 flex-1">
        <NuxtPage />
      </div>
    </div>
  </div>
</template>
