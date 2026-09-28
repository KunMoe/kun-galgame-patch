<script setup lang="ts">
const api = useApi()
const loading = ref(false)

const handleRandom = async () => {
  if (loading.value) return
  loading.value = true
  try {
    const res = await api.get<{ id: number | string }>('/home/random')
    if (res.code === 0 && res.data?.id) {
      await navigateTo(`/galgame/${res.data.id}`)
    } else {
      useKunMessage(res.message || '获取随机游戏失败', 'error')
    }
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <KunButton
    variant="light"
    color="default"
    size="sm"
    full-width
    rounded="lg"
    class-name="text-default-700 justify-start gap-3 px-3 font-normal"
    :loading="loading"
    @click="handleRandom"
  >
    <KunIcon name="lucide:dices" class="text-default-600 size-4" />
    随机一部游戏
  </KunButton>
</template>
