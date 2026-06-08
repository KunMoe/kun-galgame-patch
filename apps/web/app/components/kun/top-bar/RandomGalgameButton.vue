<script setup lang="ts">
interface Props {
  isIconOnly?: boolean
  size?: 'xs' | 'sm' | 'md' | 'lg' | 'xl'
  // Mirrors @kungal/ui-core KunUIVariant (the old @kun/ui's extra 'faded' was
  // dropped upstream; it was never passed here — callers use 'light').
  variant?: 'solid' | 'bordered' | 'light' | 'flat' | 'shadow' | 'ghost'
  className?: string
}

const props = withDefaults(defineProps<Props>(), {
  isIconOnly: false,
  size: 'sm',
  variant: 'light',
  className: ''
})

const api = useApi()
const loading = ref(false)

const handleRandom = async () => {
  if (loading.value) return
  loading.value = true
  try {
    const res = await api.get<{ id: number | string }>('/home/random')
    if (res.code === 0 && res.data?.id) {
      await navigateTo(`/patch/${res.data.id}/introduction`)
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
    :is-icon-only="props.isIconOnly"
    :size="props.size"
    :variant="props.variant"
    color="default"
    :class-name="props.className"
    :loading="loading"
    aria-label="随机一部游戏"
    @click="handleRandom"
  >
    <KunIcon name="lucide:dices" class="text-default-500 size-6" />
    <template v-if="!props.isIconOnly"> 随机一部游戏 </template>
  </KunButton>
</template>
