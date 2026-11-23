<script setup lang="ts">
type TierRole = keyof AdminUploadTiers['tiers']

const api = useApi()
const { can } = usePermissions()

const ROLES: { key: TierRole; label: string }[] = [
  { key: 'user', label: '普通用户' },
  { key: 'creator', label: '创作者' },
  { key: 'moderator', label: '版主' },
  { key: 'admin', label: '管理员 / 莲' }
]

const form = reactive<Record<TierRole, { maxFile: string; daily: string }>>({
  user: { maxFile: '', daily: '' },
  creator: { maxFile: '', daily: '' },
  moderator: { maxFile: '', daily: '' },
  admin: { maxFile: '', daily: '' }
})
const cap = ref(20)
const isSaving = ref(false)

const fill = (data: AdminUploadTiers) => {
  cap.value = data.max_file_cap_gib
  for (const { key } of ROLES) {
    form[key].maxFile = String(data.tiers[key].max_file_gib)
    form[key].daily = String(data.tiers[key].daily_gib)
  }
}

const { pending } = await useAsyncData('admin-setting-upload', async () => {
  const res = await api.get<AdminUploadTiers>('/admin/setting/upload')
  if (res.code === 0 && res.data) fill(res.data)
  return true
})

const save = async () => {
  const tiers = Object.fromEntries(
    ROLES.map(({ key }) => [
      key,
      {
        max_file_gib: Number(form[key].maxFile),
        daily_gib: Number(form[key].daily)
      }
    ])
  )
  isSaving.value = true
  const res = await api.put<AdminUploadTiers>('/admin/setting/upload', {
    tiers
  })
  isSaving.value = false
  if (res.code !== 0) {
    useKunMessage(res.message || '保存失败', 'error')
    return
  }
  if (res.data) fill(res.data)
  useKunMessage('已保存', 'success')
}
</script>

<template>
  <KunCard :bordered="true" :is-hoverable="false" content-class="space-y-4">
    <div>
      <h3 class="text-lg font-medium">补丁上传限额</h3>
      <p class="text-default-500 text-sm">
        按角色设置单文件上限与每日上限 (GB)。每日上限填 0
        表示不限。单文件上限不能超过文件服务对本站的上限
        {{ cap }} GB。保存后即生效, 其他实例 30 秒内跟上;
        文件服务另有全站每日总量上限
      </p>
    </div>
    <KunLoading v-if="pending" description="加载中..." />
    <form v-else class="space-y-3" @submit.prevent="save">
      <div
        v-for="role in ROLES"
        :key="role.key"
        class="grid items-end gap-3 sm:grid-cols-[8rem_1fr_1fr]"
      >
        <span class="text-sm font-medium sm:pb-2">{{ role.label }}</span>
        <KunInput
          v-model="form[role.key].maxFile"
          type="number"
          label="单文件上限 (GB)"
          :disabled="!can('setting.manage')"
        />
        <KunInput
          v-model="form[role.key].daily"
          type="number"
          label="每日上限 (GB, 0 = 不限)"
          :disabled="!can('setting.manage')"
        />
      </div>
      <div class="flex justify-end">
        <KunButton
          type="submit"
          :loading="isSaving"
          :disabled="!can('setting.manage')"
        >
          保存
        </KunButton>
      </div>
    </form>
  </KunCard>
</template>
