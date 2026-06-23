<script setup lang="ts">
// "显示设置" entry on /galgame, sibling to the 高级筛选 button. The controls
// themselves moved to /settings/system → "Galgame 卡片显示设置"; this is now a
// link that deep-jumps to that card (system.vue scrolls to #galgame-display).
</script>

<template>
  <NuxtLink
    to="/settings/system#galgame-display"
    class="text-default-500 hover:text-primary flex cursor-pointer items-center gap-1 rounded-md px-2 py-1 text-sm transition-colors"
  >
    <KunIcon name="lucide:eye" class="text-inherit" />
    <span>显示设置</span>
  </NuxtLink>
</template>
