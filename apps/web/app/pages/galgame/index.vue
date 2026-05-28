<script setup lang="ts">
import {
  ALL_SUPPORTED_TYPE,
  SUPPORTED_TYPE_MAP
} from '~/constants/resource'
import { GALGAME_SORT_FIELD_LABEL_MAP } from '~/constants/galgame'

const route = useRoute()
const router = useRouter()
const api = useApi()

// SFW-by-default per useApi resolution — listing only contains sfw rows
// for anonymous crawlers, so a rich keyword-laden description is safe.
useKunSeoMeta({
  title: 'Galgame 列表',
  description:
    '鲲 Galgame 补丁站收录的全部 Galgame 列表，按发布时间、资源更新时间、浏览量、下载量排序，支持按平台 / 语言 / 翻译类型筛选，免费下载 Windows / 安卓 / KRKR / Tyranor 等平台的 Galgame 中文汉化补丁。'
})

const page = ref(Number(route.query.page ?? 1))
const selectedType = ref(String(route.query.type ?? 'all'))
const sortField = ref(String(route.query.sort_field ?? 'resource_update_time'))
const sortOrder = ref(String(route.query.sort_order ?? 'desc'))
// 发售日期两级筛选：先选年，再（可选）选月。'all' = 不限。
const selectedYear = ref(String(route.query.year ?? 'all'))
const selectedMonth = ref(String(route.query.month ?? 'all'))

const limit = 24

interface ListResponse {
  galgames: GalgameCard[]
  total: number
}

// Turn the (year, month) chip selection into the backend's released_from /
// released_to bounds (wiki §17 format):
//   全部年份         → 不传（不过滤）
//   某年 + 全年      → released_from=YYYY & released_to=YYYY（整年）
//   某年 + 某月      → released_from=YYYY-MM & released_to=YYYY-MM（整月）
const releaseBounds = (): { from: string; to: string } => {
  if (selectedYear.value === 'all') return { from: '', to: '' }
  if (selectedMonth.value === 'all') {
    return { from: selectedYear.value, to: selectedYear.value }
  }
  const ym = `${selectedYear.value}-${selectedMonth.value}`
  return { from: ym, to: ym }
}

const { data, pending, refresh } = await useAsyncData<ListResponse>(
  'galgame-list',
  async () => {
    // Query params are snake_case to match apps/api/internal/common/handler.go
    // galgameListRequest.
    const params = new URLSearchParams({
      selected_type: selectedType.value,
      sort_field: sortField.value,
      sort_order: sortOrder.value,
      page: String(page.value),
      limit: String(limit)
    })
    const { from, to } = releaseBounds()
    if (from) params.set('released_from', from)
    if (to) params.set('released_to', to)

    const res = await api.get<ListResponse>(`/galgame?${params.toString()}`)
    if (res.code !== 0) return { galgames: [], total: 0 }
    return res.data
  },
  { default: () => ({ galgames: [], total: 0 }) }
)

const typeOptions = computed(() =>
  ALL_SUPPORTED_TYPE.map((t) => ({
    value: t,
    label: SUPPORTED_TYPE_MAP[t] ?? t
  }))
)

const sortFieldOptions = computed(() =>
  Object.entries(GALGAME_SORT_FIELD_LABEL_MAP).map(([value, label]) => ({
    value,
    label
  }))
)

// 年份: 全部 + 今年回溯到 1980（galgame 发售年份跨度大）。横滚容纳。
const currentYear = new Date().getFullYear()
const yearOptions = computed(() => [
  { value: 'all', label: '全部年份' },
  ...Array.from({ length: currentYear - 1979 }, (_, i) => {
    const y = String(currentYear - i)
    return { value: y, label: `${y} 年` }
  })
])

// 月份: 全年 + 1–12 月（值是 zero-pad 的 MM，拼 YYYY-MM 给后端）。
const monthOptions = computed(() => [
  { value: 'all', label: '全年' },
  ...Array.from({ length: 12 }, (_, i) => ({
    value: String(i + 1).padStart(2, '0'),
    label: `${i + 1} 月`
  }))
])

const updateQuery = async () => {
  await router.replace({
    query: {
      page: page.value,
      type: selectedType.value,
      sort_field: sortField.value,
      sort_order: sortOrder.value,
      year: selectedYear.value,
      month: selectedMonth.value
    }
  })
  await refresh()
}

const setType = (v: string) => {
  if (selectedType.value === v) return
  selectedType.value = v
  page.value = 1
  updateQuery()
}
const setSortField = (v: string) => {
  if (sortField.value === v) return
  sortField.value = v
  page.value = 1
  updateQuery()
}
const setSortOrder = (v: 'asc' | 'desc') => {
  if (sortOrder.value === v) return
  sortOrder.value = v
  page.value = 1
  updateQuery()
}
const setYear = (v: string) => {
  if (selectedYear.value === v) return
  selectedYear.value = v
  // 换年后旧的月选择语义失效，重置为全年。
  selectedMonth.value = 'all'
  page.value = 1
  updateQuery()
}
const setMonth = (v: string) => {
  if (selectedMonth.value === v) return
  selectedMonth.value = v
  page.value = 1
  updateQuery()
}
const onChangePage = (v: number) => {
  page.value = v
  updateQuery()
  if (import.meta.client) window.scrollTo({ top: 0 })
}

const totalPages = computed(() => Math.ceil((data.value?.total ?? 0) / limit))
</script>

<template>
  <div class="container mx-auto my-4 space-y-6">
    <KunHeader
      name="Galgame"
      description="本页面默认仅显示了 SFW (内容安全) 的补丁, 您可以在网站右上角切换显示全部补丁 (包括 NSFW, 也就是显示可能带有涩涩的补丁)"
    />

    <!-- Filter chip rows — modelled on kungal's GalgameCardNav. Each
         dimension is one horizontally-scrollable row of buttons (active lit
         primary, inactive muted), prefixed by a shrink-0 dimension label so
         the multiple rows stay legible. Faster than dropdown selects (one
         click vs two) and visually consistent with kungal. -->
    <div class="space-y-1.5">
      <div class="flex items-center gap-2">
        <span class="text-default-400 w-12 shrink-0 text-xs">类型</span>
        <div class="-mx-1 flex gap-1 overflow-x-auto px-1 pb-0.5">
          <button
            v-for="opt in typeOptions"
            :key="opt.value"
            type="button"
            :class="[
              'shrink-0 cursor-pointer rounded-md px-2.5 py-1 text-sm whitespace-nowrap transition-colors',
              selectedType === opt.value
                ? 'bg-primary/15 text-primary font-medium'
                : 'text-default-600 hover:bg-default-100'
            ]"
            @click="setType(opt.value)"
          >
            {{ opt.label }}
          </button>
        </div>
      </div>

      <!-- 发售年份 -->
      <div class="flex items-center gap-2">
        <span class="text-default-400 w-12 shrink-0 text-xs">发售年</span>
        <div class="-mx-1 flex gap-1 overflow-x-auto px-1 pb-0.5">
          <button
            v-for="opt in yearOptions"
            :key="opt.value"
            type="button"
            :class="[
              'shrink-0 cursor-pointer rounded-md px-2.5 py-1 text-sm whitespace-nowrap transition-colors',
              selectedYear === opt.value
                ? 'bg-primary/15 text-primary font-medium'
                : 'text-default-600 hover:bg-default-100'
            ]"
            @click="setYear(opt.value)"
          >
            {{ opt.label }}
          </button>
        </div>
      </div>

      <!-- 发售月份：仅当选定了具体年份时出现（两级筛选的第二级）。 -->
      <div v-if="selectedYear !== 'all'" class="flex items-center gap-2">
        <span class="text-default-400 w-12 shrink-0 text-xs">发售月</span>
        <div class="-mx-1 flex gap-1 overflow-x-auto px-1 pb-0.5">
          <button
            v-for="opt in monthOptions"
            :key="opt.value"
            type="button"
            :class="[
              'shrink-0 cursor-pointer rounded-md px-2.5 py-1 text-sm whitespace-nowrap transition-colors',
              selectedMonth === opt.value
                ? 'bg-primary/15 text-primary font-medium'
                : 'text-default-600 hover:bg-default-100'
            ]"
            @click="setMonth(opt.value)"
          >
            {{ opt.label }}
          </button>
        </div>
      </div>

      <div class="flex items-center gap-2">
        <span class="text-default-400 w-12 shrink-0 text-xs">排序</span>
        <div class="-mx-1 flex gap-1 overflow-x-auto px-1 pb-0.5">
          <button
            v-for="opt in sortFieldOptions"
            :key="opt.value"
            type="button"
            :class="[
              'shrink-0 cursor-pointer rounded-md px-2.5 py-1 text-sm whitespace-nowrap transition-colors',
              sortField === opt.value
                ? 'bg-primary/15 text-primary font-medium'
                : 'text-default-600 hover:bg-default-100'
            ]"
            @click="setSortField(opt.value)"
          >
            {{ opt.label }}
          </button>
          <!-- Sort direction = explicit pair (not toggle) so the user always
               sees the alternative. Sits at the end of the sort row. -->
          <span class="bg-default-200 mx-1 h-5 w-px shrink-0 self-center" aria-hidden="true" />
          <button
            type="button"
            aria-label="降序"
            :class="[
              'shrink-0 cursor-pointer rounded-md p-1 transition-colors',
              sortOrder === 'desc'
                ? 'bg-primary/15 text-primary'
                : 'text-default-500 hover:bg-default-100'
            ]"
            @click="setSortOrder('desc')"
          >
            <KunIcon name="lucide:arrow-down" class="size-4" />
          </button>
          <button
            type="button"
            aria-label="升序"
            :class="[
              'shrink-0 cursor-pointer rounded-md p-1 transition-colors',
              sortOrder === 'asc'
                ? 'bg-primary/15 text-primary'
                : 'text-default-500 hover:bg-default-100'
            ]"
            @click="setSortOrder('asc')"
          >
            <KunIcon name="lucide:arrow-up" class="size-4" />
          </button>
        </div>
      </div>
    </div>

    <KunLoading v-if="pending" description="正在获取 Galgame 数据..." />
    <div
      v-else
      class="mx-auto mb-8 grid grid-cols-2 gap-2 sm:gap-6 lg:grid-cols-3 xl:grid-cols-4"
    >
      <GalgameCard
        v-for="patch in data?.galgames"
        :key="patch.id"
        :patch="patch"
      />
    </div>

    <KunNull
      v-if="!pending && !data?.galgames?.length"
      description="暂无数据"
    />

    <div v-if="totalPages > 1" class="flex justify-center">
      <KunPagination
        :current-page="page"
        :total-page="totalPages"
        :is-loading="pending"
        @update:current-page="onChangePage"
      />
    </div>
  </div>
</template>
