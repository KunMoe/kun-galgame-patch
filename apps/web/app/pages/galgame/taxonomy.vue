<script setup lang="ts">
// Galgame taxonomy management (handbook §15.1: moyu MUST implement the full
// tag/official/engine/series CRUD UI — create / edit / delete — not a subset).
//
// Delete follows the doc's mandated two-step UX (04-taxonomy.md, 00 §15.1):
// try a plain DELETE first; if Wiki rejects with code:7 (still referenced by
// N galgames) confirm again and retry with ?force=true. Wiki owns auth
// (create = any logged-in user; PUT/DELETE = admin/moderator, role>1) — we
// forward its code+message; non-privileged users just see Wiki's 403.

import type {
  WikiTag,
  WikiOfficial,
  WikiEngine,
  WikiSeries,
  TaxonomyRevision
} from '~/composables/useGalgameEdit'
import type { KunUIColor } from '@kungal/ui-core'

// Admin/editor-only write surface (wiki §15.1: PUT/DELETE require
// role > 1). No public content — disable SEO.
useKunDisableSeo('Galgame 分类管理')

const route = useRoute()
const ge = useGalgameEdit()

// Unauthed users see the login modal in place (via <AuthRequired> in the
// template), not a redirect to home — see edit/create.vue for the reasoning.

type Kind = 'tag' | 'official' | 'engine' | 'series'
const tab = ref<Kind>('tag')
const TABS: { key: Kind; title: string }[] = [
  { key: 'tag', title: '标签' },
  { key: 'official', title: '开发商' },
  { key: 'engine', title: '引擎' },
  { key: 'series', title: '系列' }
]

// ─── tag / official / engine share one shape ──────────
type Row = { id: number; name: string; category?: string; description?: string }
const keyword = ref('')
const rows = ref<Row[]>([])
const loading = ref(false)

const loadList = async () => {
  loading.value = true
  try {
    if (tab.value === 'tag') {
      const r = await ge.tagSearch(keyword.value, undefined, 50)
      rows.value =
        r.code === 0
          ? (r.data?.items ?? []).map((t: WikiTag) => ({
              id: t.id,
              name: t.name,
              category: t.category
            }))
          : []
    } else if (tab.value === 'official') {
      const r = await ge.officialSearch(keyword.value, undefined, undefined, 50)
      rows.value =
        r.code === 0
          ? (r.data?.items ?? []).map((o: WikiOfficial) => ({
              id: o.id,
              name: o.name,
              category: o.category
            }))
          : []
    } else if (tab.value === 'engine') {
      const r = await ge.engineList()
      const all =
        r.code === 0
          ? (r.data ?? []).map((e: WikiEngine) => ({
              id: e.id,
              name: e.name,
              description: e.description
            }))
          : []
      const kw = keyword.value.trim().toLowerCase()
      rows.value = kw
        ? all.filter((e) => e.name.toLowerCase().includes(kw))
        : all
    } else {
      const r = await ge.seriesList({ limit: 50 })
      rows.value =
        r.code === 0
          ? (r.data?.items ?? []).map((s: WikiSeries) => ({
              id: s.id,
              name: s.name,
              description: s.description
            }))
          : []
    }
  } finally {
    loading.value = false
  }
}
watch(tab, () => {
  keyword.value = ''
  loadList()
})
let t: ReturnType<typeof setTimeout> | null = null
watch(keyword, () => {
  if (t) clearTimeout(t)
  t = setTimeout(loadList, 250)
})
onMounted(loadList)

// ─── Create / edit modal ──────────────────────────────
const modalOpen = ref(false)
const editing = ref<Row | null>(null) // null = create
const saving = ref(false)
const f = reactive({
  name: '',
  category: 'content',
  description: '',
  alias: '',
  galgame_ids: '' // series only
})

const openCreate = () => {
  editing.value = null
  f.name = ''
  f.category = tab.value === 'official' ? 'company' : 'content'
  f.description = ''
  f.alias = ''
  f.galgame_ids = ''
  modalOpen.value = true
}
const openEdit = (row: Row) => {
  editing.value = row
  f.name = row.name
  f.category = row.category ?? 'content'
  f.description = row.description ?? ''
  f.alias = ''
  f.galgame_ids = ''
  modalOpen.value = true
}

const aliasArr = () =>
  f.alias
    .split(/[,，]/)
    .map((s) => s.trim())
    .filter(Boolean)
const idArr = () =>
  f.galgame_ids
    .split(/[,，]/)
    .map((s) => Number(s.trim()))
    .filter((n) => Number.isFinite(n) && n > 0)

const save = async () => {
  if (!f.name.trim() && tab.value !== 'series') {
    useKunMessage('请填写名称', 'warn')
    return
  }
  saving.value = true
  try {
    let res
    const isEdit = !!editing.value
    if (tab.value === 'tag') {
      res = isEdit
        ? await ge.updateTag({
            tag_id: editing.value!.id,
            name: f.name.trim(),
            category: f.category,
            description: f.description || undefined,
            alias: aliasArr()
          })
        : await ge.createTag({
            name: f.name.trim(),
            category: f.category,
            description: f.description || undefined,
            alias: aliasArr()
          })
    } else if (tab.value === 'official') {
      res = isEdit
        ? await ge.updateOfficial({
            official_id: editing.value!.id,
            name: f.name.trim(),
            category: f.category,
            description: f.description || undefined,
            alias: aliasArr()
          })
        : await ge.createOfficial({
            name: f.name.trim(),
            category: f.category,
            description: f.description || undefined,
            alias: aliasArr()
          })
    } else if (tab.value === 'engine') {
      res = isEdit
        ? await ge.updateEngine({
            engine_id: editing.value!.id,
            name: f.name.trim(),
            description: f.description || undefined,
            alias: aliasArr()
          })
        : await ge.createEngine({
            name: f.name.trim(),
            description: f.description || undefined,
            alias: aliasArr()
          })
    } else {
      res = isEdit
        ? await ge.updateSeries(editing.value!.id, {
            name: f.name.trim() || undefined,
            description: f.description || undefined,
            galgame_ids: idArr()
          })
        : await ge.createSeries({
            name: f.name.trim(),
            description: f.description || undefined,
            galgame_ids: idArr()
          })
    }
    if (res.code === 0) {
      useKunMessage(isEdit ? '已保存' : '已创建', 'success')
      modalOpen.value = false
      await loadList()
    } else {
      useKunMessage(res.message || '操作失败', 'error')
    }
  } finally {
    saving.value = false
  }
}

// ─── Two-step delete (doc-mandated) ───────────────────
const deleting = ref<number | null>(null)
const del = async (row: Row) => {
  const ok = await useKunAlert({
    title: '删除',
    type: 'danger',
    message: `确定删除「${row.name || '#' + row.id}」？`
  })
  if (!ok) return
  deleting.value = row.id
  try {
    const call = (force: boolean) => {
      if (tab.value === 'tag') return ge.deleteTag(row.id, force)
      if (tab.value === 'official') return ge.deleteOfficial(row.id, force)
      if (tab.value === 'engine') return ge.deleteEngine(row.id, force)
      return ge.deleteSeries(row.id)
    }
    const res = await call(false)
    if (res.code === 0) {
      useKunMessage('已删除', 'success')
      await loadList()
      return
    }
    // code:7 = still referenced by N galgames → offer forced cascade.
    if (res.code === 7 && tab.value !== 'series') {
      const okForce = await useKunAlert({
        title: '强制删除',
        type: 'danger',
        message: `${res.message}\n\n强制删除会清除它在所有作品上的关联，且不可恢复。确定继续？`
      })
      if (!okForce) {
        deleting.value = null
        return
      }
      const res2 = await call(true)
      if (res2.code === 0) {
        useKunMessage('已强制删除', 'success')
        await loadList()
      } else {
        useKunMessage(res2.message || '删除失败', 'error')
      }
      deleting.value = null
      return
    }
    useKunMessage(res.message || '删除失败（仅管理员 / 版主可操作）', 'error')
  } finally {
    deleting.value = null
  }
}

// ─── W3 / PR4 — Revision history modal ──────────────────────────────────
// Wiki single-table `taxonomy_revision` (entity column distinguishes the 4
// entity types). Snapshot shape varies per entity; we render generically as
// key/value pairs. Revert calls POST /<entity>/:id/revert with {revision: N};
// for deleted rows revert = resurrect (Wiki INSERTs main row + rebuilds
// aliases) — UI surfaces `affected_galgame_ids` so admin can manually re-link
// references via standard galgame edit. See 04-taxonomy.md §修订与回滚.
const ACTION_LABEL: Record<string, { text: string; color: KunUIColor }> = {
  created: { text: '创建', color: 'primary' },
  updated: { text: '更新', color: 'default' },
  deleted: { text: '删除', color: 'danger' },
  reverted: { text: '回滚', color: 'warning' }
}
const actionOf = (a: string): { text: string; color: KunUIColor } =>
  ACTION_LABEL[a] ?? { text: a, color: 'default' }

const histOpen = ref(false)
const histRow = ref<Row | null>(null)
const histLoading = ref(false)
const histList = ref<TaxonomyRevision[]>([])
const histPage = ref(1)
const histTotal = ref(0)
const histLimit = 20
const histTotalPage = computed(() =>
  Math.max(1, Math.ceil(histTotal.value / histLimit))
)
const acting = ref<number | null>(null)
const expandedRev = ref<number | null>(null)

const loadHistory = async () => {
  if (!histRow.value) return
  histLoading.value = true
  try {
    const res = await ge.taxListRevisions(tab.value, histRow.value.id, {
      page: histPage.value,
      limit: histLimit
    })
    if (res.code === 0) {
      histList.value = res.data?.items ?? []
      histTotal.value = res.data?.total ?? 0
    } else {
      useKunMessage(res.message || '加载历史失败', 'error')
    }
  } finally {
    histLoading.value = false
  }
}

const openHistory = async (row: Row) => {
  histRow.value = row
  histPage.value = 1
  histList.value = []
  histTotal.value = 0
  expandedRev.value = null
  histOpen.value = true
  await loadHistory()
}

watch(histPage, loadHistory)

const doRevert = async (rev: TaxonomyRevision) => {
  if (!histRow.value) return
  const verb = rev.action === 'deleted' ? '恢复（撤销删除）' : '回滚'
  const ok = await useKunAlert({
    title: verb,
    message: `${verb}到版本 #${rev.revision}？将在 Wiki 创建一条 reverted 行${
      rev.action === 'deleted'
        ? '；恢复后该实体重新出现，但被引用的作品不会自动加回——需手动到对应 galgame 编辑里重新选上'
        : ''
    }。`
  })
  if (!ok) return
  acting.value = rev.id
  try {
    const res = await ge.taxRevert(
      tab.value,
      histRow.value.id,
      rev.revision
    )
    if (res.code === 0) {
      useKunMessage(`已${verb}`, 'success')
      await loadHistory()
      // 列表也可能变化（恢复 = 实体重现 / 回滚 = 字段变更）
      await loadList()
    } else {
      useKunMessage(res.message || '操作失败（仅 admin/moderator 可）', 'error')
    }
  } finally {
    acting.value = null
  }
}

const toggleExpand = (revId: number) => {
  expandedRev.value = expandedRev.value === revId ? null : revId
}

// Pretty-print a single Snapshot field value (string / array / object).
const fmtSnapshotValue = (v: unknown): string => {
  if (v === null || v === undefined || v === '') return '（空）'
  if (Array.isArray(v)) return v.length === 0 ? '（空）' : v.map(String).join('、')
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}
</script>

<template>
  <AuthRequired>
    <!-- Outer aligns with header (max-w-7xl via default layout); table /
       form body uses inner narrow column for readability. -->
    <div class="container mx-auto my-4">
    <KunHeader
      name="Galgame 分类管理"
      description="标签 / 开发商 / 引擎 / 系列的增删改 —— 元数据由 Galgame Wiki 统一维护"
    />
    <div class="mx-auto max-w-3xl">

    <KunTab
      v-model="tab"
      :items="TABS.map((x) => ({ value: x.key, textValue: x.title }))"
      variant="light"
      color="primary"
      size="md"
      class="mt-4"
    />

    <div class="mt-4 flex gap-2">
      <KunInput
        v-model="keyword"
        :placeholder="
          tab === 'engine'
            ? '过滤引擎…'
            : tab === 'series'
              ? '系列列表（前 50）'
              : `搜索${TABS.find((x) => x.key === tab)?.title}…`
        "
        :disabled="tab === 'series'"
      />
      <KunButton @click="openCreate">新建</KunButton>
    </div>

    <KunLoading v-if="loading" class-name="mt-6" description="加载中..." />
    <KunNull
      v-else-if="!rows.length"
      class-name="mt-6"
      description="没有数据"
    />
    <div v-else class="mt-4 space-y-2">
      <KunCard v-for="r in rows" :key="r.id" :bordered="true">
        <div class="flex items-center justify-between gap-3 p-3">
          <div class="min-w-0">
            <p class="truncate text-sm font-medium">
              {{ r.name }}
              <span class="text-default-400 text-xs">#{{ r.id }}</span>
            </p>
            <p
              v-if="r.category || r.description"
              class="text-default-500 truncate text-xs"
            >
              {{ r.category }}{{ r.description ? ` · ${r.description}` : '' }}
            </p>
          </div>
          <div class="flex shrink-0 gap-2">
            <KunButton variant="light" size="sm" @click="openHistory(r)">
              历史
            </KunButton>
            <KunButton variant="light" size="sm" @click="openEdit(r)">
              编辑
            </KunButton>
            <KunButton
              variant="bordered"
              color="danger"
              size="sm"
              :loading="deleting === r.id"
              :disabled="deleting !== null"
              @click="del(r)"
            >
              删除
            </KunButton>
          </div>
        </div>
      </KunCard>
    </div>

    <!-- isDismissable=false: edit form with name + category + description
         fields; accidental backdrop click would lose unsaved input. The
         history modal below stays dismissable — it's read-only. -->
    <KunModal
      v-model="modalOpen"
      :is-show-close-button="true"
      :is-dismissable="false"
    >
      <div class="w-[92vw] max-w-md space-y-3 p-5">
        <h3 class="text-lg font-semibold">
          {{ editing ? '编辑' : '新建'
          }}{{ TABS.find((x) => x.key === tab)?.title }}
        </h3>
        <KunInput v-model="f.name" label="名称" />
        <KunSelect
          v-if="tab === 'tag'"
          v-model="f.category"
          label="类别"
          :options="[
            { value: 'content', label: 'content 内容' },
            { value: 'sexual', label: 'sexual 性相关' },
            { value: 'technical', label: 'technical 技术' }
          ]"
        />
        <KunSelect
          v-else-if="tab === 'official'"
          v-model="f.category"
          label="类别"
          :options="[
            { value: 'company', label: 'company 公司' },
            { value: 'individual', label: 'individual 个人' },
            { value: 'amateur', label: 'amateur 同人' }
          ]"
        />
        <KunInput v-model="f.description" label="描述（可选）" />
        <KunInput
          v-if="tab !== 'series'"
          v-model="f.alias"
          label="别名（逗号分隔，PUT 时替换全部）"
        />
        <KunInput
          v-else
          v-model="f.galgame_ids"
          label="Galgame ID（逗号分隔，替换系列全部成员）"
          placeholder="如 8329, 1024"
        />
        <div class="flex justify-end gap-2">
          <KunButton
            variant="bordered"
            :disabled="saving"
            @click="modalOpen = false"
          >
            取消
          </KunButton>
          <KunButton :loading="saving" @click="save">保存</KunButton>
        </div>
      </div>
    </KunModal>

    <!-- W3 / PR4 — Revision history modal (per entity) -->
    <KunModal v-model="histOpen" size="xl" :is-show-close-button="true">
      <div class="max-h-[85vh] w-[92vw] max-w-2xl space-y-3 overflow-y-auto p-5">
        <h3 class="text-lg font-semibold">
          编辑历史 ·
          {{ TABS.find((x) => x.key === tab)?.title }}
          <span class="text-default-400 text-xs">
            {{ histRow?.name }} #{{ histRow?.id }}
          </span>
        </h3>

        <KunLoading v-if="histLoading" description="加载中..." />
        <KunNull v-else-if="!histList.length" description="暂无修订记录" />

        <div v-else class="space-y-2">
          <KunCard
            v-for="rev in histList"
            :key="rev.id"
            :bordered="true"
          >
            <div class="space-y-2 p-3">
              <div class="flex flex-wrap items-center gap-2">
                <span class="font-mono text-sm font-semibold">
                  #{{ rev.revision }}
                </span>
                <KunChip :color="actionOf(rev.action).color" size="sm">
                  {{ actionOf(rev.action).text }}
                </KunChip>
                <span class="text-default-500 text-xs">
                  用户 #{{ rev.user_id }} ·
                  {{
                    formatDate(rev.created, {
                      isPrecise: true,
                      isShowYear: true
                    })
                  }}
                </span>
              </div>

              <p v-if="rev.changed_fields?.length" class="text-default-600 text-xs">
                改动字段: {{ rev.changed_fields.join(', ') }}
              </p>
              <p v-if="rev.note" class="text-default-700 text-sm">
                {{ rev.note }}
              </p>

              <!-- Deleted: surface ref_count + affected_galgame_ids -->
              <div
                v-if="rev.action === 'deleted'"
                class="border-danger/30 bg-danger/10 rounded-lg border p-2 text-xs"
              >
                <p class="text-danger font-semibold">
                  删除时被 {{ rev.ref_count ?? 0 }} 部作品引用
                </p>
                <p
                  v-if="rev.affected_galgame_ids?.length"
                  class="text-default-700 mt-1 break-all"
                >
                  受影响 Galgame ID: {{ rev.affected_galgame_ids.join(', ') }}
                </p>
                <p class="text-default-500 mt-1">
                  「恢复」只复活该实体本身；要把它重新挂到这些作品上，需到对应
                  galgame 编辑页手动加回。
                </p>
              </div>

              <!-- Expandable snapshot -->
              <KunButton
                variant="light"
                color="primary"
                size="xs"
                class-name="self-start"
                @click="toggleExpand(rev.id)"
              >
                {{ expandedRev === rev.id ? '收起快照' : '查看快照' }}
              </KunButton>
              <div
                v-if="expandedRev === rev.id"
                class="border-default/20 space-y-1 rounded-lg border p-2 text-xs"
              >
                <div
                  v-for="[k, v] in Object.entries(rev.snapshot ?? {})"
                  :key="k"
                >
                  <span class="text-default-500">{{ k }}:</span>
                  <span class="text-default-700 ml-1 break-all">
                    {{ fmtSnapshotValue(v) }}
                  </span>
                </div>
              </div>

              <div class="flex justify-end">
                <KunButton
                  variant="bordered"
                  :color="rev.action === 'deleted' ? 'success' : 'warning'"
                  size="sm"
                  :loading="acting === rev.id"
                  :disabled="acting !== null"
                  @click="doRevert(rev)"
                >
                  {{ rev.action === 'deleted' ? '恢复' : '回滚到此版本' }}
                </KunButton>
              </div>
            </div>
          </KunCard>

          <KunPagination
            v-if="histTotalPage > 1"
            v-model:current-page="histPage"
            :total-page="histTotalPage"
            :is-loading="histLoading"
            class="mt-4"
          />
        </div>
      </div>
    </KunModal>
    </div>
  </div>
  </AuthRequired>
</template>
