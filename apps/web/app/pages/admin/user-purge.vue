<script setup lang="ts">
useKunDisableSeo('用户清除')

// Stricter than the /admin shell (which admits moderators): this page's
// endpoints are admin-only (adminAuth) server-side, so gate the page on
// isAdmin too — a moderator URL-typing here would otherwise hit only 403s.
const userStore = useUserStore()
if (!userStore.isAdmin) {
  await navigateTo('/admin')
}

const api = useApi()

// Mirrors apps/api/internal/admin/dto.UserPurgePreview / UserPurgeResult.
interface UserPurgePreview {
  user_id: number
  user_exists: boolean
  comments: number
  resources: number
  comment_likes: number
  resource_likes: number
  favorites: number
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
  can_delete_user_row: boolean
}
interface UserPurgeResult {
  user_id: number
  user_row_deleted: boolean
  sessions_revoked: number
}

// KunInput's v-model is string|number and writes the raw input string back, so
// keep the bound ref a string and derive the numeric id from it.
const uid = ref('')
const uidNum = computed(() => Number(uid.value))
const uidValid = computed(
  () => Number.isInteger(uidNum.value) && uidNum.value > 0
)
// Force-delete the user's own patches (galgame entries) + everything beneath
// them. Required to delete the account when the user owns any patch
// (patch.user_id is ON DELETE RESTRICT).
const forcePurgePatches = ref(false)
const preview = ref<UserPurgePreview | null>(null)
const previewing = ref(false)
const executing = ref(false)

// A stale preview must never gate a destructive execute: clear it whenever the
// target id changes so the admin has to re-preview the new id.
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

// Toggling the force flag changes the collateral counts + can_delete_user_row,
// so re-preview (only when a preview for the current id is already shown).
watch(forcePurgePatches, () => {
  if (preview.value) loadPreview()
})

// Primary breakdown rows (always purged). Computed off the loaded preview.
const rows = computed<{ label: string; value: number; hint?: string }[]>(() => {
  const p = preview.value
  if (!p) return []
  return [
    { label: '评论', value: p.comments },
    { label: '补丁资源', value: p.resources },
    { label: '点赞 (评论 / 资源)', value: p.comment_likes + p.resource_likes },
    { label: '收藏', value: p.favorites },
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
      `补丁资源 (${p.resources})、点赞 / 收藏 / 关注、` +
      `聊天与站内私信。${collateral}\n\n` +
      `（OAuth 身份、资料库、kungal、image_service 不受影响——如需封禁请另在 OAuth 后台操作。）\n\n确定继续？`
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
        收藏 / 关注、聊天与私信，以及本地账号本身。常用于处理脚本恶意刷 spam
        的账号。<strong class="text-danger">操作不可恢复</strong>，请先预览。
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

        <div class="border-default-200 space-y-3 rounded-lg border p-3">
          <KunCheckBox v-model="forcePurgePatches" color="danger">
            强删该用户创建的补丁 (连带其下全部资源 / 评论，含其他用户的内容)
          </KunCheckBox>

          <!-- When the user owns patches, the account row can't be deleted
               unless the patches go too (patch.user_id RESTRICT). Surface the
               requirement instead of letting the execute silently 400. -->
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
