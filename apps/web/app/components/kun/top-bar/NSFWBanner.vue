<script setup lang="ts">
const { stance } = useKunNsfwStance()

const isSafeMode = computed(() => stance.value === 'hide')
</script>

<template>
  <div
    v-if="isSafeMode"
    class="bg-danger/10 text-danger border-danger/20 w-full border-b px-3 py-1.5 text-center text-xs"
    role="status"
    aria-live="polite"
  >
    <KunIcon
      name="lucide:shield-check"
      class="mr-1 inline size-3.5 align-text-bottom"
    />
    当前为全年龄模式，部分 R18 / NSFW 内容已隐藏 — 您可在右上角切换为 "模糊" 或
    "直接显示"
  </div>
</template>
