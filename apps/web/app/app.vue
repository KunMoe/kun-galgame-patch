<script setup lang="ts">
// Central keepalive registry — ONE stable config applied to every route via
// <NuxtPage :keepalive>. We cache only list/feed pages so back-nav restores
// their page + scroll (each page opts in with a matching defineOptions name).
//
// Why central, not per-page definePageMeta: Nuxt's wrapInKeepAlive only mounts
// the <KeepAlive> wrapper when a route's keepalive config is truthy. Declaring
// it per-page makes the wrapper MOUNT on a kept route and UNMOUNT when you
// navigate to a non-kept page (e.g. a /patch/:id detail). Toggling <KeepAlive>
// while it holds an active cached child orphans that child's DOM, so the cached
// page (notably the /search box) ghosts onto the bottom of the next page. A
// permanent wrapper + name-based `include` never toggles → no ghost. Keep this
// list in sync with the pages' defineOptions({ name }). Stable const reference so
// KeepAlive never sees a changed include and never prunes the cache spuriously.
const kunKeepaliveConfig = {
  include: [
    'home-page',
    'galgame-list',
    'resource-list',
    'comment-feed',
    'search-page',
    'calendar-page',
    'tag-detail',
    'official-detail',
    'user-galgame',
    'user-resource',
    'user-favorite',
    'user-contribute',
    'user-comment'
  ]
}

onMounted(() => {
  if (process.env.NODE_ENV === 'development') {
    // Disable pinia console info for dev
    localStorage.setItem(
      '__VUE_DEVTOOLS_NEXT_PLUGIN_SETTINGS__dev.esm.pinia__',
      '{"logStoreChanges":false}'
    )
    // Disable umami for dev
    localStorage.setItem('umami.disabled', '1')
  }
})
</script>

<template>
  <div>
    <!-- Tailwind v4 inlines @theme colors into utilities but doesn't emit the
         bare `--color-primary` var to :root, so `var(--color-primary)` here is
         undefined → an invisible (transparent) bar. Mirror how `bg-primary`
         compiles: fall back to the raw `--primary-500` triplet — OKLCH since
         @kungal/ui-tokens 1.7.0 (was HSL), so wrap in oklch() not hsl(). -->
    <NuxtLoadingIndicator color="var(--color-primary, oklch(var(--primary-500)))" />
    <NuxtRouteAnnouncer />
    <!-- KunUI overlay hosts, mounted once (they Teleport to body). Required or
         useKunMessage() toasts and useKunAlert() confirm dialogs render nowhere
         — the npm @kungal/ui-* packages split the old single container into
         these two providers (see @kungal/ui-nuxt README). -->
    <KunMessageProvider />
    <KunAlertProvider />
    <NuxtLayout>
      <NuxtPage :keepalive="kunKeepaliveConfig" />
    </NuxtLayout>
  </div>
</template>
