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
    <NuxtLoadingIndicator color="var(--color-primary)" />
    <NuxtRouteAnnouncer />
    <KunAlertMessageContainer />
    <NuxtLayout>
      <NuxtPage />
    </NuxtLayout>
  </div>
</template>
