<script setup lang="ts">
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
         compiles: fall back to the raw `--primary-500` triplet. -->
    <NuxtLoadingIndicator color="var(--color-primary, hsl(var(--primary-500)))" />
    <NuxtRouteAnnouncer />
    <KunAlertMessageContainer />
    <NuxtLayout>
      <NuxtPage />
    </NuxtLayout>
  </div>
</template>
