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
// 发售日期筛选：年份单选（'all' | YYYY）+ 月份多选集合（不连续月份，wiki
// §17.10 released_months）。两者正交组合：
//   年=all  + 月集合空   → 不筛
//   年=all  + 月集合     → 历年这些月（released_months）
//   年=YYYY + 月集合空   → 该年整年（released_from/to）
//   年=YYYY + 月集合     → 该年这些月（区间 + released_months 叠加）
const selectedYear = ref(String(route.query.year ?? 'all'))
const parseMonthsQuery = (q: unknown): number[] => {
  const s = String(q ?? '').trim()
  if (!s) return []
  return s
    .split(',')
    .map((x) => Number(x.trim()))
    .filter((n) => Number.isInteger(n) && n >= 1 && n <= 12)
}
const selectedMonths = ref<number[]>(parseMonthsQuery(route.query.months))

const limit = 24

interface ListResponse {
  galgames: GalgameCard[]
  total: number
}

// Compose the year + months selection into backend query params
// (released_from/to per §17, released_months per §17.10).
const releaseQuery = (): Record<string, string> => {
  const q: Record<string, string> = {}
  if (selectedYear.value !== 'all') {
    q.released_from = selectedYear.value
    q.released_to = selectedYear.value
  }
  if (selectedMonths.value.length > 0) {
    q.released_months = [...selectedMonths.value]
      .sort((a, b) => a - b)
      .join(',')
  }
  return q
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
    for (const [k, v] of Object.entries(releaseQuery())) params.set(k, v)

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

// 月份多选 1–12（空集合 = 不限月，无需 "全年" 选项）。
const monthOptions = Array.from({ length: 12 }, (_, i) => ({
  value: i + 1,
  label: `${i + 1} 月`
}))

const updateQuery = async () => {
  await router.replace({
    query: {
      page: page.value,
      type: selectedType.value,
      sort_field: sortField.value,
      sort_order: sortOrder.value,
      year: selectedYear.value,
      months: selectedMonths.value.join(',')
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
  // 月份集合独立于年（跨年也有意义，如"历年三月"），换年不重置。
  page.value = 1
  updateQuery()
}
const toggleMonth = (m: number) => {
  selectedMonths.value = selectedMonths.value.includes(m)
    ? selectedMonths.value.filter((x) => x !== m)
    : [...selectedMonths.value, m]
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

      <!-- 发售月份：多选集合（点多个月可叠加），始终可见。年=全部 +
           选若干月 = "历年这些月发售"；年=某年 + 选若干月 = 该年这些月。
           空集合 = 不限月。 -->
      <div class="flex items-center gap-2">
        <span class="text-default-400 w-12 shrink-0 text-xs">发售月</span>
        <div class="-mx-1 flex gap-1 overflow-x-auto px-1 pb-0.5">
          <button
            v-for="opt in monthOptions"
            :key="opt.value"
            type="button"
            :class="[
              'shrink-0 cursor-pointer rounded-md px-2.5 py-1 text-sm whitespace-nowrap transition-colors',
              selectedMonths.includes(opt.value)
                ? 'bg-primary/15 text-primary font-medium'
                : 'text-default-600 hover:bg-default-100'
            ]"
            @click="toggleMonth(opt.value)"
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
