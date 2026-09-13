<script setup lang="ts">

import { kunMoyuMoe } from '~/config/moyu-moe'

interface Props {
  resource: PatchResource
  galgameName?: string
}

const props = defineProps<Props>()

const emit = defineEmits<{
  edited: [resource: PatchResource]
  refresh: []
}>()

const api = useApi()
const userStore = useUserStore()
const { requireLogin } = useAuthModal()
const { open: openReport } = useReportModal()

const canManage = computed(
  () =>
    userStore.isModerator ||
    (!!props.resource.user_id && props.resource.user_id === userStore.user.id)
)

const handleShare = () => {
  const name = props.resource.name || '补丁资源'
  const url = `${location.origin}/resource/${props.resource.id}`
  navigator.clipboard
    .writeText(`${props.galgameName ?? ''}${name}资源下载: ${url}`)
    .then(() => useKunMessage('链接复制成功', 'success'))
    .catch(() => useKunMessage('复制失败，请手动复制', 'error'))
}

const editOpen = ref(false)

const isDisabled = computed(() => (props.resource.status ?? 0) !== 0)

const toggling = ref(false)
const toggleDisable = async () => {
  toggling.value = true
  try {
    const res = await api.put<{ status: number }>(
      `/patch/resource/${props.resource.id}/disable`
    )
    if (res.code === 0) {
      useKunMessage(
        res.data.status !== 0 ? '已禁用该资源下载' : '已恢复该资源下载',
        'success'
      )
      emit('refresh')
    } else {
      useKunMessage(res.message || '操作失败', 'error')
    }
  } finally {
    toggling.value = false
  }
}

const reportResource = () => {
  if (!requireLogin()) return
  openReport({
    subjectKind: 'patch_resource',
    subjectId: props.resource.id,
    subjectUrl: `${kunMoyuMoe.domain.main}/resource/${props.resource.id}`,
    snapshot: props.resource.name
  })
}

const deleteOpen = ref(false)
const deleting = ref(false)
const deleteReason = ref('')

const isForeignDelete = computed(
  () => props.resource.user_id !== userStore.user.id
)

const askDelete = () => {
  deleteReason.value = ''
  deleteOpen.value = true
}

const confirmDelete = async () => {
  deleting.value = true
  try {
    const res = await api.delete(
      `/patch/resource/${props.resource.id}`,
      isForeignDelete.value ? { reason: deleteReason.value.trim() } : undefined
    )
    if (res.code === 0) {
      useKunMessage('已删除资源', 'success')
      deleteOpen.value = false
      await navigateTo(`/galgame/${props.resource.galgame_id}?tab=resource`)
    } else {
      useKunMessage(res.message || '删除失败', 'error')
    }
  } finally {
    deleting.value = false
  }
}

interface ResourceActionItem {
  key: 'edit' | 'disable' | 'share' | 'report' | 'delete'
  label: string
  icon: string
  color?: 'default' | 'success' | 'warning' | 'danger'
  disabled?: boolean
}

const menuItems = computed<ResourceActionItem[]>(() => {
  const items: ResourceActionItem[] = []
  if (canManage.value) {
    items.push({ key: 'edit', label: '编辑资源', icon: 'lucide:pencil' })
    items.push({
      key: 'disable',
      label: isDisabled.value ? '恢复下载' : '禁用下载',
      icon: isDisabled.value ? 'lucide:download' : 'lucide:ban',
      color: isDisabled.value ? 'success' : 'warning',
      disabled: toggling.value
    })
  }
  items.push({ key: 'share', label: '复制分享链接', icon: 'lucide:share-2' })
  if (props.resource.user_id !== userStore.user.id) {
    items.push({
      key: 'report',
      label: '举报资源',
      icon: 'lucide:flag',
      color: 'danger'
    })
  }
  if (canManage.value) {
    items.push({
      key: 'delete',
      label: '删除资源',
      icon: 'lucide:trash-2',
      color: 'danger'
    })
  }
  return items
})

const onMenuSelect = (item: { key: string }) => {
  switch (item.key) {
    case 'edit':
      editOpen.value = true
      break
    case 'disable':
      toggleDisable()
      break
    case 'share':
      handleShare()
      break
    case 'report':
      reportResource()
      break
    case 'delete':
      askDelete()
      break
  }
}
</script>

<template>
  <div class="shrink-0">
    <KunDropdown :items="menuItems" position="bottom-end" @select="onMenuSelect">
      <template #trigger>
        <KunButton
          is-icon-only
          variant="light"
          color="default"
          size="sm"
          rounded="full"
          aria-label="更多操作"
        >
          <KunIcon name="lucide:ellipsis-vertical" class="size-4" />
        </KunButton>
      </template>
    </KunDropdown>

    <KunModal
      v-model="editOpen"
      inner-class-name="max-w-3xl"
      :is-dismissable="false"
      aria-label="编辑补丁资源"
    >
      <ResourcePublish
        v-if="editOpen"
        :patch-id="props.resource.galgame_id"
        :resource="props.resource"
        @close="editOpen = false"
        @success="(updated) => emit('edited', updated)"
      />
    </KunModal>

    <KunModal
      v-model="deleteOpen"
      inner-class-name="max-w-md"
      :is-dismissable="false"
      aria-label="删除补丁资源"
    >
      <div class="space-y-4 py-2">
        <h3 class="text-lg font-bold">删除补丁资源？</h3>
        <p class="text-default-600 text-sm">
          此操作不可撤销。资源记录会从数据库移除，对应的下载文件也会一并删除。
        </p>
        <p v-if="props.resource.name" class="text-default-500 text-sm">
          <span class="text-default-400">资源名称：</span>
          <strong class="text-foreground">{{ props.resource.name }}</strong>
        </p>
        <div v-if="isForeignDelete" class="space-y-1">
          <label class="text-default-600 text-sm">
            删除原因（可选，会通知作者并记入管理日志）
          </label>
          <KunInput
            v-model="deleteReason"
            placeholder="例如：转载自付费站 / 违规内容 / 重复发布"
          />
        </div>
        <div class="flex justify-end gap-2">
          <KunButton
            variant="light"
            color="default"
            :disabled="deleting"
            @click="deleteOpen = false"
          >
            取消
          </KunButton>
          <KunButton
            color="danger"
            :loading="deleting"
            :disabled="deleting"
            @click="confirmDelete"
          >
            <KunIcon v-if="!deleting" name="lucide:trash-2" class="size-4" />
            确认删除
          </KunButton>
        </div>
      </div>
    </KunModal>
  </div>
</template>
