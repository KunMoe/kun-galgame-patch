<script setup lang="ts">
const { stance } = useKunNsfwStance()
const userStore = useUserStore()

const isSafeMode = computed(() => stance.value === 'hide')
const where = computed(() =>
  userStore.isLoggedIn ? '右上角的头像菜单中' : '右上角'
)
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
    :text="`当前为全年龄模式，部分 R18 / NSFW 内容已隐藏 — 您可在${where}切换为「模糊」或「直接显示」`"
  />
</template>
