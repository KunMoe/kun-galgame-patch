<script setup lang="ts">
// Patch header actions: favorite / share / edit / delete.
//
// Design: a single icon-only action bar wrapped in tooltips. Previously
// some actions carried a text label (收藏 / 编辑) and others didn't (分享 /
// 删除), and delete used a default-color border despite being destructive.
// This version standardises:
//   - all four buttons are icon-only, same `light` variant, same size
//   - tooltip carries the accessible label, so no per-button text noise
//   - favorite turns danger-red + filled heart only when active (color
//     signal for state)
//   - delete is always danger-red so the destructive intent is obvious
//   - a thin divider separates the read-only actions (favorite / share)
//     from the owner-side actions (edit / delete) without spending a
//     whole new row on grouping
//
// Endpoint contracts:
//   - favorite: PUT /patch/:id/favorite — local-only state, optimistic UI
//   - share:    copy direct URL to clipboard
//   - edit:     navigates to /edit/rewrite?id=:id (in-site form, proxies to
//               PUT /api/v1/galgame/:gid → Wiki Service per
//               integration-guide.md §6).
//   - delete:   DELETE /patch/:id — wipes the moyu patch row plus all child
//               tables (resources / comments / contributor / favorite / pr /
//               link / resource_file_history via FK CASCADE) and best-effort
//               purges the S3 objects snapshotted from patch_resource.s3_key.
//               Does NOT touch the wiki galgame entity — wiki considers
//               status=0 published rows un-deletable (only admin can flip
//               status to 1 via /admin/galgame/:gid/status). This button is
//               about removing moyu's local patch carrier, not the upstream
//               galgame metadata.
//
// Gating: backend enforces patch.user_id == caller OR role==admin
// (PatchService.DeletePatch). UI hides the button otherwise so it doesn't
// look interactive to viewers who'd just get a 400.

interface Props {
  patch: PatchHeader
}

const props = defineProps<Props>()

const config = useRuntimeConfig()
const wikiOrigin =
  ((config.public as { wikiOrigin?: string }).wikiOrigin as string) ??
  'https://wiki.kungal.com'

const userStore = useUserStore()
const api = useApi()
const { requireLogin } = useAuthModal()

const favorite = ref(props.patch.is_favorite)

// Keep local state in sync if the parent re-fetches (e.g. after a PR merge).
watch(
  () => props.patch.is_favorite,
  (v) => {
    favorite.value = v
  }
)

// KunReaction flips `favorite` optimistically on click, then fires this; confirm
// with the server and revert on failure / logged-out.
const onFavoriteChange = async (active: boolean) => {
  // Logged-out → pop the global login modal (same as the 登录 button) rather
  // than a toast the user can't act on.
  if (!requireLogin()) {
    favorite.value = !active
    return
  }
  const res = await api.put<{ favorited: boolean }>(
    `/patch/${props.patch.id}/favorite`
  )
  if (res.code === 0) {
    favorite.value = res.data.favorited
    useKunMessage(favorite.value ? '已收藏' : '已取消收藏', 'success')
  } else {
    favorite.value = !active
    useKunMessage(res.message || '操作失败', 'error')
  }
}

const handleShare = () => {
  const name = getPreferredLanguageText(props.patch.name)
  const link = `${name} - ${window.location.origin}/patch/${props.patch.id}/introduction`
  useKunCopy(link)
}

const editHref = computed(() => `/edit/rewrite?id=${props.patch.id}`)

const canDelete = computed(() => {
  if (!userStore.user.id) return false
  return userStore.isAdmin || props.patch.user?.id === userStore.user.id
})

const deleteOpen = ref(false)
const deleting = ref(false)

const askDelete = () => {
  deleteOpen.value = true
}

const confirmDelete = async () => {
  deleting.value = true
  try {
    const res = await api.delete(`/patch/${props.patch.id}`)
    if (res.code === 0) {
      useKunMessage('已删除游戏', 'success')
      deleteOpen.value = false
      await navigateTo('/')
    } else {
      useKunMessage(res.message || '删除失败', 'error')
    }
  } finally {
    deleting.value = false
  }
}
</script>

<template>
  <div
    class="flex flex-col items-start gap-3 sm:flex-row sm:items-center sm:justify-between"
  >
    <!-- Left: 资源更新时间 + the Wiki-maintained metadata note. Both moved here
         (资源更新于 was under the creator chip) and put to the LEFT of the action
         bar per the header layout request. -->
    <div class="text-default-500 flex flex-col gap-1 text-xs">
      <p>资源更新于 {{ formatDistanceToNow(props.patch.resource_update_time) }}</p>
      <p>
        游戏元数据由
        <a
          :href="wikiOrigin"
          target="_blank"
          rel="noopener noreferrer"
          class="text-primary hover:underline"
        >
          鲲 Galgame Wiki
        </a>
        统一维护
      </p>
    </div>

    <!-- Right: the icon action bar + a one-line hint that makes the heart's
         purpose (subscribe to new-patch notifications) discoverable without
         having to hover the icon-only button. -->
    <div class="flex flex-col items-start gap-2 sm:items-end">
      <!-- Action bar: favorite / share, divider, then owner edit / delete.
           All icon-only + tooltip — uniform shape. -->
      <div
        class="border-default/20 bg-default-50/50 flex items-center gap-1 rounded-xl border p-1"
      >
      <KunTooltip :text="favorite ? '取消收藏游戏' : '收藏游戏'">
        <KunReaction
          v-model="favorite"
          icon="lucide:star"
          color="warning"
          size="sm"
          label="收藏游戏"
          @change="onFavoriteChange"
        />
      </KunTooltip>

      <KunTooltip text="复制分享链接">
        <KunButton
          variant="light"
          color="default"
          size="sm"
          is-icon-only
          aria-label="复制分享链接"
          @click="handleShare"
        >
          <KunIcon name="lucide:share-2" class="size-4" />
        </KunButton>
      </KunTooltip>

      <div class="bg-default/30 mx-1 h-5 w-px" aria-hidden="true" />

      <KunTooltip text="编辑游戏信息">
        <NuxtLink :to="editHref" aria-label="编辑游戏信息">
          <KunButton
            variant="light"
            color="default"
            size="sm"
            is-icon-only
          >
            <KunIcon name="lucide:pencil" class="size-4" />
          </KunButton>
        </NuxtLink>
      </KunTooltip>

      <KunTooltip v-if="canDelete" text="删除游戏 (不可恢复)">
        <KunButton
          variant="light"
          color="danger"
          size="sm"
          is-icon-only
          aria-label="删除游戏"
          :disabled="deleting"
          @click="askDelete"
        >
          <KunIcon name="lucide:trash-2" class="size-4" />
        </KunButton>
      </KunTooltip>
      </div>

      <!-- New-patch notification hint — the icon-only heart can't say this on
           its own. Active (favorited) → primary + bell-ring to confirm you're
           subscribed; inactive → muted nudge. -->
      <p
        :class="
          cn(
            'flex items-center gap-1.5 text-xs',
            favorite ? 'text-primary' : 'text-default-500'
          )
        "
      >
        <KunIcon
          :name="favorite ? 'lucide:bell-ring' : 'lucide:bell'"
          class="size-3.5 shrink-0"
        />
        <span>{{
          favorite
            ? '已收藏，有新补丁时会通知你'
            : '收藏后，有新补丁时第一时间通知你'
        }}</span>
      </p>
    </div>
  </div>

  <!-- isDismissable=false: destructive irreversible action — backdrop click
       must not silently close the confirm. Force explicit 取消 / 删除. -->
  <KunModal
    v-model="deleteOpen"
    inner-class-name="max-w-md"
    :is-dismissable="false"
  >
    <div class="space-y-4 py-2">
      <h3 class="text-lg font-bold">删除该游戏？</h3>
      <p class="text-default-600 text-sm">
        此操作不可撤销。本站会删除该游戏的所有补丁资源、评论、贡献者记录、收藏关系，对应的 S3 文件也会被清理。
      </p>
      <p class="text-default-500 text-xs">
        这只会删除本站记录，不会影响 Galgame Wiki 上的游戏条目。
      </p>
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
          删除
        </KunButton>
      </div>
    </div>
  </KunModal>
</template>
