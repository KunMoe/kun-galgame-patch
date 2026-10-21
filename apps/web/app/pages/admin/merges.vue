<script setup lang="ts">
import {
  MERGE_ACTION_LABEL,
  MERGE_TABLE_LABEL,
  mergeTableCounts
} from '~/constants/catalogMerge'

useKunDisableSeo('合并记录 - 管理面板')

const api = useApi()
const page = ref(1)
const limit = 30
const issuesOnly = ref(false)

const { data, pending, refresh } = await useAsyncData<{
  items: AdminMergeRecord[]
  total: number
}>(
  'admin-catalog-merges',
  async () => {
    const res = await api.get<{ items: AdminMergeRecord[]; total: number }>(
      `/admin/catalog-merges?page=${page.value}&limit=${limit}&issues=${issuesOnly.value}`
    )
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  { default: () => ({ items: [], total: 0 }) }
)

watch(page, () => refresh())
watch(issuesOnly, () => {
  page.value = 1
  refresh()
})

const totalPages = computed(() => Math.ceil((data.value?.total ?? 0) / limit))

const hasIssues = (r: AdminMergeRecord) =>
  !!r.undo_report?.conflicts || !!r.undo_report?.notes?.length

const stateOf = (r: AdminMergeRecord) => {
  if (!r.undone_at) return { text: '生效中', color: 'primary' as const }
  if (hasIssues(r)) return { text: '已撤销，有遗留', color: 'danger' as const }
  return { text: '已撤销', color: 'success' as const }
}
</script>

<template>
  <div class="space-y-6">
    <div class="space-y-2">
      <h1 class="text-2xl font-bold">合并记录</h1>
      <p class="text-default-500 text-sm">
        鲲 Galgame
        论坛数据合并作品时，本站搬动的每一条数据都记在这里；合并被撤销时按记录搬回。
        搬不回的会在「有遗留」里列出，需要人工核对。
      </p>
      <div class="flex items-center gap-2 text-sm">
        <KunSwitch v-model="issuesOnly" />
        <span>只看有遗留的撤销</span>
      </div>
    </div>

    <KunLoading v-if="pending" description="加载中..." />
    <div v-else class="space-y-2">
      <div
        v-for="r in data?.items"
        :key="r.id"
        class="border-default/20 bg-content1 shadow-kun-sm space-y-2 rounded-lg border p-3"
      >
        <div class="flex flex-wrap items-center gap-2 text-sm">
          <KunLink :to="`/galgame/${r.old_id}`">#{{ r.old_id }}</KunLink>
          <span class="text-default-500">并入</span>
          <KunLink :to="`/galgame/${r.new_id}`">#{{ r.new_id }}</KunLink>
          <KunChip size="sm" variant="flat" color="default">
            {{ MERGE_ACTION_LABEL[r.action] ?? r.action }}
          </KunChip>
          <KunChip size="sm" variant="flat" :color="stateOf(r).color">
            {{ stateOf(r).text }}
          </KunChip>
          <span class="text-default-500 text-xs">
            {{
              formatDate(r.applied_at, { isShowYear: true, isPrecise: true })
            }}
            · 记录 {{ r.rows }} 行
          </span>
        </div>

        <div
          v-if="r.undo_report"
          class="text-default-600 space-y-1 text-xs leading-relaxed"
        >
          <p v-if="r.undone_at">
            撤销于
            {{ formatDate(r.undone_at, { isShowYear: true, isPrecise: true }) }}
          </p>
          <p v-if="r.undo_report.moved_back">
            已搬回：{{ mergeTableCounts(r.undo_report.moved_back) }}
          </p>
          <p v-if="r.undo_report.reinserted">
            已补回：{{ mergeTableCounts(r.undo_report.reinserted) }}
          </p>
          <p v-if="r.undo_report.kept_on_survivor">
            合并后发在存续方、留在存续方：{{
              mergeTableCounts(r.undo_report.kept_on_survivor)
            }}
          </p>
          <p v-if="r.undo_report.deleted_since">
            合并后已被删除、不再恢复：{{
              mergeTableCounts(r.undo_report.deleted_since)
            }}
          </p>
          <p
            v-for="(ids, table) in r.undo_report.conflicts"
            :key="table"
            class="text-danger"
          >
            没能搬回的{{ MERGE_TABLE_LABEL[table] ?? table }}：#{{
              ids.join('、#')
            }}
          </p>
          <p
            v-for="(note, i) in r.undo_report.notes"
            :key="i"
            class="text-danger"
          >
            {{ note }}
          </p>
        </div>
      </div>
    </div>

    <KunNull
      v-if="!pending && !data?.items?.length"
      :description="issuesOnly ? '没有留下遗留的撤销' : '暂无合并记录'"
    />

    <div v-if="totalPages > 1" class="flex justify-center">
      <KunPagination
        :current-page="page"
        :total-page="totalPages"
        :is-loading="pending"
        @update:current-page="(v) => (page = v)"
      />
    </div>
  </div>
</template>
