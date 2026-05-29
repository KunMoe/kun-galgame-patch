<script setup lang="ts">
definePageMeta({
  // Profile home = the 动态 overview tab (matches where KunAvatar links).
  middleware: (to) =>
    navigateTo(`/user/${to.params.id}/info`, { redirectCode: 302 })
})
</script>

<template>
  <div />
</template>
