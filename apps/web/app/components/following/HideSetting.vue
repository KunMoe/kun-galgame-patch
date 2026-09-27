<script setup lang="ts">
const api = useApi()

const hidden = ref(false)
const loaded = ref(false)
const loadFailed = ref(false)
const saving = ref(false)

onMounted(async () => {
  const res = await api.get<FollowingActivitySetting>(
    '/community/activity-settings'
  )
  if (res.code !== 0) {
    loadFailed.value = true
    return
  }
  hidden.value = res.data.hidden
  loaded.value = true
})

const toggle = async (next: boolean) => {
  const previous = hidden.value
  hidden.value = next
  saving.value = true
  const res = await api.put<FollowingActivitySetting>(
    '/community/activity-settings',
    { hidden: next }
  )
  saving.value = false
  if (res.code !== 0) {
    hidden.value = previous
    useKunMessage(res.message || '保存失败，请稍后再试', 'error')
    return
  }
  hidden.value = res.data.hidden
  useKunMessage(
    res.data.hidden ? '已隐藏您的动态' : '已恢复显示您的动态',
    'success'
  )
}
</script>

<template>
  <KunCard :bordered="true">
    <template #header>
      <h2 class="px-1 pt-1 text-xl font-medium">动态隐私</h2>
    </template>
    <KunSwitch
      :model-value="hidden"
      :disabled="!loaded || saving"
      label="隐藏我的动态"
      description="开启后，您在 NextMoe 各站的发布不会出现在任何人的「关注动态」里，关注您的人也不会再收到您的新发布通知，已发出的相关通知会被撤回，之后关闭也不会恢复。您的个人主页不受影响。"
      :error="loadFailed ? '设置读取失败，请刷新页面后重试' : ''"
      @update:model-value="toggle"
    />
  </KunCard>
</template>
