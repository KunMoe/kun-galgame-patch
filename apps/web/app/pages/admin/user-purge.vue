<script setup lang="ts">
useKunDisableSeo('用户清除')

const userStore = useUserStore()
if (!userStore.isAdmin) {
  await navigateTo('/admin')
}

const api = useApi()

interface UserPurgePreview {
  user_id: number
  user_exists: boolean
  comments: number
  resources: number
  comment_likes: number
  resource_likes: number
  contributes: number
  following: number
  followers: number
  chat_memberships: number
  chat_messages: number
  private_messages: number
  owned_patches: number
  owned_patch_resources: number
  owned_patch_comments: number
  misc_traces: number
  catalog_folders: number
  catalog_folder_items: number
  catalog_folder_error?: string
  can_delete_user_row: boolean
}
interface UserPurgeResult {
  user_id: number
  user_row_deleted: boolean
  sessions_revoked: number
}

const uid = ref('')
const uidNum = computed(() => Number(uid.value))
const uidValid = computed(
  () => Number.isInteger(uidNum.value) && uidNum.value > 0
)
const forcePurgePatches = ref(false)
const preview = ref<UserPurgePreview | null>(null)
const previewing = ref(false)
const executing = ref(false)

watch(uid, () => {
  preview.value = null
})

const loadPreview = async () => {
  if (!uidValid.value) {
    useKunMessage('请输入有效的用户 ID', 'warn')
    return
  }
  previewing.value = true
  try {
    const res = await api.get<UserPurgePreview>(
      `/admin/user/${uidNum.value}/purge-preview?purge_owned_patches=${forcePurgePatches.value}`
    )
    if (res.code === 0) {
      preview.value = res.data
      if (!res.data.user_exists) {
        useKunMessage('该用户在本地不存在（可能已被清除）', 'warn')
      }
    } else {
      preview.value = null
      useKunMessage(res.message || '预览失败', 'error')
    }
  } finally {
    previewing.value = false
  }
}

watch(forcePurgePatches, () => {
  if (preview.value) loadPreview()
})

const rows = computed<{ label: string; value: number; hint?: string }[]>(() => {
  const p = preview.value
  if (!p) return []
  return [
    { label: '评论', value: p.comments },
    { label: '补丁资源', value: p.resources },
    { label: '点赞 (评论 / 资源)', value: p.comment_likes + p.resource_likes },
    { label: '贡献', value: p.contributes },
    { label: '关注 / 粉丝', value: p.following + p.followers },
    { label: '聊天室成员 / 消息', value: p.chat_memberships + p.chat_messages },
    { label: '站内私信', value: p.private_messages },
    { label: '其它 (阅读状态 / 文件历史)', value: p.misc_traces },
    { label: '本人创建的补丁', value: p.owned_patches }
  ]
})

const canExecute = computed(
  () =>
    !!preview.value &&
    preview.value.user_exists &&
    preview.value.can_delete_user_row &&
    !executing.value
)

const execute = async () => {
  const p = preview.value
  if (!p || !uidValid.value) return
  const collateral = forcePurgePatches.value
    ? `并强删其创建的 ${p.owned_patches} 个补丁（连带 ${p.owned_patch_resources} 个资源、${p.owned_patch_comments} 条评论，含其他用户的内容）。`
    : ''
  const ok = await useKunAlert({
    title: '⚠️ 清除用户全部痕迹',
    type: 'danger',
    message:
      `将【不可恢复地】删除用户 #${uidNum.value} 的本地账号，及其全部评论 (${p.comments})、` +
      `补丁资源 (${p.resources})、点赞 / 关注、` +
      `聊天与站内私信。${collateral}\n\n` +
      `（OAuth 身份、资料库、kungal、image_service 不受影响——如需封禁请另在 OAuth 后台操作。` +
      `收藏夹属于中央账号、与 kungal 共用同一份，本操作不会删除。）\n\n确定继续？`
  })
  if (!ok) return

  executing.value = true
  try {
    const res = await api.post<UserPurgeResult>(`/admin/user/${uidNum.value}/purge`, {
      purge_owned_patches: forcePurgePatches.value
    })
    if (res.code === 0) {
      const r = res.data
      useKunMessage(
        `清除完成：账号已删除，撤销登录会话 ${r.sessions_revoked} 个`,
        'success'
      )
      preview.value = null
      uid.value = ''
      forcePurgePatches.value = false
    } else {
      useKunMessage(res.message || '清除失败', 'error')
    }
  } finally {
    executing.value = false
  }
}
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-bold">用户清除</h1>
      <p class="text-default-500 mt-1 text-sm">
        清除某个用户在本站 (moyu) 的全部痕迹：评论、补丁资源 (含云端文件)、点赞 /
        关注、聊天与私信，以及本地账号本身。常用于处理脚本恶意刷 spam
        的账号。<strong class="text-danger">操作不可恢复</strong>，请先预览。
        收藏夹存在 catalog、属于中央账号，本操作不涉及。
      </p>
    </div>

    <KunCard :bordered="true">
      <div class="flex flex-wrap items-end gap-3 p-1">
        <label class="block">
          <span class="text-default-700 text-sm">用户 ID</span>
          <KunInput
            v-model="uid"
            type="number"
            placeholder="输入要清除的用户 ID"
          />
        </label>
        <KunButton
          color="primary"
          variant="flat"
          :loading="previewing"
          @click="loadPreview"
        >
          <KunIcon name="lucide:search" class="size-4" />
          预览
        </KunButton>
      </div>
    </KunCard>

    <KunCard v-if="preview" :bordered="true">
      <div class="space-y-4 p-1">
        <h2 class="text-lg font-semibold">
          将删除的内容
          <span class="text-default-400 text-sm font-normal">
            （用户 #{{ preview.user_id }}）
          </span>
        </h2>

        <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
          <div
            v-for="row in rows"
            :key="row.label"
            class="bg-default-50 flex items-baseline justify-between rounded-lg px-3 py-2"
          >
            <span class="text-default-600 text-sm">
              {{ row.label }}
              <span v-if="row.hint" class="text-default-400 text-xs">
                · {{ row.hint }}
              </span>
            </span>
            <span class="text-lg font-bold">{{ row.value }}</span>
          </div>
        </div>

        <div class="border-default-200 space-y-1 rounded-lg border p-3">
          <p class="text-default-600 text-sm">
            catalog 收藏夹
            <span class="text-default-400 text-xs">· 本操作不删除</span>
          </p>
          <p v-if="preview.catalog_folder_error" class="text-warning text-xs">
            {{ preview.catalog_folder_error }}
          </p>
          <p v-else class="text-default-500 text-xs">
            该账号有 {{ preview.catalog_folders }} 个收藏夹、共
            {{ preview.catalog_folder_items }} 个游戏。收藏夹属于中央账号，与 kungal
            共用同一份，删除它们要在 catalog 侧单独操作。
          </p>
        </div>

        <div class="border-default-200 space-y-3 rounded-lg border p-3">
          <KunCheckBox v-model="forcePurgePatches" color="danger">
            强删该用户创建的补丁 (连带其下全部资源 / 评论，含其他用户的内容)
          </KunCheckBox>

          <p
            v-if="preview.owned_patches > 0 && forcePurgePatches"
            class="text-danger text-xs"
          >
            将额外删除 {{ preview.owned_patch_resources }} 个资源与
            {{ preview.owned_patch_comments }} 条评论 —— 其中可能包含其他用户的内容。
          </p>
          <p
            v-else-if="preview.owned_patches > 0 && !forcePurgePatches"
            class="text-warning text-xs"
          >
            该用户创建了 {{ preview.owned_patches }} 个补丁，必须勾选上方选项才能删除其账号
            (否则数据库外键会阻止删除)。
          </p>
        </div>

        <div class="flex items-center justify-end gap-3">
          <span v-if="!preview.user_exists" class="text-warning text-sm">
            本地无此用户
          </span>
          <KunButton
            color="danger"
            :loading="executing"
            :disabled="!canExecute"
            @click="execute"
          >
            <KunIcon name="lucide:trash-2" class="size-4" />
            执行清除
          </KunButton>
        </div>
      </div>
    </KunCard>
  </div>
</template>
