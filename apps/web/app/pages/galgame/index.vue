<script setup lang="ts">
import { ALL_SUPPORTED_TYPE, SUPPORTED_TYPE_MAP } from '~/constants/resource'
import { GALGAME_SORT_FIELD_LABEL_MAP } from '~/constants/galgame'

const route = useRoute()
const router = useRouter()
const api = useApi()
// The "显示无补丁资源的游戏" toggle is forwarded to every request globally by
// useApi (as include_empty) — so we don't build the param here. We only need
// the store to WATCH the toggle: the 显示设置 panel lives on this page, so when
// it flips we reset to page 1 and refetch (other pages remount on navigation).
const settingStore = useSettingStore()

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

// 发售日期筛选（高级筛选面板内，参考 kungal GalgameCardNav）。两个正交控件：
//   • 年份区间 — released_from / released_to（各 '' | 'YYYY'，独立）。两端同年
//     = 单年；留一端空 = 开区间（"2020 及以后" / "2024 及以前"）。
//   • 月份多选 — selectedMonths（不连续月集合，wiki §17.10）。与年份区间 AND
//     组合，且脱离年份也成立（"历年三月" = 只选月不选年）。
// 空区间 + 空月集 = 不筛。released_from='' → 后端 nil 下界（同理上界 / 月集）。
const releasedFrom = ref(String(route.query.released_from ?? ''))
const releasedTo = ref(String(route.query.released_to ?? ''))
const parseMonthsQuery = (q: unknown): number[] => {
  const s = String(q ?? '').trim()
  if (!s) return []
  return s
    .split(',')
    .map((x) => Number(x.trim()))
    .filter((n) => Number.isInteger(n) && n >= 1 && n <= 12)
}
const selectedMonths = ref<number[]>(parseMonthsQuery(route.query.released_months))

const limit = 24

interface ListResponse {
  galgames: GalgameCard[]
  total: number
}

const { data, pending, refresh } = await useAsyncData<ListResponse>(
  'galgame-list',
  async () => {
    // Query params are snake_case to match apps/api/internal/common/handler.go
    // galgameListRequest. Date params omitted when empty (BE reads absent ==
    // unset == no bound, per pkg/utils ParseRelease*Bound).
    const params = new URLSearchParams({
      selected_type: selectedType.value,
      sort_field: sortField.value,
      sort_order: sortOrder.value,
      page: String(page.value),
      limit: String(limit)
    })
    if (releasedFrom.value) params.set('released_from', releasedFrom.value)
    if (releasedTo.value) params.set('released_to', releasedTo.value)
    if (selectedMonths.value.length > 0) {
      params.set(
        'released_months',
        [...selectedMonths.value].sort((a, b) => a - b).join(',')
      )
    }

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

// 年份: 不限 + 今年回溯到 1980（galgame 发售年份跨度大）。横滚容纳。
const currentYear = new Date().getFullYear()
const yearOptions = computed(() => [
  { value: '', label: '不限' },
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

// 高级筛选面板开合。面板内含全部发售日期控件，主栏只留类型 + 排序。
const showFilters = ref(false)

// 主栏排序项之外，发售日期是否有筛选 —— 用于高亮 "高级筛选" 按钮，让收起
// 状态下也能看出面板里有生效筛选。
const hasAdvancedFilter = computed(
  () =>
    !!releasedFrom.value ||
    !!releasedTo.value ||
    selectedMonths.value.length > 0
)

// 任一维度偏离默认值即可重置。默认: type=all / 排序=补丁更新时间 desc / 无日期。
const hasActiveFilter = computed(
  () =>
    selectedType.value !== 'all' ||
    sortField.value !== 'resource_update_time' ||
    sortOrder.value !== 'desc' ||
    hasAdvancedFilter.value
)

const buildQuery = (): Record<string, string> => {
  const q: Record<string, string> = {
    page: String(page.value),
    type: selectedType.value,
    sort_field: sortField.value,
    sort_order: sortOrder.value
  }
  if (releasedFrom.value) q.released_from = releasedFrom.value
  if (releasedTo.value) q.released_to = releasedTo.value
  if (selectedMonths.value.length > 0) {
    q.released_months = [...selectedMonths.value]
      .sort((a, b) => a - b)
      .join(',')
  }
  return q
}

const updateQuery = async () => {
  await router.replace({ query: buildQuery() })
  await refresh()
}

// Toggling "显示无补丁资源的游戏" changes both the rows and the total, so reset
// to page 1 and refetch. The param itself isn't URL-synced (it's a persisted
// preference, like title language), but page reset keeps the URL consistent.
watch(
  () => settingStore.data.showGalgamesWithoutResource,
  () => {
    page.value = 1
    updateQuery()
  }
)

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

// 年份区间 setter，带钳制：起始年晚于结束年时拖动另一端跟随，避免倒置区间
// （PG 上 from > to 会静默返回空）。
const setFromYear = (year: string) => {
  if (releasedFrom.value === year) return
  releasedFrom.value = year
  if (year && releasedTo.value && Number(releasedTo.value) < Number(year)) {
    releasedTo.value = year
  }
  page.value = 1
  updateQuery()
}
const setToYear = (year: string) => {
  if (releasedTo.value === year) return
  releasedTo.value = year
  if (year && releasedFrom.value && Number(releasedFrom.value) > Number(year)) {
    releasedFrom.value = year
  }
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

const resetFilters = () => {
  selectedType.value = 'all'
  sortField.value = 'resource_update_time'
  sortOrder.value = 'desc'
  releasedFrom.value = ''
  releasedTo.value = ''
  selectedMonths.value = []
  page.value = 1
  updateQuery()
}

const onChangePage = (v: number) => {
  page.value = v
  updateQuery()
  if (import.meta.client) window.scrollTo({ top: 0 })
}

const totalPages = computed(() => Math.ceil((data.value?.total ?? 0) / limit))

// Chip-button class shared by every filter row (active lit primary, inactive
// muted). Mirrors kungal GalgameCardNav's button styling.
const chipClass = (active: boolean) => [
  'shrink-0 cursor-pointer rounded-md px-2.5 py-1 text-sm whitespace-nowrap transition-colors',
  active
    ? 'bg-primary/15 text-primary font-medium'
    : 'text-default-600 hover:bg-default-100'
]
</script>

<template>
  <div class="container mx-auto my-4 space-y-6">
    <KunHeader
      name="Galgame"
      description="本页面默认仅显示了 SFW (内容安全) 的补丁, 您可以在网站右上角切换显示全部补丁 (包括 NSFW, 也就是显示可能带有涩涩的补丁)"
    />

    <!-- Filter bar — modelled on kungal's GalgameCardNav. The main bar keeps
         only the frequently-toggled dimensions (类型 + 排序); release-date
         lives in a foldable 高级筛选 panel so the long year/month rows don't
         clutter the page. Each row is one horizontally-scrollable strip of
         chip buttons (active lit primary, inactive muted). -->
    <div class="space-y-1.5">
      <div class="-mx-1 flex gap-1 overflow-x-auto px-1 pb-0.5">
        <button
          v-for="opt in typeOptions"
          :key="opt.value"
          type="button"
          :class="chipClass(selectedType === opt.value)"
          @click="setType(opt.value)"
        >
          {{ opt.label }}
        </button>
      </div>

      <div class="-mx-1 flex gap-1 overflow-x-auto px-1 pb-0.5">
        <button
          v-for="opt in sortFieldOptions"
          :key="opt.value"
          type="button"
          :class="chipClass(sortField === opt.value)"
          @click="setSortField(opt.value)"
        >
          {{ opt.label }}
        </button>
      </div>

      <!-- Action row: sort direction (explicit asc/desc pair so the
           alternative is always visible) sits alongside the panel toggle +
           reset. -->
      <div class="flex items-center gap-1.5">
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

        <span
          class="bg-default-200 mx-1 h-5 w-px shrink-0 self-center"
          aria-hidden="true"
        />

        <button
          type="button"
          class="text-default-500 hover:text-primary flex cursor-pointer items-center gap-1 rounded-md px-2 py-1 text-sm transition-colors"
          :class="hasAdvancedFilter && 'text-warning'"
          @click="showFilters = !showFilters"
        >
          <KunIcon name="lucide:sliders-horizontal" class="text-inherit" />
          <span>高级筛选</span>
        </button>

        <GalgameDisplaySettings />

        <button
          v-if="hasActiveFilter"
          type="button"
          class="text-default-500 hover:text-danger flex cursor-pointer items-center gap-1 rounded-md px-2 py-1 text-sm transition-colors"
          @click="resetFilters"
        >
          <KunIcon name="lucide:rotate-ccw" class="text-inherit" />
          <span>重置筛选</span>
        </button>
      </div>

      <div
        v-if="showFilters"
        class="bg-default-50 space-y-4 rounded-lg border p-3"
      >
        <div class="text-primary border-b pb-1 text-sm font-semibold">
          发售日期
        </div>

        <div>
          <div class="text-default-700 mb-1.5 text-xs font-medium">起始年份</div>
          <div class="-mx-1 flex gap-1 overflow-x-auto px-1 pb-0.5">
            <button
              v-for="opt in yearOptions"
              :key="opt.value || 'from-all'"
              type="button"
              :class="chipClass(releasedFrom === opt.value)"
              @click="setFromYear(opt.value)"
            >
              {{ opt.label }}
            </button>
          </div>
        </div>

        <div>
          <div class="text-default-700 mb-1.5 text-xs font-medium">结束年份</div>
          <div class="-mx-1 flex gap-1 overflow-x-auto px-1 pb-0.5">
            <button
              v-for="opt in yearOptions"
              :key="opt.value || 'to-all'"
              type="button"
              :class="chipClass(releasedTo === opt.value)"
              @click="setToYear(opt.value)"
            >
              {{ opt.label }}
            </button>
          </div>
        </div>

        <div>
          <div class="text-default-700 mb-1.5 text-xs font-medium">
            发售月份
            <span class="text-default-400 font-normal">(可多选, 含历年)</span>
          </div>
          <div class="-mx-1 flex gap-1 overflow-x-auto px-1 pb-0.5">
            <button
              v-for="opt in monthOptions"
              :key="opt.value"
              type="button"
              :class="chipClass(selectedMonths.includes(opt.value))"
              @click="toggleMonth(opt.value)"
            >
              {{ opt.label }}
            </button>
          </div>
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

    <KunNull v-if="!pending && !data?.galgames?.length" description="暂无数据" />

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
