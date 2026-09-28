<script setup lang="ts">
const { stance } = useKunNsfwStance()

const isSafeMode = computed(() => stance.value === 'hide')
</script>

<template>
  <KunBanner
    :model-value="isSafeMode"
    size="sm"
    color="danger"
    variant="flat"
    icon="lucide:shield-check"
    :closable="false"
    aria-label="全年龄模式提示"
    text="当前为全年龄模式，部分 R18 / NSFW 内容已隐藏"
  >
    <template #actions>
      <KunButton
        size="xs"
        variant="flat"
        color="danger"
        href="/settings/system"
      >
        切换显示方式
      </KunButton>
    </template>
  </KunBanner>
</template>
