<script setup lang="ts">
// Bare /settings has no content of its own — default to the account tab.
await navigateTo('/settings/user', { replace: true })
</script>

<template>
  <span />
</template>
