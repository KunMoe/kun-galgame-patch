<script setup lang="ts">
const api = useApi()

const isPublic = ref(true)
const loaded = ref(false)
const loadFailed = ref(false)
const saving = ref(false)

onMounted(async () => {
  const res = await api.get<PlayRecordsPrivacy>('/user/privacy')
  if (res.code !== 0) {
    loadFailed.value = true
    return
  }
  isPublic.value = res.data.play_records === 'public'
  loaded.value = true
})

const toggle = async (next: boolean) => {
  const previous = isPublic.value
  isPublic.value = next
  saving.value = true
  const res = await api.patch<PlayRecordsPrivacy>('/user/privacy', {
    play_records: next ? 'public' : 'private'
  })
  saving.value = false
  if (res.code !== 0) {
    isPublic.value = previous
    useKunMessage(res.message || '保存失败，请稍后再试', 'error')
    return
  }
  isPublic.value = res.data.play_records === 'public'
  useKunMessage(
    isPublic.value ? '已公开您的游玩记录' : '您的游玩记录已改为仅自己可见',
    'success'
  )
}
</script>

<template>
  <KunCard :bordered="true">
    <template #header>
      <h2 class="px-1 pt-1 text-xl font-medium">游戏库隐私</h2>
    </template>
    <KunSwitch
      :model-value="isPublic"
      :disabled="!loaded || saving"
      label="公开我的游玩记录（状态与时长）"
      description="开启时，任何人都能在您个人主页的「游戏库」里看到您标记的游玩状态和游玩时长，NextMoe 各站共用这一个开关。关闭后只有您自己能看到。评分始终公开，不受这个开关影响。"
      :error="loadFailed ? '设置读取失败，请刷新页面后重试' : ''"
      @update:model-value="toggle"
    />
  </KunCard>
</template>
