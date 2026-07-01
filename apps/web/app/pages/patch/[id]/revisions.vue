<script setup lang="ts">
// Galgame revision history (handbook §15). Lists Wiki revisions for this
// galgame (== patch.id, D13), shows a single-revision snapshot and the diff
// vs the previous revision, and lets the creator/admin revert. Authorization
// is enforced by Wiki — we just forward its code+message.
//
// docs/galgame_wiki/02-revisions-and-prs.md §版本历史.

import type {
  GalgameRevision,
  GalgameRevisionDetail,
  GalgameDiff
} from '~/composables/useGalgameEdit'
import type { KunUIColor } from '@kungal/ui-core'

const route = useRoute()
const gid = computed(() => Number(route.params.id))
const ge = useGalgameEdit()
const userStore = useUserStore()
const { requireLogin } = useAuthModal()

const page = ref(1)
const limit = 20
const includeMinor = ref(false)

const { data, pending, refresh } = await useAsyncData<{
  items: GalgameRevision[]
  total: number
}>(
  () => `gal-revisions-${gid.value}-${page.value}-${includeMinor.value}`,
  async () => {
    const res = await ge.listRevisions(gid.value, {
      page: page.value,
      limit,
      include_minor: includeMinor.value
    })
    if (res.code !== 0) return { items: [], total: 0 }
    return { items: res.data?.items ?? [], total: res.data?.total ?? 0 }
  },
  { default: () => ({ items: [], total: 0 }), watch: [page, includeMinor] }
)

const totalPage = computed(() =>
  Math.max(1, Math.ceil((data.value?.total ?? 0) / limit))
)

const ACTION_LABEL: Record<string, { text: string; color: KunUIColor }> = {
  created: { text: '创建', color: 'primary' },
  updated: { text: '更新', color: 'default' },
  merged: { text: 'PR 合并', color: 'success' },
  reverted: { text: '回滚', color: 'warning' },
  declined: { text: '拒绝', color: 'danger' }
}
const actionOf = (a: string): { text: string; color: KunUIColor } =>
  ACTION_LABEL[a] ?? { text: a, color: 'default' }

// ─── Snapshot / diff modal ────────────────────────────
const modalOpen = ref(false)
const modalMode = ref<'snapshot' | 'diff'>('diff')
const modalLoading = ref(false)
const activeRev = ref<number | null>(null)
const snapshot = ref<GalgameRevisionDetail | null>(null)
const diff = ref<GalgameDiff | null>(null)

const openSnapshot = async (rev: number) => {
  modalMode.value = 'snapshot'
  activeRev.value = rev
  modalOpen.value = true
  modalLoading.value = true
  snapshot.value = null
  const res = await ge.getRevision(gid.value, rev)
  modalLoading.value = false
  if (res.code === 0) snapshot.value = res.data
  else useKunMessage(res.message || '加载快照失败', 'error')
}

const openDiff = async (rev: number) => {
  modalMode.value = 'diff'
  activeRev.value = rev
  modalOpen.value = true
  modalLoading.value = true
  diff.value = null
  const res = await ge.getRevisionDiff(gid.value, rev)
  modalLoading.value = false
  if (res.code === 0) diff.value = res.data
  else useKunMessage(res.message || '加载 diff 失败', 'error')
}

// ─── Revert ───────────────────────────────────────────
const reverting = ref<number | null>(null)
const handleRevert = async (rev: number) => {
  if (!requireLogin()) return
  const ok = await useKunAlert({
    title: '回滚版本',
    message: `确定回滚到版本 #${rev}？这会创建一个新的回滚版本，不会删除历史。`
  })
  if (!ok) return
  reverting.value = rev
  try {
    const res = await ge.revert(gid.value, rev)
    if (res.code === 0) {
      useKunMessage('已回滚', 'success')
      page.value = 1
      await refresh()
    } else {
      useKunMessage(res.message || '回滚失败（仅创建者或版主可操作）', 'error')
    }
  } finally {
    reverting.value = null
  }
}

const snapshotEntries = computed(() =>
  snapshot.value
    ? Object.entries(snapshot.value.snapshot ?? {})
    : []
)
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <h2 class="text-lg font-semibold">编辑历史</h2>
      <label class="text-default-600 flex items-center gap-2 text-sm">
        <KunCheckBox v-model="includeMinor" />
        显示小修改
      </label>
    </div>

    <KunLoading v-if="pending" description="加载中..." />
    <KunNull
      v-else-if="!data?.items?.length"
      description="暂无编辑历史"
    />

    <div v-else class="space-y-2">
      <KunCard
        v-for="r in data.items"
        :key="r.id"
        :bordered="true"
      >
        <div
          class="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between"
        >
          <div class="min-w-0 space-y-1">
            <div class="flex flex-wrap items-center gap-2">
              <span class="font-mono text-sm font-semibold">
                #{{ r.revision }}
              </span>
              <KunChip :color="actionOf(r.action).color" size="sm">
                {{ actionOf(r.action).text }}
              </KunChip>
              <KunChip v-if="r.is_minor" color="default" size="sm">
                小修改
              </KunChip>
              <span
                v-if="r.reverted_to"
                class="text-default-500 text-xs"
              >
                → 回滚自 #{{ r.reverted_to }}
              </span>
            </div>
            <p v-if="r.note" class="text-default-700 text-sm">
              {{ r.note }}
            </p>
            <p class="text-default-500 text-xs">
              用户 #{{ r.user_id }} ·
              {{ r.created ? formatDate(r.created, { isPrecise: true, isShowYear: true }) : '' }}
            </p>
          </div>

          <div class="flex shrink-0 flex-wrap gap-2">
            <KunButton
              variant="light"
              size="sm"
              @click="openDiff(r.revision)"
            >
              查看 diff
            </KunButton>
            <KunButton
              variant="bordered"
              size="sm"
              @click="openSnapshot(r.revision)"
            >
              快照
            </KunButton>
            <KunButton
              variant="bordered"
              color="warning"
              size="sm"
              :loading="reverting === r.revision"
              :disabled="reverting !== null"
              @click="handleRevert(r.revision)"
            >
              回滚到此版本
            </KunButton>
          </div>
        </div>
      </KunCard>

      <KunPagination
        v-if="totalPage > 1"
        v-model:current-page="page"
        :total-page="totalPage"
        :is-loading="pending"
        class="mt-4"
      />
    </div>

    <KunModal v-model="modalOpen" size="xl" :is-show-close-button="true">
      <div class="max-h-[80vh] w-[92vw] max-w-2xl overflow-y-auto p-5">
        <h3 class="mb-4 text-lg font-semibold">
          版本 #{{ activeRev }} ·
          {{ modalMode === 'diff' ? '与上一版本的差异' : '完整快照' }}
        </h3>

        <KunLoading v-if="modalLoading" description="加载中..." />

        <GalgameEditDiffView
          v-else-if="modalMode === 'diff' && diff"
          :changed-keys="diff.changed_keys"
          :old-snap="diff.old"
          :new-snap="diff.new"
          :names="diff.names"
        />

        <div
          v-else-if="modalMode === 'snapshot' && snapshot"
          class="space-y-2"
        >
          <div
            v-for="[k, v] in snapshotEntries"
            :key="k"
            class="border-default/20 rounded-lg border p-2"
          >
            <p class="text-default-500 text-xs">{{ k }}</p>
            <pre
              class="text-default-700 text-xs break-words whitespace-pre-wrap"
              >{{
                Array.isArray(v) || (v && typeof v === 'object')
                  ? JSON.stringify(v)
                  : (v ?? '（空）')
              }}</pre
            >
          </div>
        </div>
      </div>
    </KunModal>
  </div>
</template>
