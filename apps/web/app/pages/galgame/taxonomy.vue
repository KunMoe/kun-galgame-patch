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
  WikiSeries
} from '~/composables/useGalgameEdit'

useKunSeoMeta({
  title: 'Galgame 分类管理',
  description: '管理 Galgame Wiki 的标签 / 开发商 / 引擎 / 系列'
})

const route = useRoute()
const userStore = useUserStore()
const ge = useGalgameEdit()

if (!userStore.isLoggedIn) {
  await navigateTo({ path: '/login', query: { from: route.fullPath } })
}

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
  if (!confirm(`确定删除「${row.name || '#' + row.id}」？`)) return
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
      if (
        !confirm(
          `${res.message}\n\n强制删除会清除它在所有作品上的关联，且不可恢复。确定继续？`
        )
      )
        return
      const forced = await call(true)
      if (forced.code === 0) {
        useKunMessage('已强制删除', 'success')
        await loadList()
      } else {
        useKunMessage(forced.message || '强制删除失败', 'error')
      }
      return
    }
    useKunMessage(res.message || '删除失败（仅管理员 / 协管可操作）', 'error')
  } finally {
    deleting.value = null
  }
}
</script>

<template>
  <div class="container mx-auto my-4 max-w-3xl px-4">
    <KunHeader
      name="Galgame 分类管理"
      description="标签 / 开发商 / 引擎 / 系列的增删改 —— 元数据由 Galgame Wiki 统一维护"
    />

    <nav
      class="border-default/20 bg-background/60 mt-4 flex gap-1 rounded-2xl border p-1"
    >
      <button
        v-for="x in TABS"
        :key="x.key"
        type="button"
        :class="
          cn(
            'flex-1 rounded-xl px-3 py-2 text-center text-sm transition-colors',
            tab === x.key
              ? 'bg-primary/10 text-primary font-medium'
              : 'text-default-600 hover:bg-default-100'
          )
        "
        @click="tab = x.key"
      >
        {{ x.title }}
      </button>
    </nav>

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

    <KunModal v-model:modal-value="modalOpen" :is-show-close-button="true">
      <div class="w-[92vw] max-w-md space-y-3 p-5">
        <h3 class="text-lg font-semibold">
          {{ editing ? '编辑' : '新建'
          }}{{ TABS.find((x) => x.key === tab)?.title }}
        </h3>
        <KunInput v-model="f.name" label="名称" />
        <select
          v-if="tab === 'tag' || tab === 'official'"
          v-model="f.category"
          class="border-default/30 bg-background w-full rounded-lg border px-2 py-2 text-sm"
        >
          <template v-if="tab === 'tag'">
            <option value="content">content 内容</option>
            <option value="sexual">sexual 性相关</option>
            <option value="technical">technical 技术</option>
          </template>
          <template v-else>
            <option value="company">company 公司</option>
            <option value="individual">individual 个人</option>
            <option value="amateur">amateur 同人</option>
          </template>
        </select>
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
  </div>
</template>
