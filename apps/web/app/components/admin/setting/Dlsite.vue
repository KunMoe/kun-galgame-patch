<script setup lang="ts">
const api = useApi()
const { can } = usePermissions()

const form = reactive<AdminDlsiteSetting>({ link_template: '', coupon_url: '' })
const isSaving = ref(false)

const { pending } = await useAsyncData('admin-setting-dlsite', async () => {
  const res = await api.get<AdminDlsiteSetting>('/admin/setting/dlsite')
  if (res.code === 0 && res.data) Object.assign(form, res.data)
  return true
})

const save = async () => {
  isSaving.value = true
  const res = await api.put<AdminDlsiteSetting>('/admin/setting/dlsite', {
    ...form
  })
  isSaving.value = false
  if (res.code !== 0) {
    useKunMessage(res.message || '保存失败', 'error')
    return
  }
  if (res.data) Object.assign(form, res.data)
  useKunMessage('已保存', 'success')
}
</script>

<template>
  <KunCard :bordered="true" :is-hoverable="false" content-class="space-y-4">
    <div>
      <h3 class="text-lg font-medium">DLsite 正版购买入口</h3>
      <p class="text-default-500 text-sm">
        游戏页「正版购买」按钮。已铸造带归因短链的作品优先用短链,
        其余用联盟链接模板; 模板留空则这些作品不显示按钮。保存后即生效, 其他实例
        30 秒内跟上
      </p>
    </div>
    <KunLoading v-if="pending" description="加载中..." />
    <form v-else class="space-y-3" @submit.prevent="save">
      <KunInput
        v-model="form.link_template"
        label="联盟链接模板"
        placeholder="https://dlaf.jp/.../id/{workno}.html"
        description="必须是 https 链接, {workno} 会被替换成 DLsite 作品号"
        :disabled="!can('setting.manage')"
      />
      <KunInput
        v-model="form.coupon_url"
        label="常设优惠券页"
        placeholder="https://..."
        description="没有活动时展示; store 面报告有活动时会被活动的券链接盖过"
        :disabled="!can('setting.manage')"
      />
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
